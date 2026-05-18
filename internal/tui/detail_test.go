package tui

import (
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
