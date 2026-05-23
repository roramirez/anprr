package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/roramirez/anprr/internal/diff"
	"github.com/roramirez/anprr/internal/github"
)

type detailState int

const (
	detailStateLoading        detailState = iota
	detailStateReady                      // normal scroll mode
	detailStateLineSelect                 // line-by-line cursor mode (press n to enter)
	detailStateApproveConfirm             // confirmation prompt before approving
	detailStateMergeConfirm               // confirmation prompt before merging
	detailStateCommentInput               // text input for inline or general comment
	detailStateSubmitting
)

type pendingAction int

const (
	actionNone           pendingAction = iota
	actionApprove                      // approve with optional comment
	actionRequestChanges               // request changes with required comment
	actionComment                      // standalone comment
	actionInlineComment                // comment on a specific diff line
)

type viewMode int

const (
	viewUnified viewMode = iota
	viewSplit
)

type detailTab int

const (
	detailTabDiff        detailTab = iota // [1] diff view
	detailTabDescription                  // [2] PR description
	detailTabComments                     // [3] comments
)

const (
	textareaHeight     = 5 // visible lines in the comment box
	textareaCharLimit  = 65536
	inputWidthOffset   = 4 // border + padding consumed by the textarea box
	borderWidthOffset  = 2 // gutter/border consumed by the viewport and header
	vpHeightOffset     = 5 // rows consumed by header, tabs, and footer
	textareaBorderRows = 2 // top + bottom border of the textarea box
	confirmBoxRows     = 7 // height of the approve/merge confirm prompt box
)

type DetailModel struct {
	state   detailState
	pr      github.PR
	vp      viewport.Model
	spinner spinner.Model
	input   textarea.Model
	pending pendingAction
	width   int
	height  int
	err     error

	// sub-tab navigation
	activeTab detailTab

	// diff data
	diffLines []diff.DiffLine
	diffView  viewMode

	// inline review state
	lineCursor      int                    // index into diffLines for the selected line
	pendingComments []github.InlineComment // accumulated inline comments
	commentedLines  map[int]bool           // diffLine indices that have a pending comment
	syntaxHL        bool
}

func newDetailModel() DetailModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = StyleStatusBarWarn

	ta := textarea.New()
	ta.Placeholder = "Enter your comment…"
	ta.CharLimit = textareaCharLimit
	ta.SetHeight(textareaHeight)
	ta.ShowLineNumbers = false

	return DetailModel{
		state:          detailStateLoading,
		spinner:        sp,
		input:          ta,
		commentedLines: make(map[int]bool),
	}
}

func (m DetailModel) setSize(w, h int) DetailModel {
	m.width = w
	m.height = h
	m.input.SetWidth(w - inputWidthOffset)
	vpH := h - vpHeightOffset
	if vpH < 1 {
		vpH = 1
	}
	m.vp = viewport.New(w-borderWidthOffset, vpH)
	return m
}

// resetToReady transitions back to the ready state, clearing any input or pending action.
// Used when an API call fails or the user escapes while submitting.
func (m DetailModel) resetToReady() DetailModel {
	m.state = detailStateReady
	m.pending = actionNone
	m.input.Reset()
	return m
}

func (m DetailModel) setPR(pr github.PR, focusComment bool) DetailModel {
	m.pr = pr
	if focusComment {
		m.state = detailStateCommentInput
		m.pending = actionRequestChanges
		m.input.Placeholder = "Describe the changes needed…"
		m.input.Focus()
		m.input.SetWidth(m.width - inputWidthOffset)
	}
	return m
}

func (m DetailModel) update(msg tea.Msg, client *github.Client, cache *github.Cache) (DetailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case CommentsLoadedMsg:
		return m.handleCommentsLoaded(msg)
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case tea.KeyMsg:
		return m.handleKeyByState(msg, client, cache)
	default:
		if m.state == detailStateReady || m.state == detailStateLineSelect {
			var cmd tea.Cmd
			m.vp, cmd = m.vp.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m DetailModel) handleCommentsLoaded(msg CommentsLoadedMsg) (DetailModel, tea.Cmd) {
	if msg.Err != nil {
		return m, statusCmd("Error loading comments: "+msg.Err.Error(), true)
	}
	m.pr.Comments = msg.Comments
	m.pr.LineComments = msg.LineComments
	m.pr.CommentsLoaded = true
	if m.activeTab == detailTabComments {
		m.renderComments()
	}
	return m, nil
}

func (m DetailModel) handleKeyByState(msg tea.KeyMsg, client *github.Client, cache *github.Cache) (DetailModel, tea.Cmd) {
	switch m.state {
	case detailStateCommentInput:
		return m.handleCommentInput(msg, client)
	case detailStateLineSelect:
		return m.handleLineSelect(msg, client)
	case detailStateApproveConfirm:
		return m.handleApproveConfirm(msg, client)
	case detailStateMergeConfirm:
		return m.handleMergeConfirm(msg, client)
	case detailStateSubmitting:
		if msg.String() == "b" || msg.String() == keyEsc {
			m = m.resetToReady()
		}
		return m, nil
	case detailStateReady:
		return m.handleReady(msg, client, cache)
	}
	return m, nil
}

// handleApproveConfirm handles the approve confirmation prompt.
//
//	y / enter → approve without extra comment
//	c         → approve with a comment (opens textarea)
//	esc       → cancel
func (m DetailModel) handleApproveConfirm(msg tea.KeyMsg, client *github.Client) (DetailModel, tea.Cmd) {
	switch msg.String() {
	case "y", keyEnter:
		m.state = detailStateSubmitting
		return m, submitReviewCmd(client, m.pr, github.ReviewApprove, "", m.pendingComments)
	case "c":
		m.state = detailStateCommentInput
		m.pending = actionApprove
		m.input.Placeholder = "Optional comment with your approval…"
		m.input.SetWidth(m.width - inputWidthOffset)
		m.input.Focus()
		return m, textarea.Blink
	case keyEsc:
		m.state = detailStateReady
	}
	return m, nil
}

// handleMergeConfirm handles the merge method selection prompt.
//
//	s / enter → squash merge (default)
//	m         → merge commit
//	r         → rebase
//	esc       → cancel
func (m DetailModel) handleMergeConfirm(msg tea.KeyMsg, client *github.Client) (DetailModel, tea.Cmd) {
	switch msg.String() {
	case "s", keyEnter:
		m.state = detailStateSubmitting
		return m, mergePRCmd(client, m.pr, github.MergeMethodSquash)
	case "m":
		m.state = detailStateSubmitting
		return m, mergePRCmd(client, m.pr, github.MergeMethodMerge)
	case "r":
		m.state = detailStateSubmitting
		return m, mergePRCmd(client, m.pr, github.MergeMethodRebase)
	case keyEsc:
		m.state = detailStateReady
	}
	return m, nil
}

func (m DetailModel) handleReady(msg tea.KeyMsg, client *github.Client, cache *github.Cache) (DetailModel, tea.Cmd) {
	switch msg.String() {
	case "1":
		m.activeTab = detailTabDiff
		m.rerender()
		return m, nil
	case "2":
		m.activeTab = detailTabDescription
		m.renderDescription()
		return m, nil
	case "3":
		m.activeTab = detailTabComments
		if !m.pr.CommentsLoaded {
			return m, fetchCommentsCmd(client, m.pr)
		}
		m.renderComments()
		return m, nil
	case "s":
		// toggle between unified and split view
		if m.diffView == viewUnified {
			m.diffView = viewSplit
		} else {
			m.diffView = viewUnified
		}
		m.rerender()
		return m, nil
	case "n":
		// enter line selection mode
		m.state = detailStateLineSelect
		m.lineCursor = m.firstCommentableLine()
		m.rerender()
		return m, nil
	case "a":
		if m.pr.IsDraft {
			return m, statusCmd("Cannot approve a draft PR", true)
		}
		m.state = detailStateApproveConfirm
		return m, nil
	case "m":
		return m.handleMergeKey()
	case "r":
		return m.openCommentInput(actionRequestChanges, "Describe the changes needed…")
	case "c":
		return m.openCommentInput(actionComment, "Enter your comment…")
	case "f":
		cache.Invalidate()
		m.state = detailStateLoading
		return m, fetchDiffCmd(client, cache, m.pr)
	case "w":
		return m, openBrowserCmd(m.pr.URL)
	case "b", keyEsc:
		return m, func() tea.Msg { return NavigateToListMsg{} }
	case "q":
		return m, tea.Quit
	default:
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}
}

func (m DetailModel) handleLineSelect(msg tea.KeyMsg, _ *github.Client) (DetailModel, tea.Cmd) {
	switch msg.String() {
	case "s":
		if m.diffView == viewUnified {
			m.diffView = viewSplit
		} else {
			m.diffView = viewUnified
		}
		m.rerender()
		return m, nil
	case "j", "down":
		m.lineCursor = m.nextCommentable(m.lineCursor + 1)
		m.scrollToCursor()
		m.rerender()
	case "k", "up":
		m.lineCursor = m.prevCommentable(m.lineCursor - 1)
		m.scrollToCursor()
		m.rerender()
	case "n", keyEnter:
		// open comment input for this line
		dl := m.diffLines[m.lineCursor]
		m.state = detailStateCommentInput
		m.pending = actionInlineComment
		m.input.Placeholder = fmt.Sprintf("Comment on %s…", diff.FormatPosition(dl))
		m.input.SetWidth(m.width - inputWidthOffset)
		m.input.Focus()
		return m, textarea.Blink
	case keyEsc, "q":
		m.state = detailStateReady
		m.lineCursor = 0
		m.rerender()
	}
	return m, nil
}

func (m DetailModel) handleMergeKey() (DetailModel, tea.Cmd) {
	if m.pr.IsDraft {
		return m, statusCmd("Cannot merge a draft PR", true)
	}
	if m.pr.Mergeable == "CONFLICTING" {
		return m, statusCmd("PR has conflicts — resolve before merging", true)
	}
	m.state = detailStateMergeConfirm
	return m, nil
}

func (m DetailModel) openCommentInput(action pendingAction, placeholder string) (DetailModel, tea.Cmd) {
	m.state = detailStateCommentInput
	m.pending = action
	m.input.Placeholder = placeholder
	m.input.SetWidth(m.width - inputWidthOffset)
	m.input.Focus()
	return m, textarea.Blink
}

func (m DetailModel) handleCommentInput(msg tea.KeyMsg, client *github.Client) (DetailModel, tea.Cmd) {
	switch msg.String() {
	case "ctrl+d":
		return m.handleCommentSubmit(client)
	case keyEsc:
		return m.handleCommentCancel(), nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m DetailModel) handleCommentSubmit(client *github.Client) (DetailModel, tea.Cmd) {
	body := strings.TrimSpace(m.input.Value())
	if body == "" {
		return m, nil
	}
	m.input.Reset()
	switch m.pending {
	case actionInlineComment:
		dl := m.diffLines[m.lineCursor]
		m.commentedLines[m.lineCursor] = true
		m.pendingComments = append(m.pendingComments, github.InlineComment{
			Path: dl.Path,
			Line: diff.CommentLine(dl),
			Side: diff.CommentSide(dl),
			Body: body,
		})
		m.state = detailStateLineSelect
		m.rerender()
		return m, statusCmd(fmt.Sprintf("✓ Comment added (%d pending)", len(m.pendingComments)), false)
	case actionApprove:
		m.state = detailStateSubmitting
		return m, submitReviewCmd(client, m.pr, github.ReviewApprove, body, m.pendingComments)
	case actionRequestChanges:
		m.state = detailStateSubmitting
		return m, submitReviewCmd(client, m.pr, github.ReviewRequestChanges, body, m.pendingComments)
	case actionComment:
		m.state = detailStateSubmitting
		owner, repo := splitRepo(m.pr.Repo)
		return m, postCommentCmd(client, owner, repo, m.pr.Number, body)
	}
	return m, nil
}

func (m DetailModel) handleCommentCancel() DetailModel {
	switch m.pending {
	case actionInlineComment:
		m.state = detailStateLineSelect
	case actionApprove:
		m.state = detailStateApproveConfirm
	default:
		m.state = detailStateReady
	}
	m.input.Reset()
	m.pending = actionNone
	return m
}

func footerExtraHeight(state detailState) int {
	switch state {
	case detailStateCommentInput:
		return textareaHeight + textareaBorderRows
	case detailStateApproveConfirm, detailStateMergeConfirm:
		return confirmBoxRows
	}
	return 0
}

func updateDetailDiff(m DetailModel, msg DiffLoadedMsg, syntaxHL bool) (DetailModel, tea.Cmd) {
	if msg.Err != nil {
		m.err = msg.Err
		m.state = detailStateReady
		return m, statusCmd("Error loading diff: "+msg.Err.Error(), true)
	}
	m.syntaxHL = syntaxHL
	m.diffLines = diff.Parse(msg.RawDiff)
	m.commentedLines = make(map[int]bool)
	m.rerender()
	m.state = detailStateReady
	m.err = nil
	return m, nil
}

// rerender rebuilds the viewport content reflecting current view mode, cursor and comments.
func (m *DetailModel) rerender() {
	if len(m.diffLines) == 0 {
		return
	}
	var hl diff.Highlighter = diff.NoopHighlighter{}
	if m.syntaxHL {
		hl = diff.NewChromaHighlighter()
	}
	cursor := -1
	if m.state == detailStateLineSelect || m.state == detailStateCommentInput {
		cursor = m.lineCursor
	}
	var rendered string
	if m.diffView == viewSplit {
		rendered = diff.RenderSplit(m.diffLines, m.width, hl, cursor, m.commentedLines)
	} else {
		rendered = diff.Render(m.diffLines, m.width-borderWidthOffset, hl, cursor, m.commentedLines)
	}
	m.vp.SetContent(rendered)
}

func (m *DetailModel) renderDescription() {
	body := m.pr.Body
	if body == "" {
		body = "_No description provided._"
	}
	rendered, err := glamour.Render(body, "dark")
	if err != nil {
		rendered = body
	}
	m.vp.SetContent(rendered)
	m.vp.GotoTop()
}

func (m *DetailModel) renderComments() {
	if !m.pr.CommentsLoaded {
		m.vp.SetContent(StyleStatusBar.Render("  Loading comments…"))
		return
	}
	total := len(m.pr.Comments) + len(m.pr.LineComments)
	if total == 0 {
		m.vp.SetContent(StyleStatusBar.Render("  No comments."))
		return
	}
	var sb strings.Builder
	sep := strings.Repeat("─", m.vp.Width)

	for _, c := range m.pr.Comments {
		renderCommentBlock(&sb, c.Author.Login, timeAgo(c.CreatedAt), c.Body, sep)
	}
	for _, rc := range m.pr.LineComments {
		loc := rc.Path
		if rc.Line > 0 {
			loc += fmt.Sprintf(":%d", rc.Line)
		}
		renderCommentBlock(&sb, rc.Author.Login, loc, rc.Body, sep)
	}
	m.vp.SetContent(sb.String())
	m.vp.GotoTop()
}

func renderCommentBlock(sb *strings.Builder, author, meta, body, sep string) {
	sb.WriteString(StyleHelpKey.Render(author))
	sb.WriteString(StylePRAge.Render("  " + meta))
	sb.WriteByte('\n')
	for _, line := range strings.Split(strings.TrimSpace(body), "\n") {
		sb.WriteString("  " + StylePRTitle.Render(line) + "\n")
	}
	sb.WriteString(StylePRAge.Render(sep) + "\n")
}

func (m DetailModel) firstCommentableLine() int {
	for i, dl := range m.diffLines {
		if dl.Commentable {
			return i
		}
	}
	return 0
}

func (m DetailModel) nextCommentable(from int) int {
	for i := from; i < len(m.diffLines); i++ {
		if m.diffLines[i].Commentable {
			return i
		}
	}
	return m.lineCursor // stay if no next
}

func (m DetailModel) prevCommentable(from int) int {
	for i := from; i >= 0; i-- {
		if m.diffLines[i].Commentable {
			return i
		}
	}
	return m.lineCursor // stay if no prev
}

func (m *DetailModel) scrollToCursor() {
	// each diff line renders as 1 line in the viewport
	targetY := m.lineCursor - m.vp.Height/2
	if targetY < 0 {
		targetY = 0
	}
	m.vp.SetYOffset(targetY)
}

func (m DetailModel) view(width, height int, statusBar string) string {
	header := m.renderHeader(width)
	tabs := m.renderTabs()
	footer := m.renderFooter(width, statusBar)
	headerH := lipgloss.Height(header) + lipgloss.Height(tabs)
	footerH := lipgloss.Height(footer) + footerExtraHeight(m.state)
	bodyH := height - headerH - footerH
	if bodyH < 1 {
		bodyH = 1
	}
	m.vp.Width = width - 2
	m.vp.Height = bodyH

	var body string
	if m.state == detailStateLoading {
		body = lipgloss.Place(width, bodyH, lipgloss.Center, lipgloss.Center,
			m.spinner.View()+" Loading…")
	} else {
		body = m.vp.View()
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, tabs, body, footer)
}

func (m DetailModel) renderTabs() string {
	totalComments := len(m.pr.Comments) + len(m.pr.LineComments)
	tabs := []struct {
		label string
		idx   detailTab
	}{
		{"  [1] Diff  ", detailTabDiff},
		{"  [2] Description  ", detailTabDescription},
		{fmt.Sprintf("  [3] Comments (%d)  ", totalComments), detailTabComments},
	}
	var out strings.Builder
	for _, tab := range tabs {
		if m.activeTab == tab.idx {
			out.WriteString(StyleTabActive.Render(tab.label))
		} else {
			out.WriteString(StyleTabInactive.Render(tab.label))
		}
	}
	return out.String()
}

func (m DetailModel) renderHeader(width int) string {
	title := fmt.Sprintf(" #%d %s — %s", m.pr.Number, m.pr.Title, m.pr.Repo)
	pending := ""
	if len(m.pendingComments) > 0 {
		pending = StyleStatusBarWarn.Render(fmt.Sprintf("  [%d comment(s) pending]", len(m.pendingComments)))
	}
	checkLabel := renderCheckLabel(m.pr.CheckState)
	meta := fmt.Sprintf("author: %s  base: %s  +%d -%d",
		m.pr.Author.Login, m.pr.BaseRef, m.pr.Additions, m.pr.Deletions)
	if m.pr.Mergeable == "CONFLICTING" {
		meta += "  " + StyleStatusConflict.Render("⚠ conflicts")
	}
	if checkLabel != "" {
		meta += "  " + checkLabel
	}
	if len(title) > width-2 {
		title = title[:width-3] + "…"
	}
	return StyleHeader.Width(width).Render(title+pending) + "\n" +
		StyleStatusBar.Render(meta)
}

func renderCheckLabel(state string) string {
	switch state {
	case "SUCCESS":
		return StyleCheckSuccess.Render("✓ checks passed")
	case "FAILURE", "ERROR":
		return StyleCheckFailure.Render("✗ checks failed")
	case "PENDING", "IN_PROGRESS", "QUEUED":
		return StyleCheckPending.Render("○ checks running")
	case "EXPECTED":
		return StyleCheckNone.Render("— no checks")
	default:
		return ""
	}
}

// renderConfirmBox renders a rounded green-bordered prompt box.
func renderConfirmBox(width int, content string) string {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#00FF7F")).
		Padding(0, 2).
		Width(width - borderWidthOffset).
		Render(content)
}

func (m DetailModel) renderFooter(width int, statusBar string) string {
	switch m.state {
	case detailStateMergeConfirm:
		return m.renderMergeConfirmFooter(width)
	case detailStateApproveConfirm:
		return m.renderApproveConfirmFooter(width)
	case detailStateCommentInput:
		return m.renderCommentInputFooter(width)
	}
	keys := m.renderReadyKeys()
	if statusBar != "" {
		return keys + "\n" + statusBar
	}
	return keys
}

func (m DetailModel) renderMergeConfirmFooter(width int) string {
	return renderConfirmBox(width,
		StyleStatusBarOK.Render("Merge PR #"+fmt.Sprint(m.pr.Number)+"?")+"\n\n"+
			StyleHelpKey.Render("s / enter")+StyleHelpDesc.Render("Squash and merge (recommended)")+"\n"+
			StyleHelpKey.Render("m        ")+StyleHelpDesc.Render("Merge commit")+"\n"+
			StyleHelpKey.Render("r        ")+StyleHelpDesc.Render("Rebase and merge")+"\n"+
			StyleHelpKey.Render("esc      ")+StyleHelpDesc.Render("Cancel"),
	)
}

func (m DetailModel) renderApproveConfirmFooter(width int) string {
	pending := ""
	if len(m.pendingComments) > 0 {
		pending = fmt.Sprintf("  (%d inline comment(s) will be included)", len(m.pendingComments))
	}
	return renderConfirmBox(width,
		StyleStatusBarOK.Render("Approve PR #"+fmt.Sprint(m.pr.Number)+"?")+pending+"\n\n"+
			StyleHelpKey.Render("y / enter")+StyleHelpDesc.Render("Approve now")+"\n"+
			StyleHelpKey.Render("c        ")+StyleHelpDesc.Render("Approve with a comment")+"\n"+
			StyleHelpKey.Render("esc      ")+StyleHelpDesc.Render("Cancel"),
	)
}

func (m DetailModel) renderCommentInputFooter(width int) string {
	pos := ""
	if m.pending == actionInlineComment && m.lineCursor < len(m.diffLines) {
		pos = StyleHelpKey.Render(diff.FormatPosition(m.diffLines[m.lineCursor])) + "  "
	}
	keys := pos + StyleFooter.Render("ctrl+d=submit  esc=cancel  enter=new line")
	taBox := StyleTextareaBox.Width(width - borderWidthOffset).Render(m.input.View())
	return keys + "\n" + taBox
}

func (m DetailModel) renderReadyKeys() string {
	switch m.state {
	case detailStateLineSelect:
		return StyleFooter.Render("j/k=move  n/enter=comment here  esc=exit line mode")
	case detailStateSubmitting:
		return StyleFooter.Render(m.spinner.View() + " Submitting…  esc=cancel")
	default:
		viewLabel := "[unified]"
		if m.diffView == viewSplit {
			viewLabel = "[split]"
		}
		return StyleFooter.Render(fmt.Sprintf(
			"1/2/3=tab  s=%s  n=inline  a=approve  m=merge  r=changes  c=comment  w=web  b=back  ?=help  q=quit",
			viewLabel,
		))
	}
}

// Commands

func submitReviewCmd(client *github.Client, pr github.PR, event github.ReviewEvent, body string, inline []github.InlineComment) tea.Cmd {
	return func() tea.Msg {
		owner, repo := splitRepo(pr.Repo)
		err := client.SubmitReview(owner, repo, pr.Number, event, body, inline)
		return ReviewDoneMsg{Err: err}
	}
}

func postCommentCmd(client *github.Client, owner, repo string, number int, body string) tea.Cmd {
	return func() tea.Msg {
		err := client.PostComment(owner, repo, number, body)
		return CommentDoneMsg{Err: err}
	}
}

func mergePRCmd(client *github.Client, pr github.PR, method github.MergeMethod) tea.Cmd {
	return func() tea.Msg {
		owner, repo := splitRepo(pr.Repo)
		err := client.MergePR(owner, repo, pr.Number, method)
		return MergeDoneMsg{Err: err}
	}
}

func fetchCommentsCmd(client *github.Client, pr github.PR) tea.Cmd {
	return func() tea.Msg {
		owner, repo := splitRepo(pr.Repo)
		comments, reviewComments, err := client.FetchComments(owner, repo, pr.Number)
		return CommentsLoadedMsg{Comments: comments, LineComments: reviewComments, Err: err}
	}
}

func openBrowserCmd(url string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", url)
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		default:
			cmd = exec.Command("xdg-open", url)
		}
		if err := cmd.Start(); err != nil {
			return nil
		}
		return nil
	}
}
