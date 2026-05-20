package github

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const prListTTL = 60 * time.Second

// ListPRs fetches open PRs for all repos in a single GraphQL request.
// Returns PRs from "My PRs" perspective (all open PRs in the repos).
// The caller filters for "needs review" using the current user login.
func (c *Client) ListPRs(repos []string, cache *Cache) ([]PR, error) {
	const cacheKey = "prs"
	if v, ok := cache.Get(cacheKey); ok {
		return v.([]PR), nil
	}

	query, aliasMap := buildListPRsQuery(repos, "")
	var data map[string]json.RawMessage
	if err := c.GraphQL(query, nil, &data); err != nil {
		return nil, err
	}

	prs, err := parseListPRsResponse(data, aliasMap)
	if err != nil {
		return nil, err
	}

	cache.Set(cacheKey, prs, prListTTL)
	return prs, nil
}

// LoadMorePRs fetches the next page for a specific repo using its endCursor.
func (c *Client) LoadMorePRs(repo, cursor string, cache *Cache) ([]PR, bool, string, error) {
	query, aliasMap := buildListPRsQuery([]string{repo}, cursor)
	var data map[string]json.RawMessage
	if err := c.GraphQL(query, nil, &data); err != nil {
		return nil, false, "", err
	}
	prs, err := parseListPRsResponse(data, aliasMap)
	if err != nil {
		return nil, false, "", err
	}
	hasNext := false
	endCursor := ""
	if len(prs) > 0 {
		hasNext = prs[len(prs)-1].HasNextPage
		endCursor = prs[len(prs)-1].EndCursor
	}
	return prs, hasNext, endCursor, nil
}

// buildListPRsQuery builds a GraphQL query that fetches PRs for all repos in a
// single request using numeric aliases (r0, r1, …) to avoid GraphQL identifier
// restrictions (hyphens, dots, etc. are not allowed in field aliases).
// Returns the query and a map of alias → original "owner/repo" string.
func buildListPRsQuery(repos []string, afterCursor string) (string, map[string]string) {
	aliasMap := make(map[string]string, len(repos)) // alias → repo
	var sb strings.Builder

	sb.WriteString("{\n")
	for i, repo := range repos {
		parts := strings.SplitN(repo, "/", 2)
		alias := fmt.Sprintf("r%d", i) // safe GraphQL identifier regardless of repo name
		aliasMap[alias] = repo

		after := ""
		if afterCursor != "" {
			after = fmt.Sprintf(`, after: %q`, afterCursor)
		}

		fmt.Fprintf(&sb, `  %s: repository(owner: %q, name: %q) {
    pullRequests(first: 50, states: [OPEN], orderBy: {field: UPDATED_AT, direction: DESC}%s) {
      nodes {
        number title url isDraft body createdAt updatedAt additions deletions
        headRefName baseRefName mergeable
        author { login __typename }
        reviewRequests(first: 10) { nodes { requestedReviewer { ... on User { login } } } }
        reviews(last: 10) { nodes { author { login } state submittedAt } }
        commits(last: 1) { nodes { commit { statusCheckRollup { state } } } }
      }
      pageInfo { hasNextPage endCursor }
    }
  }
`, alias, parts[0], parts[1], after)
	}
	sb.WriteString("}")
	return sb.String(), aliasMap
}

type gqlPR struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	IsDraft   bool   `json:"isDraft"`
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	HeadRef   string `json:"headRefName"`
	BaseRef   string `json:"baseRefName"`
	Mergeable string `json:"mergeable"`
	Author    struct {
		Login    string `json:"login"`
		Typename string `json:"__typename"` // "User", "Bot", "Mannequin"
	} `json:"author"`
	ReviewRequests struct {
		Nodes []struct {
			RequestedReviewer struct {
				Login string `json:"login"`
			} `json:"requestedReviewer"`
		} `json:"nodes"`
	} `json:"reviewRequests"`
	Reviews struct {
		Nodes []struct {
			Author struct {
				Login string `json:"login"`
			} `json:"author"`
			State       string `json:"state"`
			SubmittedAt string `json:"submittedAt"`
		} `json:"nodes"`
	} `json:"reviews"`
	Commits struct {
		Nodes []struct {
			Commit struct {
				StatusCheckRollup *struct {
					State string `json:"state"`
				} `json:"statusCheckRollup"`
			} `json:"commit"`
		} `json:"nodes"`
	} `json:"commits"`
}

type gqlPageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

type gqlRepo struct {
	PullRequests struct {
		Nodes    []gqlPR     `json:"nodes"`
		PageInfo gqlPageInfo `json:"pageInfo"`
	} `json:"pullRequests"`
}

func parseListPRsResponse(data map[string]json.RawMessage, aliasMap map[string]string) ([]PR, error) {
	var result []PR
	for alias, repoName := range aliasMap {
		raw, ok := data[alias]
		if !ok {
			continue
		}
		prs, err := parsePRsForRepo(raw, repoName)
		if err != nil {
			return nil, err
		}
		result = append(result, prs...)
	}
	return result, nil
}

func parsePRsForRepo(raw json.RawMessage, repoName string) ([]PR, error) {
	var repo gqlRepo
	if err := json.Unmarshal(raw, &repo); err != nil {
		return nil, fmt.Errorf("parsing repo %s: %w", repoName, err)
	}
	nodes := repo.PullRequests.Nodes
	prs := make([]PR, len(nodes))
	for i, node := range nodes {
		prs[i] = convertPR(node, repoName)
	}
	if len(prs) > 0 {
		prs[len(prs)-1].HasNextPage = repo.PullRequests.PageInfo.HasNextPage
		prs[len(prs)-1].EndCursor = repo.PullRequests.PageInfo.EndCursor
	}
	return prs, nil
}

func convertPR(node gqlPR, repo string) PR {
	var reviews []Review
	for _, r := range node.Reviews.Nodes {
		t, _ := time.Parse(time.RFC3339, r.SubmittedAt)
		reviews = append(reviews, Review{
			Author:      User{Login: r.Author.Login},
			State:       r.State,
			SubmittedAt: t,
		})
	}

	var requestedReviewers []User
	for _, rr := range node.ReviewRequests.Nodes {
		if login := rr.RequestedReviewer.Login; login != "" {
			requestedReviewers = append(requestedReviewers, User{Login: login})
		}
	}

	createdAt, _ := time.Parse(time.RFC3339, node.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339, node.UpdatedAt)

	checkState := extractCheckState(node)

	pr := PR{
		Number:             node.Number,
		Title:              node.Title,
		Body:               node.Body,
		URL:                node.URL,
		IsDraft:            node.IsDraft,
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
		Additions:          node.Additions,
		Deletions:          node.Deletions,
		HeadRef:            node.HeadRef,
		BaseRef:            node.BaseRef,
		Mergeable:          node.Mergeable,
		Author:             User{Login: node.Author.Login, IsBot: isBotAuthor(node.Author.Login, node.Author.Typename)},
		Repo:               repo,
		Reviews:            reviews,
		RequestedReviewers: requestedReviewers,
		CheckState:         checkState,
	}
	pr.ReviewStatus = DerivePRStatus(pr)
	return pr
}

func extractCheckState(node gqlPR) string {
	if len(node.Commits.Nodes) == 0 {
		return ""
	}
	if r := node.Commits.Nodes[0].Commit.StatusCheckRollup; r != nil {
		return r.State
	}
	return ""
}

// isBotAuthor returns true if the author is a bot or automated account.
// Checks GraphQL __typename ("Bot", "Mannequin") and common login patterns
// like "dependabot[bot]", "app/dependabot", "renovate[bot]".
func isBotAuthor(login, typename string) bool {
	switch typename {
	case "Bot", "Mannequin":
		return true
	}
	return strings.Contains(login, "[bot]") || strings.HasPrefix(login, "app/")
}

// DerivePRStatus computes the review status from the PR's reviews and mergeable field.
func DerivePRStatus(pr PR) PRStatus {
	if pr.Mergeable == "CONFLICTING" {
		return StatusConflict
	}
	return statusFromLatestReviews(collectLatestReviews(pr.Reviews))
}

// collectLatestReviews groups reviews by reviewer login, keeping only the most
// recent actionable review per person (COMMENTED and DISMISSED are skipped).
func collectLatestReviews(reviews []Review) map[string]Review {
	latest := map[string]Review{}
	for _, r := range reviews {
		if r.State == "COMMENTED" || r.State == "DISMISSED" {
			continue
		}
		if prev, ok := latest[r.Author.Login]; !ok || r.SubmittedAt.After(prev.SubmittedAt) {
			latest[r.Author.Login] = r
		}
	}
	return latest
}

// statusFromLatestReviews derives PRStatus from the latest-per-reviewer map.
func statusFromLatestReviews(latest map[string]Review) PRStatus {
	hasApproved := false
	for _, r := range latest {
		if r.State == "CHANGES_REQUESTED" {
			return StatusChangesRequested
		}
		if r.State == "APPROVED" {
			hasApproved = true
		}
	}
	if hasApproved {
		return StatusApproved
	}
	return StatusPending
}
