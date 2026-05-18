package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/roramirez/anprr/internal/github"
)

func loadedDetail() DetailModel {
	m := newDetailModel()
	m = m.setSize(120, 40)
	m, _ = updateDetailDiff(m, DiffLoadedMsg{
		RawDiff: "diff --git a/auth/token.go b/auth/token.go\n--- a/auth/token.go\n+++ b/auth/token.go\n@@ -10,3 +10,4 @@\n ctx1\n-old line\n+new line\n ctx2\n",
	}, false)
	return m
}

func TestDetailModel_initialStateIsLoading(t *testing.T) {
	m := newDetailModel()
	if m.state != detailStateLoading {
		t.Errorf("expected loading, got %d", m.state)
	}
}

func TestDetailModel_diffLoadedTransitionsToReady(t *testing.T) {
	m := loadedDetail()
	if m.state != detailStateReady {
		t.Error("expected ready state after diff loaded")
	}
	if len(m.diffLines) == 0 {
		t.Error("expected diff lines to be populated")
	}
}

func TestDetailModel_scrollOffset(t *testing.T) {
	m := loadedDetail()
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}, nil, nil)
	if m.state != detailStateReady {
		t.Error("state should remain ready after scroll")
	}
}

func TestDetailModel_backKey(t *testing.T) {
	m := loadedDetail()
	_, cmd := m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")}, nil, nil)
	if cmd == nil {
		t.Fatal("expected NavigateToListMsg command")
	}
	if _, ok := cmd().(NavigateToListMsg); !ok {
		t.Errorf("expected NavigateToListMsg")
	}
}

func TestDetailModel_commentInputFocus(t *testing.T) {
	m := loadedDetail()
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")}, nil, nil)
	if m.state != detailStateCommentInput {
		t.Error("expected comment input state after 'c'")
	}
	if m.pending != actionComment {
		t.Errorf("expected actionComment, got %d", m.pending)
	}
}

func TestDetailModel_commentInputEscCancels(t *testing.T) {
	m := loadedDetail()
	m.state = detailStateCommentInput
	m.pending = actionComment
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyEsc}, nil, nil)
	if m.state != detailStateReady {
		t.Error("expected ready state after esc")
	}
}

func TestDetailModel_requestChangesKey(t *testing.T) {
	m := loadedDetail()
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")}, nil, nil)
	if m.state != detailStateCommentInput {
		t.Error("expected comment input state after 'r'")
	}
	if m.pending != actionRequestChanges {
		t.Errorf("expected actionRequestChanges, got %d", m.pending)
	}
}

func TestDetailModel_draftCannotApprove(t *testing.T) {
	m := loadedDetail()
	m.pr = github.PR{IsDraft: true, Repo: "org/repo"}
	_, cmd := m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}, nil, nil)
	if cmd == nil {
		t.Fatal("expected a command")
	}
	if msg, ok := cmd().(StatusMsg); !ok || !msg.IsError {
		t.Error("expected error status for draft approve")
	}
}

func TestDetailModel_focusCommentOnInit(t *testing.T) {
	m := newDetailModel()
	m = m.setSize(120, 40)
	m = m.setPR(github.PR{Repo: "org/repo"}, true)
	if m.state != detailStateCommentInput {
		t.Error("expected comment input when focusComment=true")
	}
}

// Sub-tab tests

func TestDetailModel_tabSwitch(t *testing.T) {
	m := loadedDetail()

	// default is diff tab
	if m.activeTab != detailTabDiff {
		t.Errorf("expected detailTabDiff by default, got %d", m.activeTab)
	}

	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")}, nil, nil)
	if m.activeTab != detailTabDescription {
		t.Errorf("expected detailTabDescription after '2', got %d", m.activeTab)
	}

	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")}, nil, nil)
	if m.activeTab != detailTabComments {
		t.Errorf("expected detailTabComments after '3', got %d", m.activeTab)
	}

	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")}, nil, nil)
	if m.activeTab != detailTabDiff {
		t.Errorf("expected detailTabDiff after '1', got %d", m.activeTab)
	}
}

func TestDetailModel_tab3EmitsCommandWhenNotLoaded(t *testing.T) {
	m := loadedDetail()
	m.pr.CommentsLoaded = false

	_, cmd := m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")}, nil, nil)
	if cmd == nil {
		t.Error("expected fetchCommentsCmd when comments not loaded")
	}
}

func TestDetailModel_tab3NoCommandWhenAlreadyLoaded(t *testing.T) {
	m := loadedDetail()
	m.pr.CommentsLoaded = true
	m.pr.Comments = []github.Comment{
		{Author: github.User{Login: "alice"}, Body: "test comment"},
	}

	_, cmd := m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")}, nil, nil)
	// no fetch needed — comments already loaded
	if cmd != nil {
		t.Error("expected no cmd when comments already loaded")
	}
}

func TestDetailModel_commentsLoadedMsg(t *testing.T) {
	m := loadedDetail()
	m.activeTab = detailTabComments

	msg := CommentsLoadedMsg{
		Comments: []github.Comment{
			{Author: github.User{Login: "alice"}, Body: "LGTM"},
		},
		LineComments: []github.LineComment{
			{Author: github.User{Login: "bob"}, Body: "nil check", Path: "auth.go", Line: 10},
		},
	}
	m, _ = m.update(msg, nil, nil)

	if !m.pr.CommentsLoaded {
		t.Error("expected CommentsLoaded = true")
	}
	if len(m.pr.Comments) != 1 || m.pr.Comments[0].Author.Login != "alice" {
		t.Errorf("comments not populated: %v", m.pr.Comments)
	}
	if len(m.pr.LineComments) != 1 || m.pr.LineComments[0].Path != "auth.go" {
		t.Errorf("lineComments not populated: %v", m.pr.LineComments)
	}
}

func TestDetailModel_tab2ViewportHasContent(t *testing.T) {
	m := loadedDetail()
	m.pr.Body = "## Summary\n\nThis PR adds a new feature."

	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")}, nil, nil)
	if m.activeTab != detailTabDescription {
		t.Fatalf("expected description tab")
	}
	content := m.vp.View()
	// glamour renders the body — at minimum the title text should appear
	if !strings.Contains(content, "Summary") {
		t.Errorf("expected rendered description in viewport, got: %q", content[:min(len(content), 100)])
	}
}

func TestDetailModel_tab3ViewportHasCommentAuthors(t *testing.T) {
	m := loadedDetail()
	m.pr.CommentsLoaded = true
	m.pr.Comments = []github.Comment{
		{Author: github.User{Login: "reviewer1"}, Body: "Nice work!"},
	}
	m.pr.LineComments = []github.LineComment{
		{Author: github.User{Login: "reviewer2"}, Body: "Fix this", Path: "main.go", Line: 5},
	}

	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")}, nil, nil)
	content := m.vp.View()
	if !strings.Contains(content, "reviewer1") {
		t.Errorf("expected reviewer1 in comments viewport")
	}
	if !strings.Contains(content, "reviewer2") {
		t.Errorf("expected reviewer2 in comments viewport")
	}
}

// View mode tests

func TestDetailModel_toggleSplitView(t *testing.T) {
	m := loadedDetail()
	if m.diffView != viewUnified {
		t.Error("expected unified view by default")
	}
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")}, nil, nil)
	if m.diffView != viewSplit {
		t.Error("expected split view after 's'")
	}
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")}, nil, nil)
	if m.diffView != viewUnified {
		t.Error("expected unified view after second 's'")
	}
}

func TestDetailModel_splitViewToggleInLineSelect(t *testing.T) {
	m := loadedDetail()
	m.state = detailStateLineSelect
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")}, nil, nil)
	if m.diffView != viewSplit {
		t.Error("expected split view toggled from line select mode")
	}
}

// Inline comment tests

func TestDetailModel_enterLineSelectMode(t *testing.T) {
	m := loadedDetail()
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}, nil, nil)
	if m.state != detailStateLineSelect {
		t.Errorf("expected lineSelect state after 'n', got %d", m.state)
	}
	// cursor should be on a commentable line
	if m.lineCursor >= len(m.diffLines) {
		t.Error("lineCursor out of bounds")
	}
	if !m.diffLines[m.lineCursor].Commentable {
		t.Error("lineCursor should point to a commentable line")
	}
}

func TestDetailModel_lineSelectEscReturnsToReady(t *testing.T) {
	m := loadedDetail()
	m.state = detailStateLineSelect
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyEsc}, nil, nil)
	if m.state != detailStateReady {
		t.Errorf("expected ready after esc, got %d", m.state)
	}
}

func TestDetailModel_lineSelectNavigation(t *testing.T) {
	m := loadedDetail()
	m.state = detailStateLineSelect
	m.lineCursor = m.firstCommentableLine()
	initial := m.lineCursor

	// move down
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}, nil, nil)
	if m.lineCursor <= initial {
		t.Errorf("cursor should have moved down: was %d, now %d", initial, m.lineCursor)
	}
	// all positions should be commentable
	if !m.diffLines[m.lineCursor].Commentable {
		t.Error("cursor should always land on commentable line")
	}

	// move up
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}, nil, nil)
	if m.lineCursor != initial {
		t.Errorf("cursor should have moved back up to %d, got %d", initial, m.lineCursor)
	}
}

func TestDetailModel_inlineCommentAccumulates(t *testing.T) {
	m := loadedDetail()

	// enter line select
	m.state = detailStateLineSelect
	m.lineCursor = m.firstCommentableLine()

	// press n to open comment input
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}, nil, nil)
	if m.state != detailStateCommentInput {
		t.Fatalf("expected comment input, got %d", m.state)
	}
	if m.pending != actionInlineComment {
		t.Fatalf("expected actionInlineComment, got %d", m.pending)
	}

	// type comment body (set value directly on the textarea)
	m.input.SetValue("this needs a nil check")

	// submit with ctrl+d
	m, cmd := m.update(tea.KeyMsg{Type: tea.KeyCtrlD}, nil, nil)
	if m.state != detailStateLineSelect {
		t.Errorf("expected back to lineSelect after inline comment, got %d", m.state)
	}
	if len(m.pendingComments) != 1 {
		t.Errorf("expected 1 pending comment, got %d", len(m.pendingComments))
	}
	if m.pendingComments[0].Body != "this needs a nil check" {
		t.Errorf("body: got %q", m.pendingComments[0].Body)
	}
	// status message should be emitted
	if cmd == nil {
		t.Error("expected status cmd after inline comment")
	}
}

func TestDetailModel_inlineCommentMarksLine(t *testing.T) {
	m := loadedDetail()
	m.state = detailStateLineSelect
	m.lineCursor = m.firstCommentableLine()
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}, nil, nil)
	m.input.SetValue("test comment")
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyCtrlD}, nil, nil)

	if !m.commentedLines[m.firstCommentableLine()] {
		t.Error("expected line to be marked as commented")
	}
}

func TestDetailModel_inlineCommentEscStaysInLineSelect(t *testing.T) {
	m := loadedDetail()
	m.state = detailStateLineSelect
	m.lineCursor = m.firstCommentableLine()
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}, nil, nil)
	if m.state != detailStateCommentInput {
		t.Fatal("expected comment input")
	}
	// esc should go back to line select, not ready
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyEsc}, nil, nil)
	if m.state != detailStateLineSelect {
		t.Errorf("expected lineSelect after esc from inline input, got %d", m.state)
	}
	if len(m.pendingComments) != 0 {
		t.Error("no comment should be added on esc")
	}
}
