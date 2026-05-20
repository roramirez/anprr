package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/roramirez/anprr/internal/github"
)

func searchReviewRequestedCmd(client *github.Client, cache *github.Cache, repos []string) tea.Cmd {
	return func() tea.Msg {
		set, err := client.SearchReviewRequested(repos, cache)
		return ReviewRequestedLoadedMsg{Set: set, Err: err}
	}
}

type listState int

const (
	listStateLoading listState = iota
	listStateReady
)

type tabIndex int

const (
	tabMyPRs       tabIndex = iota // [1] PRs I authored
	tabNeedsReview                 // [2] PRs that specifically need my attention
	tabAllOpen                     // [3] All open PRs not authored by me
)

type ListModel struct {
	state              listState
	tab                tabIndex
	allPRs             []github.PR // all PRs fetched via GraphQL
	cursor             int
	currentUser        string
	width              int
	height             int
	spinner            spinner.Model
	err                error
	hasNextPage        map[string]bool   // repo → has more pages
	endCursor          map[string]string // repo → cursor
	reviewRequestedSet map[string]bool   // "owner/repo#number" → true, from Search API
}

func newListModel() ListModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = StyleStatusBarWarn
	return ListModel{
		state:              listStateLoading,
		tab:                tabMyPRs,
		spinner:            sp,
		hasNextPage:        make(map[string]bool),
		endCursor:          make(map[string]string),
		reviewRequestedSet: make(map[string]bool),
	}
}

func (m ListModel) setReviewRequestedSet(set map[string]bool) ListModel {
	m.reviewRequestedSet = set
	return m
}

func (m ListModel) setSize(w, h int) ListModel {
	m.width = w
	m.height = h
	return m
}

func (m ListModel) setCurrentUser(login string) ListModel {
	m.currentUser = login
	return m
}

// visiblePRs returns the PRs for the current tab.
func (m ListModel) visiblePRs() []github.PR {
	var result []github.PR
	for _, pr := range m.allPRs {
		if m.prMatchesTab(pr) {
			result = append(result, pr)
		}
	}
	return result
}

func (m ListModel) prMatchesTab(pr github.PR) bool {
	switch m.tab {
	case tabMyPRs:
		return pr.Author.Login == m.currentUser
	case tabAllOpen:
		return pr.Author.Login != m.currentUser
	case tabNeedsReview:
		return m.needsMyReview(pr)
	default:
		return false
	}
}

func (m ListModel) needsMyReview(pr github.PR) bool {
	if pr.Author.IsBot {
		// Bot PRs (Dependabot, Renovate, etc.) need attention when still pending —
		// someone needs to merge or close them. They don't appear in Search API
		// review-requested because bots don't formally request reviewers.
		return pr.ReviewStatus == github.StatusPending
	}
	// Human PRs — use precise sources:
	// 1. GitHub Search API confirmed this PR needs my review
	//    (includes team requests, respects GitHub's own "needs review" logic)
	key := fmt.Sprintf("%s#%d", pr.Repo, pr.Number)
	if m.reviewRequestedSet[key] {
		return true
	}
	// 2. I already reviewed but new commits were pushed after my last review
	return needsReReview(pr, m.currentUser)
}

// needsReReview returns true if the PR was updated after the current user's last review,
// meaning new commits arrived that haven't been reviewed yet.
func needsReReview(pr github.PR, user string) bool {
	var myLast *github.Review
	for i := range pr.Reviews {
		r := &pr.Reviews[i]
		if r.Author.Login == user {
			if myLast == nil || r.SubmittedAt.After(myLast.SubmittedAt) {
				myLast = r
			}
		}
	}
	if myLast == nil {
		return false
	}
	return pr.UpdatedAt.After(myLast.SubmittedAt)
}

func (m ListModel) selectedPR() (github.PR, bool) {
	prs := m.visiblePRs()
	if len(prs) == 0 || m.cursor >= len(prs) {
		return github.PR{}, false
	}
	return prs[m.cursor], true
}

func (m ListModel) update(msg tea.Msg, client *github.Client, cache *github.Cache, repos []string) (ListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		if m.state != listStateReady {
			return m, nil
		}
		var keyCmd tea.Cmd
		m, keyCmd = m.handleKey(msg, client, cache, repos)
		if m.state == listStateLoading {
			var spinCmd tea.Cmd
			m.spinner, spinCmd = m.spinner.Update(msg)
			return m, tea.Batch(keyCmd, spinCmd)
		}
		return m, keyCmd
	}

	if m.state == listStateLoading {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m ListModel) handleKey(msg tea.KeyMsg, client *github.Client, cache *github.Cache, repos []string) (ListModel, tea.Cmd) {
	prs := m.visiblePRs()
	switch msg.String() {
	case "1":
		m.tab, m.cursor = tabMyPRs, 0
	case "2":
		m.tab, m.cursor = tabNeedsReview, 0
	case "3":
		m.tab, m.cursor = tabAllOpen, 0
	case "j", "down":
		if m.cursor < len(prs)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case keyEnter:
		if pr, ok := m.selectedPR(); ok {
			return m, navigateToDetail(pr, false)
		}
	case "a":
		return m, m.handleApproveKey(client)
	case "r", "c":
		if pr, ok := m.selectedPR(); ok {
			return m, navigateToDetail(pr, true)
		}
	case "f":
		cache.Invalidate()
		m.state = listStateLoading
		return m, tea.Batch(fetchPRsCmd(client, cache, repos), searchReviewRequestedCmd(client, cache, repos))
	case "F":
		return m, m.handleLoadMoreKey(client, cache)
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m ListModel) handleApproveKey(client *github.Client) tea.Cmd {
	pr, ok := m.selectedPR()
	if !ok {
		return nil
	}
	if pr.IsDraft {
		return statusCmd("Cannot approve a draft PR", true)
	}
	return submitReviewCmd(client, pr, github.ReviewApprove, "", nil)
}

func (m ListModel) handleLoadMoreKey(client *github.Client, cache *github.Cache) tea.Cmd {
	var cmds []tea.Cmd
	for repo, hasNext := range m.hasNextPage {
		if hasNext {
			cmds = append(cmds, loadMoreCmd(client, cache, repo, m.endCursor[repo]))
		}
	}
	return tea.Batch(cmds...)
}

func updateListModel(m ListModel, msg PRsLoadedMsg) (ListModel, []tea.Cmd) {
	if msg.Err != nil {
		m.err = msg.Err
		m.state = listStateReady
		return m, []tea.Cmd{statusCmd("Error: "+msg.Err.Error(), true)}
	}
	m.allPRs = msg.PRs
	m.state = listStateReady
	m.err = nil

	// update pagination info from last PR per repo
	repoSeen := map[string]bool{}
	for i := len(m.allPRs) - 1; i >= 0; i-- {
		pr := m.allPRs[i]
		if !repoSeen[pr.Repo] {
			repoSeen[pr.Repo] = true
			if pr.HasNextPage {
				m.hasNextPage[pr.Repo] = true
				m.endCursor[pr.Repo] = pr.EndCursor
			} else {
				m.hasNextPage[pr.Repo] = false
			}
		}
	}

	// clamp cursor
	prs := m.visiblePRs()
	if m.cursor >= len(prs) && len(prs) > 0 {
		m.cursor = len(prs) - 1
	}
	return m, nil
}

func updateListModelMore(m ListModel, msg MorePRsLoadedMsg) (ListModel, []tea.Cmd) {
	if msg.Err != nil {
		return m, []tea.Cmd{statusCmd("Error: "+msg.Err.Error(), true)}
	}
	m.allPRs = append(m.allPRs, msg.PRs...)
	m.hasNextPage[msg.Repo] = msg.HasNext
	m.endCursor[msg.Repo] = msg.EndCursor
	return m, nil
}

func (m ListModel) view(width, height int, statusBar string, rateLimit int64) string {
	headerH := 3 // header + tabs + divider
	footerH := 3 // footer keys + status bar
	listH := height - headerH - footerH
	if listH < 1 {
		listH = 1
	}

	header := m.renderHeader(width, rateLimit)
	tabs := m.renderTabs(width)
	body := m.renderBody(width, listH)
	footer := m.renderFooter(width, statusBar)

	return lipgloss.JoinVertical(lipgloss.Left, header, tabs, body, footer)
}

func (m ListModel) renderHeader(width int, rateLimit int64) string {
	title := " anprr"
	var rateStr string
	if rateLimit >= 0 && rateLimit < 100 {
		rateStr = StyleRateLimit.Render(fmt.Sprintf(" ⚠ %d requests remaining", rateLimit))
	}
	padding := width - len(title) - lipgloss.Width(rateStr) - 2
	if padding < 0 {
		padding = 0
	}
	return StyleHeader.Width(width).Render(title + strings.Repeat(" ", padding) + rateStr)
}

func (m ListModel) renderTabs(width int) string {
	tabs := []struct {
		label string
		idx   tabIndex
	}{
		{"  [1] My PRs  ", tabMyPRs},
		{"  [2] Needs Review  ", tabNeedsReview},
		{"  [3] All Open  ", tabAllOpen},
	}
	var rendered []string
	totalW := 0
	for _, tab := range tabs {
		var s string
		if m.tab == tab.idx {
			s = StyleTabActive.Render(tab.label)
		} else {
			s = StyleTabInactive.Render(tab.label)
		}
		rendered = append(rendered, s)
		totalW += lipgloss.Width(s)
	}
	rest := width - totalW
	if rest < 0 {
		rest = 0
	}
	return strings.Join(rendered, "") + strings.Repeat(" ", rest)
}

func (m ListModel) renderBody(width, height int) string {
	if m.state == listStateLoading {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
			m.spinner.View()+" Loading PRs…")
	}
	if m.err != nil {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
			StyleStatusBarError.Render("Error: "+m.err.Error()))
	}

	prs := m.visiblePRs()
	if len(prs) == 0 {
		msg := "No PRs found."
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, msg)
	}

	var lines []string
	for i, pr := range prs {
		line := m.renderPRRow(pr, i == m.cursor, width)
		lines = append(lines, line)
	}

	// pagination notice
	anyMore := false
	for _, has := range m.hasNextPage {
		if has {
			anyMore = true
			break
		}
	}
	if anyMore {
		lines = append(lines, StyleStatusBar.Render("  Showing first 50 — press F to load more"))
	}

	// pad to fill height
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}

	return strings.Join(lines[:min(len(lines), height)], "\n")
}

func (m ListModel) renderPRRow(pr github.PR, selected bool, width int) string {
	cursor := "  "
	if selected {
		cursor = StyleCursor.Render("▶ ")
	}

	title := pr.Title
	titleStyle := StylePRTitle
	switch {
	case pr.IsDraft:
		title = "[draft] " + title
		titleStyle = StylePRTitleDraft
	case pr.Author.IsBot:
		title = "[bot] " + title
		titleStyle = StylePRTitleDraft // reuse dimmed style for bots
	}

	dot, dotStyle := statusDot(pr)
	age := timeAgo(pr.UpdatedAt)

	// fixed column widths
	numberStr := fmt.Sprintf("#%-4d", pr.Number)
	repoStr := pr.Repo
	ageStr := age
	checkIcon := renderCheckIcon(pr.CheckState)

	// fixed chars: cursor(2) + sep(2) + sep(2) + repo + sep(2) + age + sep(2) + dot(1) + sep(2) + icon(1)
	fixedW := 2 + len(numberStr) + 2 + 2 + len(repoStr) + 2 + len(ageStr) + 2 + 1 + 2 + 1
	titleW := width - fixedW
	if titleW < 10 {
		titleW = 10
	}
	if len(title) > titleW {
		title = title[:titleW-1] + "…"
	} else {
		title += strings.Repeat(" ", titleW-len(title))
	}

	return cursor +
		StylePRRepo.Render(numberStr) + "  " +
		titleStyle.Render(title) + "  " +
		StylePRRepo.Render(repoStr) + "  " +
		StylePRAge.Render(ageStr) + "  " +
		dotStyle.Render(dot) + "  " +
		checkIcon
}

func (m ListModel) renderFooter(width int, statusBar string) string {
	keys := StyleFooter.Render("1/2/3=tab  enter=view  a=approve  r=changes  c=comment  f=refresh  F=more  ?=help  q=quit")
	sb := ""
	if statusBar != "" {
		sb = "\n" + statusBar
	}
	_ = width
	return keys + sb
}

func renderCheckIcon(state string) string {
	switch state {
	case "SUCCESS":
		return StyleCheckSuccess.Render("✓")
	case "FAILURE", "ERROR":
		return StyleCheckFailure.Render("✗")
	case "PENDING", "IN_PROGRESS", "QUEUED":
		return StyleCheckPending.Render("○")
	default:
		return StyleCheckNone.Render("—")
	}
}

func statusDot(pr github.PR) (string, lipgloss.Style) {
	if pr.IsDraft {
		return "○", StyleStatusDraft
	}
	switch pr.ReviewStatus {
	case github.StatusApproved:
		return dotFilled, StyleStatusApproved
	case github.StatusChangesRequested:
		return dotFilled, StyleStatusChanges
	case github.StatusConflict:
		return dotFilled, StyleStatusConflict
	default:
		return dotFilled, StyleStatusPending
	}
}

func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	default:
		return fmt.Sprintf("%dw", int(d.Hours()/(24*7)))
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Commands

func navigateToDetail(pr github.PR, focusComment bool) tea.Cmd {
	return func() tea.Msg {
		return NavigateToDetailMsg{PR: pr, FocusComment: focusComment}
	}
}

func loadMoreCmd(client *github.Client, cache *github.Cache, repo, cursor string) tea.Cmd {
	return func() tea.Msg {
		prs, hasNext, endCursor, err := client.LoadMorePRs(repo, cursor, cache)
		return MorePRsLoadedMsg{PRs: prs, HasNext: hasNext, EndCursor: endCursor, Repo: repo, Err: err}
	}
}
