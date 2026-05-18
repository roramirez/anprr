package github

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	diffTTL            = 5 * time.Minute
	reviewRequestedTTL = 60 * time.Second
	reviewRequestedKey = "review-requested"
)

// GetCurrentUser returns the login of the authenticated user.
func (c *Client) GetCurrentUser() (string, error) {
	var user struct {
		Login string `json:"login"`
	}
	if err := c.REST("GET", "/user", nil, &user); err != nil {
		return "", err
	}
	return user.Login, nil
}

// GetDiff returns the raw unified diff for a PR.
func (c *Client) GetDiff(owner, repo string, number int, cache *Cache) (string, error) {
	key := fmt.Sprintf("diff:%s/%s#%d", owner, repo, number)
	if v, ok := cache.Get(key); ok {
		return v.(string), nil
	}

	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d", baseURL, owner, repo, number)
	data, err := c.do(http.MethodGet, url, nil, "application/vnd.github.v3.diff")
	if err != nil {
		return "", err
	}

	diff := string(data)
	cache.Set(key, diff, diffTTL)
	return diff, nil
}

type reviewComment struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Side string `json:"side"`
	Body string `json:"body"`
}

// SubmitReview submits a review on a PR, optionally with inline comments.
func (c *Client) SubmitReview(owner, repo string, number int, event ReviewEvent, body string, inline []InlineComment) error {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews", owner, repo, number)

	comments := make([]reviewComment, 0, len(inline))
	for _, ic := range inline {
		comments = append(comments, reviewComment(ic))
	}

	payload := map[string]interface{}{
		"event":    string(event),
		"body":     body,
		"comments": comments,
	}
	return c.REST("POST", path, payload, nil)
}

// MergeMethod controls how a PR is merged.
type MergeMethod string

const (
	MergeMethodMerge  MergeMethod = "merge"
	MergeMethodSquash MergeMethod = "squash"
	MergeMethodRebase MergeMethod = "rebase"
)

// MergePR merges a pull request using the given method.
func (c *Client) MergePR(owner, repo string, number int, method MergeMethod) error {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d/merge", owner, repo, number)
	payload := map[string]string{"merge_method": string(method)}
	return c.REST("PUT", path, payload, nil)
}

// SearchReviewRequested returns the set of open PRs across repos where the
// authenticated user has been requested as a reviewer.
// Keys in the returned map are "owner/repo#number".
func (c *Client) SearchReviewRequested(repos []string, cache *Cache) (map[string]bool, error) {
	if v, ok := cache.Get(reviewRequestedKey); ok {
		return v.(map[string]bool), nil
	}

	// Build query: is:pr is:open review-requested:@me repo:a/b repo:c/d ...
	var parts []string
	parts = append(parts, "is:pr", "is:open", "review-requested:@me")
	for _, r := range repos {
		parts = append(parts, "repo:"+r)
	}
	q := strings.Join(parts, " ")

	result := make(map[string]bool)
	page := 1
	for {
		endpoint := fmt.Sprintf("/search/issues?q=%s&per_page=100&page=%d",
			url.QueryEscape(q), page)

		var resp struct {
			Items []struct {
				Number        int    `json:"number"`
				RepositoryURL string `json:"repository_url"` // "https://api.github.com/repos/owner/name"
			} `json:"items"`
			TotalCount int `json:"total_count"`
		}
		if err := c.REST("GET", endpoint, nil, &resp); err != nil {
			return nil, err
		}

		for _, item := range resp.Items {
			// repository_url ends with "repos/owner/name"
			repoPath := strings.TrimPrefix(item.RepositoryURL, baseURL+"/repos/")
			key := fmt.Sprintf("%s#%d", repoPath, item.Number)
			result[key] = true
		}

		if len(resp.Items) < 100 {
			break // last page
		}
		page++
	}

	cache.Set(reviewRequestedKey, result, reviewRequestedTTL)
	return result, nil
}

// FetchComments fetches general comments and inline review comments for a PR.
func (c *Client) FetchComments(owner, repo string, number int) ([]Comment, []LineComment, error) {
	// general issue comments
	var rawComments []struct {
		Body      string `json:"body"`
		CreatedAt string `json:"created_at"`
		User      struct {
			Login string `json:"login"`
		} `json:"user"`
	}
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/comments?per_page=100", owner, repo, number)
	if err := c.REST("GET", path, nil, &rawComments); err != nil {
		return nil, nil, err
	}
	comments := make([]Comment, 0, len(rawComments))
	for _, rc := range rawComments {
		t, _ := time.Parse(time.RFC3339, rc.CreatedAt)
		comments = append(comments, Comment{
			Author:    User{Login: rc.User.Login},
			Body:      rc.Body,
			CreatedAt: t,
		})
	}

	// inline review comments
	var rawLineComments []struct {
		Body     string `json:"body"`
		Path     string `json:"path"`
		Line     int    `json:"line"`
		Position int    `json:"position"`
		User     struct {
			Login string `json:"login"`
		} `json:"user"`
	}
	path2 := fmt.Sprintf("/repos/%s/%s/pulls/%d/comments?per_page=100", owner, repo, number)
	if err := c.REST("GET", path2, nil, &rawLineComments); err != nil {
		return comments, nil, err
	}
	reviewComments := make([]LineComment, 0, len(rawLineComments))
	for _, rc := range rawLineComments {
		line := rc.Line
		if line == 0 {
			line = rc.Position
		}
		reviewComments = append(reviewComments, LineComment{
			Author: User{Login: rc.User.Login},
			Body:   rc.Body,
			Path:   rc.Path,
			Line:   line,
		})
	}
	return comments, reviewComments, nil
}

// PostComment posts a comment on a PR.
func (c *Client) PostComment(owner, repo string, number int, body string) error {
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, repo, number)
	payload := map[string]string{"body": body}
	return c.REST("POST", path, payload, nil)
}
