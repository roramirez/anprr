# anprr — Functional Specification

## 1. Commands

### `anprr` (no args)
Launches the TUI. Requires a token and at least one repo configured.

- If no token: print `"No token configured. Run: anprr login --token <token>"` and exit 1
- If no repos: print `"No repositories configured. Run: anprr repos add <owner/repo>"` and exit 1
- If terminal < 80x24: print `"Terminal too small (need 80x24, current NxM)."` and exit 1

### `anprr login --token <token>`
Saves the token to `~/.config/anprr/config.toml`. Creates the file if it does not exist.
- Validates that the token is non-empty
- Does NOT validate against GitHub (no network call)
- Prints `"Token saved."` on success

### `anprr repos add <owner/repo>`
Appends `owner/repo` to the `repos` list in config. Does not deduplicate — adding the same repo twice shows it twice (user error).
- Prints `"Added owner/repo."` on success
- Prints error if the argument is missing or not in `owner/repo` format

### `anprr repos remove <owner/repo>`
Removes all occurrences of `owner/repo` from the repos list.
- Prints `"Removed owner/repo."` on success
- Prints `"owner/repo not found."` if not present (exit 0)

### `anprr repos list`
Prints each configured repo on its own line. Prints `"(none)"` if the list is empty.

### `anprr help` / `anprr --help` / `anprr -h`
Prints command usage and exits 0.

---

## 2. Auth flow

Priority chain (first non-empty wins):

1. `--token <value>` flag passed to `anprr`
2. `GITHUB_TOKEN` environment variable
3. `token` field in `~/.config/anprr/config.toml`

Error messages:
- Empty token after resolution → `"No token configured. Run: anprr login --token <token>"`
- HTTP 401 from GitHub → `"Authentication failed. Check your token."`
- HTTP 403 from GitHub → `"Permission denied. Your token may lack required scopes (repo, read:user)."`

---

## 3. Config file

Location: `$XDG_CONFIG_HOME/anprr/config.toml` if `XDG_CONFIG_HOME` is set, otherwise `~/.config/anprr/config.toml`.

```toml
token  = "ghp_xxxx"
repos  = ["myorg/backend", "myorg/frontend"]
no-syntax = false   # set to true to disable syntax highlighting (on by default)
```

All fields are optional — missing fields use zero values. The file and its parent directory are created on first `anprr login` call.

---

## 4. TUI screens

### 4.1 Minimum terminal size

Width ≥ 80 columns, height ≥ 24 rows. If the terminal is too small on launch OR is resized below minimum during use, the TUI renders:

```
Terminal too small (need 80x24, current NxM).
Resize to continue.
```

Normal rendering resumes automatically on the next resize event above the minimum.

### 4.2 Screen 1 — PR List

```
┌─ anprr ── ⚡ 4823 requests remaining ──────────────────┐
│ [1] My PRs  [2] Needs Review  [3] All Open           │
├──────────────────────────────────────────────────────┤
│ ▶ #42  fix auth bug          myorg/backend   2h  ●   │
│   #38  update deps           myorg/backend   1d  ●   │
│   #91  [draft] wip feature   myorg/frontend  3h  ○   │
│                                                      │
│   Showing 50 of 63 — press F to load more            │
├──────────────────────────────────────────────────────┤
│ 1/2/3=tab  enter=view  a=approve  r=changes          │
│ c=comment  f=refresh  ?=help  q=quit                 │
└──────────────────────────────────────────────────────┘
```

**Tabs:**

| Tab | Key | Contenido |
|-----|-----|-----------|
| My PRs | `1` | PRs donde soy el autor |
| Needs Review | `2` | PRs que específicamente necesitan mi atención (precisos) |
| All Open | `3` | Todos los PRs abiertos en repos seguidos que no son míos |

**"Needs Review" — lógica precisa:**
Un PR aparece en esta tab si:
1. **GitHub Search API** (`review-requested:@me`) confirma que estoy pedido como reviewer — incluye team requests, respeta la lógica interna de GitHub
2. **Re-review detection** — ya revisé el PR pero se hicieron push de commits después de mi última review (`pr.UpdatedAt > myLastReview.submittedAt`)

Esta tab NO usa la condición genérica "no es mío + pendiente" — es exacta.

**"All Open"** — todos los PRs no autorales sin filtrar. Sirve para tener visibilidad completa del estado del repo aunque no seas el reviewer designado.

**Status dot colors:**
- `●` yellow (`#FFBF00`) — changes requested
- `●` green (`#00FF7F`) — approved
- `●` gray (`#888888`) — pending (no review yet)
- `●` red (`#FF4444`) — merge conflict
- `○` gray — draft PR

**Draft PRs:** title rendered dimmed, prefixed with `[draft]`. Approve (`a`) is disabled — pressing it shows `"Cannot approve a draft PR"` in the status bar for 3 seconds.

**Pagination:** if `pageInfo.hasNextPage` is true, a notice line appears at the bottom of the list. Pressing `F` (uppercase) loads the next page and appends results.

**Loading state:** spinner (from `bubbles/spinner`) replaces the list while fetching.

**Status bar:** single line below the footer keys. Shows transient messages (3s timeout): action confirmations, errors, rate limit warnings.

### 4.3 Screen 2 — PR Detail + Diff

```
┌─ #42 fix auth bug — myorg/backend ───────────────────┐
│ author: jdoe  base: main  +45 -12  2 files           │
├──────────────────────────────────────────────────────┤
│  diff --git a/auth/token.go b/auth/token.go          │
│  @@ -23,7 +23,9 @@                                   │
│    func ValidateToken(t *Token) error {               │
│  - if token == nil {                                  │
│  + if token == nil || token.Expired() {               │
│  +   return ErrTokenExpired                           │
│    }                                                  │
│                                                      │
├──────────────────────────────────────────────────────┤
│ a=approve  r=changes  c=comment  f=refresh           │
│ b=back  w=web  ?=help                                │
└──────────────────────────────────────────────────────┘
```

**Diff viewport:** uses `bubbles/viewport`. Supports `j`/`k`, `pgdn`/`pgup`, arrow keys, and mouse wheel scroll.

**Merge (`m`):** opens a method selection prompt:
```
╭─ Merge PR #42? ────────────────────────────────╮
│  s / enter    Squash and merge (recommended)   │
│  m            Merge commit                     │
│  r            Rebase and merge                 │
│  esc          Cancel                           │
╰────────────────────────────────────────────────╯
```
- Blocked if PR is a draft → `"Cannot merge a draft PR"`
- Blocked if `mergeable == CONFLICTING` → `"PR has conflicts — resolve before merging"`
- On success: cache invalidated, returns to list, status bar shows `✓ PR merged`
- API: `PUT /repos/{owner}/{repo}/pulls/{number}/merge` with `{"merge_method": "squash"|"merge"|"rebase"}`

**Comment input:** pressing `c` or `r` opens a multi-line textarea at the bottom of the screen. `ctrl+d` submits, `esc` cancels, `enter` adds a new line.

**Loading state:** spinner while fetching diff.

### 4.4 Help overlay

Triggered by `?` on any screen. Rendered as a centered box over the current screen:

```
┌─ Key Bindings ───────────────────────────────────────┐
│                                                      │
│  1 / 2       Switch tab (list screen)                │
│  j / ↓       Move down / scroll down                 │
│  k / ↑       Move up / scroll up                     │
│  pgdn/pgup   Page scroll (detail screen)             │
│  enter       Open PR detail                          │
│  a           Approve PR                              │
│  r           Request changes                         │
│  c           Post comment                            │
│  f           Refresh                                 │
│  F           Load more PRs (list screen)             │
│  w           Open PR in browser                      │
│  b / esc     Back to list                            │
│  ?           Toggle this help                        │
│  q / ctrl+c  Quit                                    │
│                                                      │
│  Press ? or esc to close                             │
└──────────────────────────────────────────────────────┘
```

---

## 5. PR status derivation

Given the list of reviews on a PR (GitHub `reviews` field, last 10):

1. If `mergeable == "CONFLICTING"` → `StatusConflict`
2. Else, group reviews by reviewer login, take the latest per reviewer:
   - If any latest review is `CHANGES_REQUESTED` → `StatusChangesRequested`
   - Else if at least one latest review is `APPROVED` → `StatusApproved`
   - Else → `StatusPending`

Draft PRs always render as `StatusPending` regardless of reviews.

---

## 6. GitHub API contracts

### GraphQL — list PRs

Endpoint: `POST https://api.github.com/graphql`

Headers:
- `Authorization: Bearer <token>`
- `Content-Type: application/json`

Query uses dynamic aliases — one per repo — so all repos are fetched in a single request. Each alias is derived by replacing `/` with `_` in `owner/repo`.

Fields requested per PR node:
```
number, title, url, isDraft, createdAt, updatedAt,
additions, deletions, headRefName, baseRefName, mergeable,
author { login },
reviewRequests(first: 10) { nodes { requestedReviewer { ... on User { login } } } },
reviews(last: 10) { nodes { author { login } state submittedAt } }
```

Pagination fields per repo:
```
pageInfo { hasNextPage endCursor }
```

### REST — get diff

`GET https://api.github.com/repos/{owner}/{repo}/pulls/{number}`

Headers:
- `Authorization: Bearer <token>`
- `Accept: application/vnd.github.v3.diff`

Returns raw unified diff text.

### REST — submit review

`POST https://api.github.com/repos/{owner}/{repo}/pulls/{number}/reviews`

Body:
```json
{ "event": "APPROVE" | "REQUEST_CHANGES" | "COMMENT", "body": "..." }
```

### REST — post comment

`POST https://api.github.com/repos/{owner}/{repo}/issues/{number}/comments`

Body:
```json
{ "body": "..." }
```

### REST — merge PR

`PUT https://api.github.com/repos/{owner}/{repo}/pulls/{number}/merge`

Body:
```json
{ "merge_method": "squash" }
```

`merge_method` values: `squash` (default), `merge`, `rebase`.

HTTP 405 = not mergeable (conflicts or branch protections). HTTP 409 = conflict.

### REST — get current user

`GET https://api.github.com/user`

Returns `{ "login": "..." }`. Called once at startup to identify the authenticated user (used to filter "needs my review" tab).

### Error handling

| HTTP status | Error |
|-------------|-------|
| 401 | `ErrUnauthorized` |
| 403 | `ErrForbidden` |
| 404 | `ErrNotFound` |
| 422 | `ErrUnprocessable` (e.g. approving own PR) |
| 5xx | `ErrServerError` |
| network | wrapped `net` error |

All errors are surfaced in the TUI status bar, not as panics or os.Exit.

---

## 7. Cache behavior

- **Store:** in-memory map with per-entry expiry timestamps. Protected by `sync.RWMutex`.
- **TTL:** 60 seconds for PR list. 300 seconds for diffs (diffs don't change frequently).
- **Keys:** PR list keyed by `"prs"`. Diffs keyed by `"diff:{owner}/{repo}#{number}"`.
- **Invalidation:** pressing `f` (refresh) clears all cache entries and re-fetches.
- **Miss behavior:** fetch from API, store result, return. On fetch error, return error (no stale-on-error).

---

## 8. Rate limit behavior

- Every HTTP response reads `X-RateLimit-Remaining` header and stores it atomically.
- If remaining < 100: header bar shows `⚠ N requests remaining` in yellow.
- If remaining == 0: all fetch operations are blocked; status bar shows `"Rate limit exceeded. Resets at HH:MM."` using `X-RateLimit-Reset` header (Unix timestamp).

---

## 9. Syntax highlighting

- **On by default.** Disable with `--no-syntax` flag or `no-syntax = true` in config.
- Uses `ChromaHighlighter` which detects language from the `diff --git a/path/file.ext` file header.
- Unknown or unsupported extensions: falls back to `NoopHighlighter` silently.
- Syntax colors (foreground) are layered on top of diff background colors (add/remove) — they do not replace them.
