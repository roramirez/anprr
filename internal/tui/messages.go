package tui

import (
	"time"

	"github.com/roramirez/anprr/internal/github"
)

// PRsLoadedMsg is sent when the PR list fetch completes.
type PRsLoadedMsg struct {
	PRs []github.PR
	Err error
}

// MorePRsLoadedMsg is sent when load-more fetch completes.
type MorePRsLoadedMsg struct {
	PRs       []github.PR
	HasNext   bool
	EndCursor string
	Repo      string
	Err       error
}

// DiffLoadedMsg is sent when a diff fetch completes.
type DiffLoadedMsg struct {
	RawDiff string
	Err     error
}

// ReviewDoneMsg is sent when a review submission completes.
type ReviewDoneMsg struct {
	Err error
}

// CommentDoneMsg is sent when a comment post completes.
type CommentDoneMsg struct {
	Err error
}

// CurrentUserMsg is sent once at startup with the authenticated user login.
type CurrentUserMsg struct {
	Login string
	Err   error
}

// NavigateToDetailMsg triggers navigation to the detail screen.
type NavigateToDetailMsg struct {
	PR           github.PR
	FocusComment bool // true when coming from 'r' on list screen
}

// NavigateToListMsg triggers navigation back to the list screen.
type NavigateToListMsg struct{}

// StatusMsg shows a transient message in the status bar.
type StatusMsg struct {
	Text    string
	IsError bool
	IsOK    bool
}

// ClearStatusMsg clears the status bar after a timeout.
type ClearStatusMsg struct{}

// ReviewRequestedLoadedMsg carries the result of the Search API fetch.
type ReviewRequestedLoadedMsg struct {
	Set map[string]bool
	Err error
}

// TickMsg fires periodically for auto-refresh.
type TickMsg time.Time

// ToggleHelpMsg shows/hides the help overlay.
type ToggleHelpMsg struct{}

// WindowTooSmallMsg fires when the terminal is below minimum size.
type WindowTooSmallMsg struct {
	Width  int
	Height int
}
