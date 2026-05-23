// Package demo provides mock data and a fake HTTP transport for the --demo flag.
// No real GitHub token or network access is required.
package demo

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// MockDiff is a realistic Go unified diff used in the demo detail view.
//
//go:embed testdata/mock.diff
var MockDiff string

// MockUser is the fake authenticated user login.
const MockUser = "roramirez"

const mockPRBody = "## Summary\n\n" +
	"Add token expiry validation and per-scope authorization to the auth package.\n\n" +
	"## Changes\n\n" +
	"- **New field** `Scopes []string` on `Token` struct\n" +
	"- **New error** `ErrTokenExpired` returned when token is past `ExpiresAt`\n" +
	"- `Validate()` now accepts optional required scopes and checks each one\n" +
	"- Helper `hasScope()` for internal scope lookup\n\n" +
	"## How to test\n\n" +
	"```bash\ngo test ./internal/auth/... -race\n```\n\n" +
	"All existing tests pass. New tests cover expiry and scope validation."

const mockCommentsJSON = `[
  {"body": "This looks good overall. Should ErrTokenExpired be exported or internal?", "created_at": "2026-05-18T10:00:00Z", "user": {"login": "alice"}},
  {"body": "LGTM! Left one inline comment on the scope check.", "created_at": "2026-05-18T11:30:00Z", "user": {"login": "bob"}}
]`

const mockReviewCommentsJSON = `[
  {"body": "Consider using errors.Is here instead of direct comparison for better error wrapping support.", "path": "internal/auth/token.go", "line": 42, "position": 5, "user": {"login": "bob"}}
]`

// now is a stable reference point for demo timestamps.
var now = time.Now()

// prNode returns a fake GraphQL PR node JSON.
func prNode(number int, title, head, base string, additions, deletions int, //nolint:unparam
	authorLogin, typename, mergeable string, isDraft bool,
	reviewStates []string, reviewRequestedLogins []string,
	updatedAgo time.Duration,
	checkState string,
) map[string]interface{} {
	reviews := make([]map[string]interface{}, 0, len(reviewStates))
	for i, state := range reviewStates {
		reviewer := "alice"
		if i%2 == 1 {
			reviewer = "bob"
		}
		reviews = append(reviews, map[string]interface{}{
			"author":      map[string]interface{}{"login": reviewer},
			"state":       state,
			"submittedAt": now.Add(-updatedAgo - time.Hour).Format(time.RFC3339),
		})
	}
	rrs := make([]map[string]interface{}, 0, len(reviewRequestedLogins))
	for _, login := range reviewRequestedLogins {
		rrs = append(rrs, map[string]interface{}{
			"requestedReviewer": map[string]interface{}{"login": login},
		})
	}

	var commitsNode interface{}
	if checkState != "" {
		commitsNode = map[string]interface{}{
			"nodes": []map[string]interface{}{
				{"commit": map[string]interface{}{
					"statusCheckRollup": map[string]interface{}{"state": checkState},
				}},
			},
		}
	} else {
		commitsNode = map[string]interface{}{"nodes": []interface{}{}}
	}

	return map[string]interface{}{
		"number":      number,
		"title":       title,
		"url":         fmt.Sprintf("https://github.com/acme/backend/pull/%d", number),
		"isDraft":     isDraft,
		"createdAt":   now.Add(-48 * time.Hour).Format(time.RFC3339),
		"updatedAt":   now.Add(-updatedAgo).Format(time.RFC3339),
		"additions":   additions,
		"deletions":   deletions,
		"headRefName": head,
		"baseRefName": base,
		"body":        mockPRBody,
		"mergeable":   mergeable,
		"author":      map[string]interface{}{"login": authorLogin, "__typename": typename},
		"reviews":     map[string]interface{}{"nodes": reviews},
		"reviewRequests": map[string]interface{}{
			"nodes": rrs,
		},
		"commits": commitsNode,
	}
}

// GraphQLResponse builds a fake GraphQL response for the given repos.
func GraphQLResponse() map[string]interface{} {
	nodes := []map[string]interface{}{
		// [1] My PRs
		prNode(42, "feat: add token expiry and scope validation", "feat/token-expiry", "main",
			45, 12, MockUser, "User", "MERGEABLE", false,
			[]string{"APPROVED"}, nil, 2*time.Hour, "SUCCESS"),
		prNode(38, "fix: handle nil pointer in cache eviction", "fix/cache-nil", "main",
			8, 3, MockUser, "User", "MERGEABLE", false,
			[]string{"CHANGES_REQUESTED"}, nil, 5*time.Hour, "FAILURE"),
		prNode(34, "fix: resolve session token storage conflict", "fix/session-conflict", "main",
			22, 7, MockUser, "User", "CONFLICTING", false,
			nil, nil, 3*time.Hour, "SUCCESS"),
		prNode(35, "wip: new dashboard layout", "wip/dashboard", "main",
			120, 0, MockUser, "User", "MERGEABLE", true,
			nil, nil, 1*time.Hour, "PENDING"),

		// [2] Needs Review (human — reviewer requested)
		prNode(41, "refactor: split middleware into separate packages", "refactor/middleware", "main",
			230, 180, "alice", "User", "MERGEABLE", false,
			nil, []string{MockUser}, 30*time.Minute, "SUCCESS"),

		// [2] Needs Review (bot — Dependabot, pending)
		prNode(40, "chore(deps): bump golang.org/x/net from 0.17 to 0.23", "deps/bump-net", "main",
			3, 3, "dependabot[bot]", "Bot", "MERGEABLE", false,
			nil, nil, 3*time.Hour, "SUCCESS"),
		prNode(39, "chore(deps): bump github.com/BurntSushi/toml from 1.3.2 to 1.4.0", "deps/bump-toml", "main",
			2, 2, "app/dependabot", "Bot", "MERGEABLE", false,
			nil, nil, 6*time.Hour, "PENDING"),

		// [3] All Open (others)
		prNode(37, "docs: update API reference with new endpoints", "docs/api-ref", "main",
			95, 12, "carol", "User", "MERGEABLE", false,
			nil, nil, 4*time.Hour, ""),
		prNode(36, "test: increase coverage for auth package", "test/auth-coverage", "main",
			180, 10, "bob", "User", "CONFLICTING", false,
			nil, nil, 8*time.Hour, "FAILURE"),
	}

	repoData := map[string]interface{}{
		"pullRequests": map[string]interface{}{
			"nodes": nodes,
			"pageInfo": map[string]interface{}{
				"hasNextPage": false,
				"endCursor":   "",
			},
		},
	}
	return map[string]interface{}{
		"r0": repoData,
	}
}

// SearchResponse returns a fake /search/issues response for review-requested:@me.
func SearchResponse() map[string]interface{} {
	return map[string]interface{}{
		"total_count": 1,
		"items": []map[string]interface{}{
			{
				"number":         41,
				"repository_url": "https://api.github.com/repos/acme/backend",
			},
		},
	}
}

// Transport is a fake http.RoundTripper that serves all demo responses.
type Transport struct{}

func (Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	path := req.URL.Path
	switch {
	case path == "/user":
		return jsonResponse(map[string]string{"login": MockUser}), nil
	case path == "/graphql":
		return jsonResponse(map[string]interface{}{"data": GraphQLResponse(), "errors": nil}), nil
	case isGetIssueComments(path, req.Method):
		return rawResponse(mockCommentsJSON), nil
	case isGetPullComments(path, req.Method):
		return rawResponse(mockReviewCommentsJSON), nil
	case isPullMerge(path, req.Method):
		return jsonResponse(map[string]string{"sha": "abc123", "merged": "true", "message": "PR merged"}), nil
	case isPullDiff(path, req):
		return rawResponse(MockDiff), nil
	case path == "/search/issues":
		return jsonResponse(SearchResponse()), nil
	default:
		return jsonResponse(map[string]interface{}{}), nil
	}
}

func isGetIssueComments(path, method string) bool {
	return strings.Contains(path, "/issues/") && strings.HasSuffix(path, "/comments") && method == http.MethodGet
}

func isGetPullComments(path, method string) bool {
	return strings.Contains(path, "/pulls/") && strings.HasSuffix(path, "/comments") && method == http.MethodGet
}

func isPullMerge(path, method string) bool {
	return strings.Contains(path, "/pulls/") && strings.HasSuffix(path, "/merge") && method == http.MethodPut
}

func isPullDiff(path string, req *http.Request) bool {
	return strings.Contains(path, "/pulls/") && req.Header.Get("Accept") == "application/vnd.github.v3.diff"
}

func jsonResponse(body interface{}) *http.Response {
	data, _ := json.Marshal(body)
	h := make(http.Header)
	h.Set("X-RateLimit-Remaining", "4999")
	h.Set("X-RateLimit-Reset", "9999999999")
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(string(data))),
		Header:     h,
	}
}

func rawResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}
