# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

---

## [Unreleased]

### Fixed

- `anprr repos add` now returns an error and exits non-zero when the repository is already present in the config (or in the target scope)

### Added

- **Head branch in PR detail** — header now shows `head → base` instead of only the base branch
- **Scopes** — named profiles in config (`[scopes.work]`, `[scopes.personal]`) each with their own `token` and `repos`; select with `--scope <name>` on any command
- `anprr scopes list` — lists all configured scopes

### Fixed

- **Spinner stuck after submit** — "Submitting…" footer no longer stays visible after a successful review approval or comment post; detail view correctly returns to ready state

## [0.1.0] - 2026-05-23

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
- **CI checks** — CI status shown in PR list and detail screen with pass/fail/pending indicators
- **PR description and comments tabs** — detail screen has tabs for Diff, Description, and Comments
- **`--version` flag** — prints the current version and exits
- **Makefile** — `make build`, `make test`, `make lint`; `make help` is the default target

[Unreleased]: https://github.com/roramirez/anprr/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/roramirez/anprr/releases/tag/v0.1.0
