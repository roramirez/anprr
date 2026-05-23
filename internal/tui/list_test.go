package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/roramirez/anprr/internal/github"
)

func makePR(number int, author, repo string, draft bool, status github.PRStatus) github.PR {
	return github.PR{
		Number:       number,
		Title:        fmt.Sprintf("PR #%d", number),
		Repo:         repo,
		Author:       github.User{Login: author},
		IsDraft:      draft,
		ReviewStatus: status,
		UpdatedAt:    time.Now(),
	}
}

func TestListModel_initialStateIsLoading(t *testing.T) {
	m := newListModel()
	if m.state != listStateLoading {
		t.Errorf("expected loading state, got %d", m.state)
	}
}

func TestListModel_PRsLoadedTransitionsToReady(t *testing.T) {
	m := newListModel()
	m = m.setCurrentUser("alice")
	prs := []github.PR{makePR(1, "alice", "org/repo", false, github.StatusPending)}
	m, _ = updateListModel(m, PRsLoadedMsg{PRs: prs})
	if m.state != listStateReady {
		t.Error("expected ready state after PRsLoadedMsg")
	}
	if len(m.allPRs) != 1 {
		t.Errorf("expected 1 PR, got %d", len(m.allPRs))
	}
}

func TestListModel_tabSwitch(t *testing.T) {
	m := newListModel()
	m.state = listStateReady
	m = m.setCurrentUser("alice")
	m.allPRs = []github.PR{
		makePR(1, "alice", "org/repo", false, github.StatusPending), // mine
		makePR(2, "bob", "org/repo", false, github.StatusPending),   // not mine
	}

	// 1 → My PRs
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")}, nil, nil, nil)
	if m.tab != tabMyPRs {
		t.Errorf("expected tabMyPRs, got %d", m.tab)
	}
	if prs := m.visiblePRs(); len(prs) != 1 || prs[0].Number != 1 {
		t.Errorf("My PRs: expected PR #1, got %v", prs)
	}

	// 2 → Needs Review
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")}, nil, nil, nil)
	if m.tab != tabNeedsReview {
		t.Errorf("expected tabNeedsReview, got %d", m.tab)
	}

	// 3 → All Open
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")}, nil, nil, nil)
	if m.tab != tabAllOpen {
		t.Errorf("expected tabAllOpen, got %d", m.tab)
	}
	if prs := m.visiblePRs(); len(prs) != 1 || prs[0].Number != 2 {
		t.Errorf("All Open: expected PR #2, got %v", prs)
	}
}

func TestListModel_cursorNavigation(t *testing.T) {
	m := newListModel()
	m.state = listStateReady
	m = m.setCurrentUser("alice")
	m.allPRs = []github.PR{
		makePR(1, "alice", "org/repo", false, github.StatusPending),
		makePR(2, "alice", "org/repo", false, github.StatusPending),
		makePR(3, "alice", "org/repo", false, github.StatusPending),
	}

	// move down
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}, nil, nil, nil)
	if m.cursor != 1 {
		t.Errorf("cursor: got %d, want 1", m.cursor)
	}
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}, nil, nil, nil)
	if m.cursor != 2 {
		t.Errorf("cursor: got %d, want 2", m.cursor)
	}
	// clamp at bottom
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}, nil, nil, nil)
	if m.cursor != 2 {
		t.Errorf("cursor should not exceed len-1, got %d", m.cursor)
	}
	// move up
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}, nil, nil, nil)
	if m.cursor != 1 {
		t.Errorf("cursor: got %d, want 1", m.cursor)
	}
	// clamp at top
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}, nil, nil, nil)
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}, nil, nil, nil)
	if m.cursor != 0 {
		t.Errorf("cursor should not go below 0, got %d", m.cursor)
	}
}

// Tab 2 — All Open

func TestListModel_allOpenTab(t *testing.T) {
	m := newListModel()
	m.state = listStateReady
	m = m.setCurrentUser("alice")
	m.allPRs = []github.PR{
		makePR(1, "alice", "org/repo", false, github.StatusPending),  // mine
		makePR(2, "bob", "org/repo", false, github.StatusPending),    // not mine
		makePR(3, "carol", "org/repo", false, github.StatusApproved), // not mine, approved
	}

	// switch to tab 3 (All Open)
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")}, nil, nil, nil)
	if m.tab != tabAllOpen {
		t.Fatalf("expected tabAllOpen, got %d", m.tab)
	}
	prs := m.visiblePRs()
	if len(prs) != 2 {
		t.Errorf("expected 2 PRs in All Open (bob+carol), got %d", len(prs))
	}
}

// Tab 3 — Needs Review (Search API + re-review)

func TestListModel_tab3Key(t *testing.T) {
	m := newListModel()
	m.state = listStateReady
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")}, nil, nil, nil)
	if m.tab != tabAllOpen {
		t.Errorf("expected tabAllOpen after '3', got %d", m.tab)
	}
}

func TestListModel_needsReviewUsesSearchSet(t *testing.T) {
	m := newListModel()
	m.state = listStateReady
	m = m.setCurrentUser("alice")
	m.allPRs = []github.PR{
		makePR(42, "bob", "org/repo", false, github.StatusPending),
		makePR(99, "bob", "org/repo", false, github.StatusPending),
	}
	// only PR 42 is in the Search API set
	m = m.setReviewRequestedSet(map[string]bool{"org/repo#42": true})

	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")}, nil, nil, nil)
	prs := m.visiblePRs()
	if len(prs) != 1 || prs[0].Number != 42 {
		t.Errorf("expected only PR #42 in needs-review, got %v", prs)
	}
}

func TestListModel_needsReviewEmptySetShowsNothing(t *testing.T) {
	m := newListModel()
	m.state = listStateReady
	m = m.setCurrentUser("alice")
	m.allPRs = []github.PR{
		makePR(1, "bob", "org/repo", false, github.StatusPending),
	}
	// no search set entries, no prior reviews → nothing in tab 2
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")}, nil, nil, nil)
	prs := m.visiblePRs()
	if len(prs) != 0 {
		t.Errorf("expected empty needs-review tab, got %d PRs", len(prs))
	}
}

// needsReReview tests

// Bot filtering tests

func TestListModel_botPRsPendingAppearInNeedsReview(t *testing.T) {
	m := newListModel()
	m.state = listStateReady
	m = m.setCurrentUser("roramirez")

	botPR := makePR(6, "app/dependabot", "roramirez/mmterm", false, github.StatusPending)
	botPR.Author.IsBot = true
	m.allPRs = []github.PR{botPR}

	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")}, nil, nil, nil)
	prs := m.visiblePRs()
	if len(prs) != 1 || prs[0].Number != 6 {
		t.Errorf("pending bot PRs should appear in Needs Review, got %d", len(prs))
	}
}

func TestListModel_botPRsApprovedNotInNeedsReview(t *testing.T) {
	m := newListModel()
	m.state = listStateReady
	m = m.setCurrentUser("roramirez")

	botPR := makePR(6, "app/dependabot", "roramirez/mmterm", false, github.StatusApproved)
	botPR.Author.IsBot = true
	m.allPRs = []github.PR{botPR}

	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")}, nil, nil, nil)
	prs := m.visiblePRs()
	if len(prs) != 0 {
		t.Errorf("approved bot PRs should NOT appear in Needs Review, got %d", len(prs))
	}
}

func TestListModel_botPRsAppearInAllOpen(t *testing.T) {
	m := newListModel()
	m.state = listStateReady
	m = m.setCurrentUser("roramirez")

	botPR := makePR(6, "app/dependabot", "roramirez/mmterm", false, github.StatusPending)
	botPR.Author.IsBot = true
	m.allPRs = []github.PR{botPR}

	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")}, nil, nil, nil)
	prs := m.visiblePRs()
	if len(prs) != 1 {
		t.Errorf("bot PRs should appear in All Open, got %d", len(prs))
	}
}

// Pure function tests

func TestRenderCheckIcon(t *testing.T) {
	cases := []struct {
		state string
		want  string
	}{
		{"SUCCESS", "✓"},
		{"FAILURE", "✗"},
		{"ERROR", "✗"},
		{"PENDING", "○"},
		{"IN_PROGRESS", "○"},
		{"QUEUED", "○"},
		{"", "—"},
		{"EXPECTED", "—"},
	}
	for _, c := range cases {
		got := renderCheckIcon(c.state)
		if !strings.Contains(got, c.want) {
			t.Errorf("renderCheckIcon(%q) = %q, want to contain %q", c.state, got, c.want)
		}
	}
}

func TestStatusDot(t *testing.T) {
	cases := []struct {
		pr      github.PR
		wantDot string
	}{
		{github.PR{IsDraft: true}, "○"},
		{github.PR{ReviewStatus: github.StatusApproved}, "●"},
		{github.PR{ReviewStatus: github.StatusChangesRequested}, "●"},
		{github.PR{ReviewStatus: github.StatusConflict}, "●"},
		{github.PR{ReviewStatus: github.StatusPending}, "●"},
	}
	for _, c := range cases {
		dot, _ := statusDot(c.pr)
		if dot != c.wantDot {
			t.Errorf("statusDot(%+v) dot = %q, want %q", c.pr, dot, c.wantDot)
		}
	}
}

func TestTimeAgo(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "just now"},
		{5 * time.Minute, "5m"},
		{3 * time.Hour, "3h"},
		{2 * 24 * time.Hour, "2d"},
		{10 * 24 * time.Hour, "1w"},
	}
	for _, c := range cases {
		got := timeAgo(time.Now().Add(-c.d))
		if got != c.want {
			t.Errorf("timeAgo(-%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestUpdateListModelMore_appends(t *testing.T) {
	m := newListModel()
	m.allPRs = []github.PR{makePR(1, "alice", "org/repo", false, github.StatusPending)}

	msg := MorePRsLoadedMsg{
		PRs:       []github.PR{makePR(2, "bob", "org/repo", false, github.StatusPending)},
		HasNext:   false,
		EndCursor: "",
		Repo:      "org/repo",
	}
	m, _ = updateListModelMore(m, msg)
	if len(m.allPRs) != 2 {
		t.Errorf("expected 2 PRs, got %d", len(m.allPRs))
	}
	if m.hasNextPage["org/repo"] {
		t.Error("expected hasNextPage=false")
	}
}

func TestUpdateListModelMore_error(t *testing.T) {
	m := newListModel()
	msg := MorePRsLoadedMsg{Err: fmt.Errorf("rate limited")}
	m, cmds := updateListModelMore(m, msg)
	if len(m.allPRs) != 0 {
		t.Error("expected no PRs on error")
	}
	if len(cmds) == 0 {
		t.Error("expected error status cmd")
	}
}

func TestUpdateListModel_error(t *testing.T) {
	m := newListModel()
	m, cmds := updateListModel(m, PRsLoadedMsg{Err: fmt.Errorf("api error")})
	if m.state != listStateReady {
		t.Error("expected ready state even on error")
	}
	if len(cmds) == 0 {
		t.Error("expected error status cmd")
	}
}

func TestSelectedPR_emptyCursor(t *testing.T) {
	m := newListModel()
	m.state = listStateReady
	_, ok := m.selectedPR()
	if ok {
		t.Error("expected false for empty list")
	}
}

func TestSelectedPR_cursorBeyondLen(t *testing.T) {
	m := newListModel()
	m.state = listStateReady
	m = m.setCurrentUser("alice")
	m.allPRs = []github.PR{makePR(1, "alice", "org/repo", false, github.StatusPending)}
	m.cursor = 99
	_, ok := m.selectedPR()
	if ok {
		t.Error("expected false when cursor >= len")
	}
}

func TestUpdateListModel_paginationClampsCursor(t *testing.T) {
	m := newListModel()
	m = m.setCurrentUser("alice")
	m.cursor = 5
	prs := []github.PR{makePR(1, "alice", "org/repo", false, github.StatusPending)}
	m, _ = updateListModel(m, PRsLoadedMsg{PRs: prs})
	if m.cursor != 0 {
		t.Errorf("cursor should clamp to 0, got %d", m.cursor)
	}
}

func TestUpdateListModel_paginationTracking(t *testing.T) {
	m := newListModel()
	m = m.setCurrentUser("alice")
	pr := makePR(1, "alice", "org/repo", false, github.StatusPending)
	pr.HasNextPage = true
	pr.EndCursor = "abc"
	m, _ = updateListModel(m, PRsLoadedMsg{PRs: []github.PR{pr}})
	if !m.hasNextPage["org/repo"] {
		t.Error("expected hasNextPage=true for org/repo")
	}
	if m.endCursor["org/repo"] != "abc" {
		t.Errorf("endCursor: got %q", m.endCursor["org/repo"])
	}
}

func TestMin(t *testing.T) {
	if min(3, 5) != 3 {
		t.Error("min(3,5) should be 3")
	}
	if min(7, 2) != 2 {
		t.Error("min(7,2) should be 2")
	}
}

func TestNeedsReReview_noReviews(t *testing.T) {
	pr := makePR(1, "bob", "org/repo", false, github.StatusPending)
	if needsReReview(pr, "alice") {
		t.Error("expected false when no reviews exist")
	}
}

func TestNeedsReReview_reviewBeforeUpdate(t *testing.T) {
	pr := makePR(1, "bob", "org/repo", false, github.StatusApproved)
	reviewTime := pr.UpdatedAt.Add(-time.Hour) // reviewed 1h before last update
	pr.Reviews = []github.Review{
		{Author: github.User{Login: "alice"}, State: "APPROVED", SubmittedAt: reviewTime},
	}
	if !needsReReview(pr, "alice") {
		t.Error("expected true: PR updated after alice's review")
	}
}

func TestNeedsReReview_reviewAfterUpdate(t *testing.T) {
	pr := makePR(1, "bob", "org/repo", false, github.StatusApproved)
	reviewTime := pr.UpdatedAt.Add(time.Hour) // reviewed 1h AFTER last update
	pr.Reviews = []github.Review{
		{Author: github.User{Login: "alice"}, State: "APPROVED", SubmittedAt: reviewTime},
	}
	if needsReReview(pr, "alice") {
		t.Error("expected false: alice's review is newer than PR update")
	}
}

func TestNeedsReReview_latestReviewWins(t *testing.T) {
	pr := makePR(1, "bob", "org/repo", false, github.StatusApproved)
	// alice reviewed twice; her latest review is after the PR update
	pr.Reviews = []github.Review{
		{Author: github.User{Login: "alice"}, State: "APPROVED", SubmittedAt: pr.UpdatedAt.Add(-2 * time.Hour)},
		{Author: github.User{Login: "alice"}, State: "APPROVED", SubmittedAt: pr.UpdatedAt.Add(time.Hour)},
	}
	if needsReReview(pr, "alice") {
		t.Error("expected false: alice's latest review is after PR update")
	}
}

func TestListModel_reReviewAppearsInTab3(t *testing.T) {
	m := newListModel()
	m.state = listStateReady
	m = m.setCurrentUser("alice")

	pr := makePR(5, "bob", "org/repo", false, github.StatusApproved)
	// alice reviewed before the PR was updated
	pr.Reviews = []github.Review{
		{Author: github.User{Login: "alice"}, State: "APPROVED", SubmittedAt: pr.UpdatedAt.Add(-time.Hour)},
	}
	m.allPRs = []github.PR{pr}
	// search set is empty (review was already submitted so GitHub removed alice from reviewers)
	m = m.setReviewRequestedSet(map[string]bool{})

	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")}, nil, nil, nil)
	prs := m.visiblePRs()
	if len(prs) != 1 || prs[0].Number != 5 {
		t.Errorf("expected PR #5 in needs-review via re-review detection, got %v", prs)
	}
}

func TestPrTitleAndStyle_conflict(t *testing.T) {
	pr := github.PR{Title: "fix auth", Mergeable: "CONFLICTING"}
	title, _ := prTitleAndStyle(pr)
	if !strings.Contains(title, "[conflict]") {
		t.Errorf("expected [conflict] prefix, got %q", title)
	}
	if !strings.Contains(title, "fix auth") {
		t.Errorf("expected original title in output, got %q", title)
	}
}

func TestPrTitleAndStyle_draft(t *testing.T) {
	pr := github.PR{Title: "wip", IsDraft: true}
	title, _ := prTitleAndStyle(pr)
	if !strings.Contains(title, "[draft]") {
		t.Errorf("expected [draft] prefix, got %q", title)
	}
}

func TestPrTitleAndStyle_normal(t *testing.T) {
	pr := github.PR{Title: "add tests", Mergeable: "MERGEABLE"}
	title, _ := prTitleAndStyle(pr)
	if strings.Contains(title, "[") {
		t.Errorf("expected no prefix for normal PR, got %q", title)
	}
}

func TestListModel_draftCannotApprove(t *testing.T) {
	m := newListModel()
	m.state = listStateReady
	m = m.setCurrentUser("alice")
	m.allPRs = []github.PR{makePR(1, "alice", "org/repo", true, github.StatusPending)}

	_, cmd := m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}, nil, nil, nil)
	if cmd == nil {
		t.Fatal("expected a command (status error) for draft approve")
	}
	msg := cmd()
	statusMsg, ok := msg.(StatusMsg)
	if !ok {
		t.Fatalf("expected StatusMsg, got %T", msg)
	}
	if !statusMsg.IsError {
		t.Error("expected error status for draft approve")
	}
}
