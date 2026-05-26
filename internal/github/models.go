package github

import "time"

type PRStatus string

const (
	StatusPending          PRStatus = "pending"
	StatusApproved         PRStatus = "approved"
	StatusChangesRequested PRStatus = "changes_requested"
	StatusConflict         PRStatus = "conflict"
)

type ReviewEvent string

const (
	ReviewApprove        ReviewEvent = "APPROVE"
	ReviewRequestChanges ReviewEvent = "REQUEST_CHANGES"
	ReviewComment        ReviewEvent = "COMMENT"
)

type User struct {
	Login string
	IsBot bool // true for Bot, Mannequin, or logins containing "[bot]"
}

type Review struct {
	Author      User
	State       string // APPROVED, CHANGES_REQUESTED, COMMENTED, DISMISSED
	SubmittedAt time.Time
}

// Comment is a general PR comment (not attached to a diff line).
type Comment struct {
	Author    User
	Body      string
	CreatedAt time.Time
}

// LineComment is an inline comment attached to a specific diff line.
type LineComment struct {
	Author User
	Body   string
	Path   string
	Line   int
}

type PR struct {
	Number             int
	Title              string
	Body               string // PR description (markdown)
	URL                string
	IsDraft            bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
	Additions          int
	Deletions          int
	HeadRef            string
	BaseRef            string
	Mergeable          string // MERGEABLE, CONFLICTING, UNKNOWN
	Author             User
	Repo               string // "owner/repo"
	Reviews            []Review
	RequestedReviewers []User
	ReviewStatus       PRStatus
	// CheckState is the aggregate CI status from statusCheckRollup.
	// Values: "SUCCESS", "FAILURE", "PENDING", "ERROR", "EXPECTED", "" (no checks configured)
	CheckState     string
	Comments       []Comment     // general PR comments (loaded on-demand)
	LineComments   []LineComment // inline review comments (loaded on-demand)
	CommentsLoaded bool          // true once comments have been fetched
	// Pagination cursor from the repo this PR belongs to (used for load-more)
	HasNextPage bool
	EndCursor   string
}

// Mergeable values from the GitHub GraphQL API.
const (
	MergeableConflicting = "CONFLICTING"
	MergeableMergeable   = "MERGEABLE"
	MergeableUnknown     = "UNKNOWN"
)

// CheckState values from statusCheckRollup in the GitHub GraphQL API.
const (
	CheckStateSuccess    = "SUCCESS"
	CheckStateFailure    = "FAILURE"
	CheckStateError      = "ERROR"
	CheckStatePending    = "PENDING"
	CheckStateInProgress = "IN_PROGRESS"
	CheckStateQueued     = "QUEUED"
	CheckStateExpected   = "EXPECTED"
)

// InlineComment is a pending review comment attached to a specific diff line.
type InlineComment struct {
	Path string // file path, e.g. "auth/token.go"
	Line int    // line number in the file
	Side string // "RIGHT" (new file) or "LEFT" (old file)
	Body string
}
