package github

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestBuildListPRsQuery_singleRepo(t *testing.T) {
	query, aliasMap := buildListPRsQuery([]string{"owner/repo"}, "")
	if len(aliasMap) != 1 {
		t.Errorf("expected 1 alias, got %d", len(aliasMap))
	}
	if aliasMap["r0"] != "owner/repo" {
		t.Errorf("aliasMap: %v", aliasMap)
	}
	if !strings.Contains(query, `r0: repository(owner: "owner", name: "repo")`) {
		t.Errorf("query missing alias:\n%s", query)
	}
}

func TestBuildListPRsQuery_multipleRepos(t *testing.T) {
	repos := []string{"myorg/backend", "myorg/frontend"}
	query, aliasMap := buildListPRsQuery(repos, "")
	if len(aliasMap) != 2 {
		t.Fatalf("expected 2 aliases, got %d", len(aliasMap))
	}
	if !strings.Contains(query, "r0:") || !strings.Contains(query, "r1:") {
		t.Errorf("query missing numeric aliases:\n%s", query)
	}
	// verify repos are correctly mapped
	repos0 := aliasMap["r0"]
	repos1 := aliasMap["r1"]
	if repos0 != "myorg/backend" || repos1 != "myorg/frontend" {
		t.Errorf("aliasMap: %v", aliasMap)
	}
}

func TestBuildListPRsQuery_repoWithHyphens(t *testing.T) {
	// repos with hyphens and dots must not produce invalid GraphQL identifiers
	repos := []string{"my-org/my-service-gcp", "another.org/repo.name"}
	query, aliasMap := buildListPRsQuery(repos, "")
	if len(aliasMap) != 2 {
		t.Fatalf("expected 2 aliases, got %d", len(aliasMap))
	}
	// aliases must be simple identifiers (r0, r1) — no hyphens or dots
	for alias := range aliasMap {
		for _, c := range alias {
			if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_') {
				t.Errorf("alias %q contains invalid GraphQL identifier char %q", alias, c)
			}
		}
	}
	// original repo names must appear correctly quoted in the query
	if !strings.Contains(query, `"my-org"`) || !strings.Contains(query, `"my-service-gcp"`) {
		t.Errorf("query missing hyphenated repo names:\n%s", query)
	}
}

func TestBuildListPRsQuery_withCursor(t *testing.T) {
	query, _ := buildListPRsQuery([]string{"a/b"}, "cursor123")
	if !strings.Contains(query, `after: "cursor123"`) {
		t.Errorf("query missing cursor:\n%s", query)
	}
}

func TestParseListPRsResponse(t *testing.T) {
	aliasMap := map[string]string{"r0": "owner/repo"}
	prJSON := `{
		"number": 7, "title": "fix: something", "url": "https://github.com/owner/repo/pull/7",
		"isDraft": false, "body": "## Summary\nFixes a bug.",
		"createdAt": "2026-05-01T00:00:00Z", "updatedAt": "2026-05-18T00:00:00Z",
		"additions": 10, "deletions": 2,
		"headRefName": "fix/bug", "baseRefName": "main", "mergeable": "MERGEABLE",
		"author": {"login": "alice", "__typename": "User"},
		"reviewRequests": {"nodes": []},
		"reviews": {"nodes": []},
		"commits": {"nodes": [{"commit": {"statusCheckRollup": {"state": "SUCCESS"}}}]}
	}`
	repoJSON := `{"pullRequests":{"nodes":[` + prJSON + `],"pageInfo":{"hasNextPage":false,"endCursor":""}}}`
	data := map[string]json.RawMessage{"r0": json.RawMessage(repoJSON)}

	prs, err := parseListPRsResponse(data, aliasMap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(prs))
	}
	pr := prs[0]
	if pr.Number != 7 || pr.Title != "fix: something" {
		t.Errorf("wrong PR fields: %+v", pr)
	}
	if pr.Repo != "owner/repo" {
		t.Errorf("repo: got %q", pr.Repo)
	}
	if pr.Body != "## Summary\nFixes a bug." {
		t.Errorf("body: got %q", pr.Body)
	}
	if pr.CheckState != "SUCCESS" {
		t.Errorf("checkState: got %q", pr.CheckState)
	}
	if pr.Author.Login != "alice" || pr.Author.IsBot {
		t.Errorf("author: %+v", pr.Author)
	}
}

func TestParseListPRsResponse_missingAlias(t *testing.T) {
	aliasMap := map[string]string{"r0": "owner/repo"}
	data := map[string]json.RawMessage{} // no "r0" key
	prs, err := parseListPRsResponse(data, aliasMap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 0 {
		t.Errorf("expected 0 PRs, got %d", len(prs))
	}
}

func TestIsBotAuthor(t *testing.T) {
	cases := []struct {
		login    string
		typename string
		want     bool
	}{
		{"roramirez", "User", false},
		{"dependabot[bot]", "Bot", true},
		{"app/dependabot", "User", true}, // login prefix heuristic
		{"renovate[bot]", "Bot", true},
		{"some-user", "User", false},
		{"mannequin-user", "Mannequin", true},
		{"copilot[bot]", "Bot", true},
	}
	for _, c := range cases {
		got := isBotAuthor(c.login, c.typename)
		if got != c.want {
			t.Errorf("isBotAuthor(%q, %q) = %v, want %v", c.login, c.typename, got, c.want)
		}
	}
}

func TestDerivePRStatus_conflict(t *testing.T) {
	pr := PR{Mergeable: "CONFLICTING"}
	if got := DerivePRStatus(pr); got != StatusConflict {
		t.Errorf("got %s", got)
	}
}

func TestDerivePRStatus_approved(t *testing.T) {
	pr := PR{
		Mergeable: "MERGEABLE",
		Reviews: []Review{
			{Author: User{Login: "alice"}, State: "APPROVED", SubmittedAt: time.Now()},
		},
	}
	if got := DerivePRStatus(pr); got != StatusApproved {
		t.Errorf("got %s", got)
	}
}

func TestDerivePRStatus_changesRequested(t *testing.T) {
	pr := PR{
		Mergeable: "MERGEABLE",
		Reviews: []Review{
			{Author: User{Login: "alice"}, State: "CHANGES_REQUESTED", SubmittedAt: time.Now()},
		},
	}
	if got := DerivePRStatus(pr); got != StatusChangesRequested {
		t.Errorf("got %s", got)
	}
}

func TestDerivePRStatus_latestWins(t *testing.T) {
	// alice first requested changes, then approved → approved wins
	t1 := time.Now().Add(-time.Hour)
	t2 := time.Now()
	pr := PR{
		Mergeable: "MERGEABLE",
		Reviews: []Review{
			{Author: User{Login: "alice"}, State: "CHANGES_REQUESTED", SubmittedAt: t1},
			{Author: User{Login: "alice"}, State: "APPROVED", SubmittedAt: t2},
		},
	}
	if got := DerivePRStatus(pr); got != StatusApproved {
		t.Errorf("got %s, want approved", got)
	}
}

func TestConvertPR_checkState(t *testing.T) {
	withCheck := gqlPR{
		Number: 1, Title: "t", Mergeable: "MERGEABLE",
		Author: struct {
			Login    string `json:"login"`
			Typename string `json:"__typename"`
		}{Login: "alice", Typename: "User"},
	}
	withCheck.Commits.Nodes = []struct {
		Commit struct {
			StatusCheckRollup *struct {
				State string `json:"state"`
			} `json:"statusCheckRollup"`
		} `json:"commit"`
	}{
		{Commit: struct {
			StatusCheckRollup *struct {
				State string `json:"state"`
			} `json:"statusCheckRollup"`
		}{StatusCheckRollup: &struct {
			State string `json:"state"`
		}{State: "SUCCESS"}}},
	}
	pr := convertPR(withCheck, "org/repo")
	if pr.CheckState != "SUCCESS" {
		t.Errorf("expected SUCCESS, got %q", pr.CheckState)
	}

	// no commits → empty CheckState
	empty := gqlPR{
		Number: 2, Title: "t", Mergeable: "MERGEABLE",
		Author: struct {
			Login    string `json:"login"`
			Typename string `json:"__typename"`
		}{Login: "bob", Typename: "User"},
	}
	pr2 := convertPR(empty, "org/repo")
	if pr2.CheckState != "" {
		t.Errorf("expected empty CheckState, got %q", pr2.CheckState)
	}
}

func TestDerivePRStatus_pending(t *testing.T) {
	pr := PR{Mergeable: "MERGEABLE"}
	if got := DerivePRStatus(pr); got != StatusPending {
		t.Errorf("got %s", got)
	}
}
