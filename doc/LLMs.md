# anprr — LLM Quick Reference

GitHub PR review TUI built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).
Full spec: `SPEC.md`. This file is the dense implementation reference for LLMs.

## File Map

| File | What lives here |
|---|---|
| `main.go` | CLI entry point; subcommand dispatch; `mustResolveApp`; `cmdLogin/Repos/Scopes` |
| `internal/config/config.go` | `Config`, `Scope`; `Load`/`Save`/`ResolveScope`/`ResolveToken`/`ValidateRepo` |
| `internal/github/client.go` | `Client` — HTTP, rate-limit tracking, `do` helper |
| `internal/github/rest.go` | REST calls: `SubmitReview`, `MergePR`, `PostInlineComment` |
| `internal/github/graphql.go` | GraphQL calls: `FetchPRs`, `FetchPRDetails`, `SearchReviewRequested` |
| `internal/github/cache.go` | `Cache` — in-memory store for PR details |
| `internal/github/models.go` | `PR`, `Review`, `User`, `PRStatus`, `ReviewEvent` enums |
| `internal/tui/app.go` | `AppModel` — root Bubble Tea model; routes between screens |
| `internal/tui/list.go` | `ListModel` — PR list screen; tabs (my/needsReview/allOpen) |
| `internal/tui/detail.go` | `DetailModel` — PR detail screen; diff view, comment input, actions |
| `internal/tui/messages.go` | All `tea.Msg` types shared across TUI models |
| `internal/tui/keys.go` | `keyMap` — keybindings and help text |
| `internal/tui/styles.go` | All `lipgloss.Style` vars (prefix `Style`) |
| `internal/tui/help.go` | Help overlay render |
| `internal/diff/parser.go` | `Parse` — unified diff → `[]DiffLine` |
| `internal/diff/render.go` | `Render` — unified diff ANSI string; `Highlighter` interface |
| `internal/diff/split.go` | `Split` — `[]DiffLine` → `[]SplitRow` (side-by-side pairing) |
| `internal/diff/split_render.go` | `RenderSplit` — side-by-side ANSI string |
| `internal/demo/data.go` | Mock data for `--demo` mode |

## Core Types

```go
// AppModel (internal/tui/app.go)
type AppModel struct {
    client        *github.Client
    cache         *github.Cache
    currentUser   string
    repos         []string
    syntaxHL      bool
    width, height int
    tooSmall      bool
    active        screen        // screenList | screenDetail
    list          ListModel
    detail        DetailModel
    showHelp      bool
    statusText    string
    statusIsError bool
    statusIsOK    bool
}

// ListModel (internal/tui/list.go)
type ListModel struct {
    state              listState   // loading | ready
    tab                tabIndex    // tabMyPRs | tabNeedsReview | tabAllOpen
    allPRs             []github.PR
    cursor             int
    currentUser        string
    width, height      int
    spinner            spinner.Model
    err                error
    hasNextPage        map[string]bool
    endCursor          map[string]string
    reviewRequestedSet map[string]bool // "owner/repo#number" → true
}

// DetailModel (internal/tui/detail.go)
type DetailModel struct {
    state         detailState // loading | ready | lineSelect | approveConfirm | mergeConfirm | commentInput | submitting
    pending       pendingAction
    pr            *github.PR
    viewport      viewport.Model
    textarea      textarea.Model
    spinner       spinner.Model
    cursor        int  // line-select cursor (index into DiffLines)
    splitView     bool // unified vs side-by-side diff
    commented     map[int]bool // diffLine indices with pending comments
    // ... width, height, err, etc.
}

// github.PR (internal/github/models.go)
type PR struct {
    Number       int
    Title        string
    HeadBranch   string
    BaseBranch   string
    Author       User
    State        string
    IsDraft      bool
    Mergeable    string
    CreatedAt    time.Time
    UpdatedAt    time.Time
    URL          string
    Repo         string     // "owner/repo"
    Reviews      []Review
    Status       PRStatus
    Diff         string
    Body         string
    Comments     []Comment
    // ... more fields
}
```

## Key Invariants

- `mustResolveApp` in `main.go` is the single entry point for resolving token + repos + config; do not inline this logic elsewhere.
- All `tea.Msg` types live in `internal/tui/messages.go`; do not define new ones in `app.go`, `list.go`, or `detail.go`.
- All `lipgloss.Style` vars live in `internal/tui/styles.go` with the `Style` prefix; do not create styles inline in render functions.
- The `Highlighter` interface (`internal/diff/render.go`) is the only way to pass syntax coloring into diff rendering — do not reach into `chroma` directly from outside the `diff` package.
- `colSep = 2` in `list.go` is the fixed column separator width used in `fixedW` calculations; never replace with a magic literal.
- Extract helpers when a function's cognitive complexity would exceed ~10; see commit history for the pattern (`mustResolveApp`, `renderSplitHeaderRow`, `footerExtraHeight`).
- Replace repeated string/color literals with named constants before a function exceeds B+ Halstead effort.

## Code Style

- Run `gofmt -l -w .` before every commit — all code must be `gofmt`-clean.
- Never manually align `struct` fields, `case` blocks, or function args; let `gofmt` decide.
- All code must pass `go vet ./...` with zero errors.
- No exported symbols without a doc comment.
- Prefer early returns over deep nesting to keep cognitive complexity low.
- Named constants over magic literals — one constant per repeated value, even if it's just `2`.

## Testing

- Every feature or bug fix must include tests covering the new or changed behavior.
- Tests live in `*_test.go` files in the same package.
- Run `go test -race ./...` before reporting a task complete.
- Use table-driven tests for functions with multiple input variants.
- For TUI model changes: test `Update` with the relevant `tea.Msg`, not just `View`.

## Commit Convention

Use [Conventional Commits](https://www.conventionalcommits.org/) with these scopes:

| Scope | When |
|---|---|
| `anprr` | CLI entry point / subcommand changes |
| `tui` | Any change under `internal/tui/` |
| `diff` | Any change under `internal/diff/` |
| `github` | Any change under `internal/github/` |
| `config` | Any change under `internal/config/` |
| `demo` | Any change under `internal/demo/` |

Every commit that adds a user-visible change must include an entry in `CHANGELOG.md` under `## [Unreleased]`.

## Build & Check

```sh
make build    # compile binary
make test     # go test -race ./...
make fmt      # gofmt -l -w .
make vet      # go vet ./...
make check    # fmt + vet + verify + test (run before committing)
make lint     # golangci-lint run ./...
```

Use `/commit` skill to validate, compose, and push a commit — it runs `gofmt`, `go vet`, and `go test -race` automatically.

## Code Quality Gates (kimun)

`km` ([kimun](https://github.com/lnds/kimun)) — static + git analysis. Install: `cargo install --git https://github.com/lnds/kimun`. Config: `.kimun.toml` at repo root. Score target: see `fail_below` there.

Before every commit:
```sh
km score --trend origin/main --fail-if-worse
```

Before touching a file:
```sh
km hotspots    # high churn × complexity files
km knowledge   # bus-factor risk (>80% single author)
```

In PR context:
```sh
km score diff main   # per-dimension delta; negative deltas need justification
```

Do not let the score drop below **A+**.

Key drivers for regressions:
- **Cognitive complexity** — extract helpers when a function branches more than ~10 times.
- **Halstead effort** — replace repeated literals with named constants; reduce operand vocabulary.
- **Dead code** — remove unused vars and blank identifiers immediately.
