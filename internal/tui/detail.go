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
	"github.com/charmbracelet/lipgloss"
	"github.com/roramirez/anprr/internal/diff"
	"github.com/roramirez/anprr/internal/github"
)

type detailState int

const (
	detailStateLoading      detailState = iota
	detailStateReady                    // normal scroll mode
	detailStateLineSelect               // line-by-line cursor mode (press n to enter)
	detailStateApproveConfirm           // confirmation prompt before approving
	detailStateCommentInput             // text input for inline or general comment
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

const textareaHeight = 5 // visible lines in the comment box

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

	// diff data
	diffLines []diff.DiffLine
	diffView viewMode

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
	ta.CharLimit = 65536
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
	m.input.SetWidth(w - 4) // -4 for border + padding
	vpH := h - 5
	if vpH < 1 {
		vpH = 1
	}
	m.vp = viewport.New(w-2, vpH) // -2 for gutter
	return m
}

func (m DetailModel) setPR(pr github.PR, focusComment bool) DetailModel {
	m.pr = pr
	if focusComment {
		m.state = detailStateCommentInput
		m.pending = actionRequestChanges
		m.input.Placeholder = "Describe the changes needed…"
		m.input.Focus()
		m.input.SetWidth(m.width - 4)
	}
	return m
}

func (m DetailModel) update(msg tea.Msg, client *github.Client, cache *github.Cache) (DetailModel, tea.Cmd) {
	switch msg := msg.(type) {

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		switch m.state {

		case detailStateCommentInput:
			return m.handleCommentInput(msg, client)

		case detailStateLineSelect:
			return m.handleLineSelect(msg, client)

		case detailStateApproveConfirm:
			return m.handleApproveConfirm(msg, client)

		case detailStateReady:
			return m.handleReady(msg, client, cache)
		}

	default:
		if m.state == detailStateReady || m.state == detailStateLineSelect {
			var cmd tea.Cmd
			m.vp, cmd = m.vp.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

// handleApproveConfirm handles the approve confirmation prompt.
//   y / enter → approve without extra comment
//   c         → approve with a comment (opens textarea)
//   esc       → cancel
func (m DetailModel) handleApproveConfirm(msg tea.KeyMsg, client *github.Client) (DetailModel, tea.Cmd) {
	switch msg.String() {
	case "y", "enter":
		m.state = detailStateSubmitting
		return m, submitReviewCmd(client, m.pr, github.ReviewApprove, "", m.pendingComments)
	case "c":
		m.state = detailStateCommentInput
		m.pending = actionApprove
		m.input.Placeholder = "Optional comment with your approval…"
		m.input.SetWidth(m.width - 4)
		m.input.Focus()
		return m, textarea.Blink
	case "esc":
		m.state = detailStateReady
	}
	return m, nil
}

func (m DetailModel) handleReady(msg tea.KeyMsg, client *github.Client, cache *github.Cache) (DetailModel, tea.Cmd) {
	switch msg.String() {
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
	case "r":
		m.state = detailStateCommentInput
		m.pending = actionRequestChanges
		m.input.Placeholder = "Describe the changes needed…"
		m.input.SetWidth(m.width - 4)
		m.input.Focus()
		return m, textarea.Blink
	case "c":
		m.state = detailStateCommentInput
		m.pending = actionComment
		m.input.Placeholder = "Enter your comment…"
		m.input.SetWidth(m.width - 4)
		m.input.Focus()
		return m, textarea.Blink
	case "f":
		cache.Invalidate()
		m.state = detailStateLoading
		return m, fetchDiffCmd(client, cache, m.pr)
	case "w":
		return m, openBrowserCmd(m.pr.URL)
	case "b", "esc":
		return m, func() tea.Msg { return NavigateToListMsg{} }
	case "q":
		return m, tea.Quit
	default:
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}
}

func (m DetailModel) handleLineSelect(msg tea.KeyMsg, client *github.Client) (DetailModel, tea.Cmd) {
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
	case "n", "enter":
		// open comment input for this line
		dl := m.diffLines[m.lineCursor]
		m.state = detailStateCommentInput
		m.pending = actionInlineComment
		m.input.Placeholder = fmt.Sprintf("Comment on %s…", diff.FormatPosition(dl))
		m.input.SetWidth(m.width - 4)
		m.input.Focus()
		return m, textarea.Blink
	case "esc", "q":
		m.state = detailStateReady
		m.lineCursor = 0
		m.rerender()
	}
	return m, nil
}

func (m DetailModel) handleCommentInput(msg tea.KeyMsg, client *github.Client) (DetailModel, tea.Cmd) {
	switch msg.String() {
	case "ctrl+d":
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

	case "esc":
		switch m.pending {
		case actionInlineComment:
			m.state = detailStateLineSelect
		case actionApprove:
			m.state = detailStateApproveConfirm // go back to confirm prompt
		default:
			m.state = detailStateReady
		}
		m.input.Reset()
		m.pending = actionNone
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
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
		rendered = diff.Render(m.diffLines, m.width-2, hl, cursor, m.commentedLines)
	}
	m.vp.SetContent(rendered)
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
	footer := m.renderFooter(width, statusBar)
	headerH := lipgloss.Height(header)
	footerH := lipgloss.Height(footer)
	// reserve extra space for the textarea box or approve confirm prompt
	switch m.state {
	case detailStateCommentInput:
		footerH += textareaHeight + 2 // +2 for border
	case detailStateApproveConfirm:
		footerH += 6 // prompt box height
	}
	bodyH := height - headerH - footerH
	if bodyH < 1 {
		bodyH = 1
	}
	m.vp.Width = width - 2
	m.vp.Height = bodyH

	var body string
	if m.state == detailStateLoading {
		body = lipgloss.Place(width, bodyH, lipgloss.Center, lipgloss.Center,
			m.spinner.View()+" Loading diff…")
	} else {
		body = m.vp.View()
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (m DetailModel) renderHeader(width int) string {
	title := fmt.Sprintf(" #%d %s — %s", m.pr.Number, m.pr.Title, m.pr.Repo)
	pending := ""
	if len(m.pendingComments) > 0 {
		pending = StyleStatusBarWarn.Render(fmt.Sprintf("  [%d comment(s) pending]", len(m.pendingComments)))
	}
	meta := fmt.Sprintf("author: %s  base: %s  +%d -%d",
		m.pr.Author.Login, m.pr.BaseRef, m.pr.Additions, m.pr.Deletions)
	if len(title) > width-2 {
		title = title[:width-3] + "…"
	}
	return StyleHeader.Width(width).Render(title+pending) + "\n" +
		StyleStatusBar.Render(meta)
}

func (m DetailModel) renderFooter(width int, statusBar string) string {
	var keys string
	switch m.state {
	case detailStateApproveConfirm:
		pending := ""
		if len(m.pendingComments) > 0 {
			pending = fmt.Sprintf("  (%d inline comment(s) will be included)", len(m.pendingComments))
		}
		prompt := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#00FF7F")).
			Padding(0, 2).
			Width(width - 2).
			Render(
				StyleStatusBarOK.Render("Approve PR #"+fmt.Sprint(m.pr.Number)+"?") + pending + "\n\n" +
					StyleHelpKey.Render("y / enter") + StyleHelpDesc.Render("Approve now") + "\n" +
					StyleHelpKey.Render("c        ") + StyleHelpDesc.Render("Approve with a comment") + "\n" +
					StyleHelpKey.Render("esc      ") + StyleHelpDesc.Render("Cancel"),
			)
		return prompt

	case detailStateCommentInput:
		pos := ""
		if m.pending == actionInlineComment && m.lineCursor < len(m.diffLines) {
			pos = StyleHelpKey.Render(diff.FormatPosition(m.diffLines[m.lineCursor])) + "  "
		}
		keys = pos + StyleFooter.Render("ctrl+d=submit  esc=cancel  enter=new line")
		taBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#00BFFF")).
			Width(width - 2).
			Render(m.input.View())
		return keys + "\n" + taBox
	case detailStateLineSelect:
		keys = StyleFooter.Render("j/k=move  n/enter=comment here  esc=exit line mode")
	case detailStateSubmitting:
		keys = StyleFooter.Render(m.spinner.View() + " Submitting…")
	default:
		viewLabel := "[unified]"
		if m.diffView == viewSplit {
			viewLabel = "[split]"
		}
		keys = StyleFooter.Render(fmt.Sprintf(
			"s=%s  n=inline  a=approve  r=changes  c=comment  f=refresh  w=web  b=back  ?=help  q=quit",
			viewLabel,
		))
	}
	sb := ""
	if statusBar != "" {
		sb = "\n" + statusBar
	}
	_ = width
	return keys + sb
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
		cmd.Start()
		return nil
	}
}
