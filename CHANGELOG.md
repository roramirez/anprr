# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

---

## [Unreleased]

### Added

- **3-tab PR list** — `[1] My PRs`, `[2] Needs Review`, `[3] All Open`
- **Precise "Needs Review"** — GitHub Search API (`review-requested:@me`) combined with re-review detection (new commits after last review); bot PRs (Dependabot, Renovate) appear when pending, excluded when already reviewed
- **PR detail + diff viewer** — scrollable unified diff with `bubbles/viewport`; `j`/`k`, `pgdn`/`pgup`, mouse scroll
- **Side-by-side split diff** — toggle with `s`; pairs removed/added lines, empty slots filled with `░` filler
- **Syntax highlighting** — on by default using chroma with file-level tokenization (multi-line strings, block comments correctly colored); disable with `--no-syntax`
- **Inline review comments** — enter line-select mode with `n`, navigate with `j`/`k`, add comment per line; accumulated comments sent with the final review
- **Multi-line comment textarea** — `ctrl+d` to submit, `enter` for new line, `esc` to cancel
- **Approve confirmation** — `a` opens a prompt: approve now or add an optional comment
- **Merge from TUI** — `m` selects squash / merge commit / rebase without leaving the terminal; blocked on drafts and conflicts
- **Request changes + post comment** — `r` and `c` with textarea input
- **Open in browser** — `w` opens the PR URL
- **Help overlay** — `?` shows all key bindings grouped by context
- **Rate limit indicator** — yellow warning in header when fewer than 100 API requests remain
- **Auto-refresh** — PR list refreshes every 60 seconds; manual refresh with `f`
- **Pagination** — `F` loads more PRs when a repo has more than 50 open
- **Bot PR detection** — Dependabot and other bots detected via GraphQL `__typename` and login patterns; shown dimmed with `[bot]` prefix
- **Draft PR detection** — drafts shown dimmed with `[draft]` prefix; approve blocked
- **Demo mode** — `./anprr --demo` runs with mock data, no token required
- **Config subcommands** — `anprr login`, `anprr repos add/remove/list`
- **CI** — GitHub Actions workflow: `gofmt`, `go vet`, `go test -race`, binary build
- **Dependabot** — weekly updates for Go modules and GitHub Actions

- **GraphQL aliases** — numeric identifiers (`r0`, `r1`, …) support repos with hyphens, dots, and other special characters
- **Syntax highlight color fix** — correct `#rrggbb` format for chroma v2 token colors

[Unreleased]: https://github.com/roramirez/anprr/compare/main...HEAD
