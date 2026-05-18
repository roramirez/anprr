// Package demo provides mock data and a fake HTTP transport for the --demo flag.
// No real GitHub token or network access is required.
package demo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// MockDiff is a realistic Go unified diff used in the demo detail view.
const MockDiff = `diff --git a/internal/auth/token.go b/internal/auth/token.go
--- a/internal/auth/token.go
+++ b/internal/auth/token.go
@@ -18,12 +18,24 @@ import (
 	"time"
 )

+var ErrTokenExpired = errors.New("token has expired")
+
 type Token struct {
 	Value     string
 	ExpiresAt time.Time
+	Scopes    []string
 }

-func Validate(t *Token) error {
-	if t == nil {
-		return errors.New("token is nil")
-	}
-	return nil
+func Validate(t *Token, required ...string) error {
+	if t == nil {
+		return errors.New("token is nil")
+	}
+	if time.Now().After(t.ExpiresAt) {
+		return ErrTokenExpired
+	}
+	for _, scope := range required {
+		if !hasScope(t.Scopes, scope) {
+			return fmt.Errorf("missing required scope: %s", scope)
+		}
+	}
+	return nil
+}
+
+func hasScope(scopes []string, s string) bool {
+	for _, v := range scopes {
+		if v == s {
+			return true
+		}
+	}
+	return false
 }
diff --git a/internal/auth/token_test.go b/internal/auth/token_test.go
--- a/internal/auth/token_test.go
+++ b/internal/auth/token_test.go
@@ -1,18 +1,42 @@
 package auth

 import (
+	"errors"
 	"testing"
+	"time"
 )

-func TestValidate_nil(t *testing.T) {
-	if err := Validate(nil); err == nil {
-		t.Error("expected error for nil token")
+func TestValidate(t *testing.T) {
+	valid := &Token{
+		Value:     "tok",
+		ExpiresAt: time.Now().Add(time.Hour),
+		Scopes:    []string{"repo", "read:user"},
 	}
-}

-func TestValidate_ok(t *testing.T) {
-	tok := &Token{Value: "abc"}
-	if err := Validate(tok); err != nil {
-		t.Errorf("unexpected error: %v", err)
+	if err := Validate(valid, "repo"); err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+
+	expired := &Token{
+		Value:     "old",
+		ExpiresAt: time.Now().Add(-time.Hour),
+	}
+	if !errors.Is(Validate(expired), ErrTokenExpired) {
+		t.Error("expected ErrTokenExpired")
+	}
+
+	if err := Validate(valid, "admin"); err == nil {
+		t.Error("expected error for missing scope")
 	}
 }
`

// MockUser is the fake authenticated user login.
const MockUser = "roramirez"

// now is a stable reference point for demo timestamps.
var now = time.Now()

// prNode returns a fake GraphQL PR node JSON.
func prNode(number int, title, head, base string, additions, deletions int,
	authorLogin, typename, mergeable string, isDraft bool,
	reviewStates []string, reviewRequestedLogins []string,
	updatedAgo time.Duration,
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
		"mergeable":   mergeable,
		"author":      map[string]interface{}{"login": authorLogin, "__typename": typename},
		"reviews":     map[string]interface{}{"nodes": reviews},
		"reviewRequests": map[string]interface{}{
			"nodes": rrs,
		},
	}
}

// GraphQLResponse builds a fake GraphQL response for the given repos.
func GraphQLResponse() map[string]interface{} {
	nodes := []map[string]interface{}{
		// [1] My PRs
		prNode(42, "feat: add token expiry and scope validation", "feat/token-expiry", "main",
			45, 12, MockUser, "User", "MERGEABLE", false,
			[]string{"APPROVED"}, nil, 2*time.Hour),
		prNode(38, "fix: handle nil pointer in cache eviction", "fix/cache-nil", "main",
			8, 3, MockUser, "User", "MERGEABLE", false,
			[]string{"CHANGES_REQUESTED"}, nil, 5*time.Hour),
		prNode(35, "wip: new dashboard layout", "wip/dashboard", "main",
			120, 0, MockUser, "User", "MERGEABLE", true,
			nil, nil, 1*time.Hour),

		// [2] Needs Review (human — reviewer requested)
		prNode(41, "refactor: split middleware into separate packages", "refactor/middleware", "main",
			230, 180, "alice", "User", "MERGEABLE", false,
			nil, []string{MockUser}, 30*time.Minute),

		// [2] Needs Review (bot — Dependabot, pending)
		prNode(40, "chore(deps): bump golang.org/x/net from 0.17 to 0.23", "deps/bump-net", "main",
			3, 3, "dependabot[bot]", "Bot", "MERGEABLE", false,
			nil, nil, 3*time.Hour),
		prNode(39, "chore(deps): bump github.com/BurntSushi/toml from 1.3.2 to 1.4.0", "deps/bump-toml", "main",
			2, 2, "app/dependabot", "Bot", "MERGEABLE", false,
			nil, nil, 6*time.Hour),

		// [3] All Open (others)
		prNode(37, "docs: update API reference with new endpoints", "docs/api-ref", "main",
			95, 12, "carol", "User", "MERGEABLE", false,
			nil, nil, 4*time.Hour),
		prNode(36, "test: increase coverage for auth package", "test/auth-coverage", "main",
			180, 10, "bob", "User", "CONFLICTING", false,
			nil, nil, 8*time.Hour),
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
	var body interface{}

	switch {
	case path == "/user":
		body = map[string]string{"login": MockUser}

	case path == "/graphql":
		wrapper := map[string]interface{}{
			"data":   GraphQLResponse(),
			"errors": nil,
		}
		body = wrapper

	case strings.Contains(path, "/pulls/") && strings.HasSuffix(path, "/merge") && req.Method == http.MethodPut:
		body = map[string]string{"sha": "abc123", "merged": "true", "message": "PR merged"}

	case strings.Contains(path, "/pulls/") && req.Header.Get("Accept") == "application/vnd.github.v3.diff":
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(MockDiff)),
			Header:     make(http.Header),
		}, nil

	case path == "/search/issues":
		body = SearchResponse()

	default:
		body = map[string]interface{}{}
	}

	data, _ := json.Marshal(body)
	h := make(http.Header)
	h.Set("X-RateLimit-Remaining", "4999")
	h.Set("X-RateLimit-Reset", "9999999999")
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(string(data))),
		Header:     h,
	}, nil
}
