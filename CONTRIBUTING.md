# Contributing to TidyDAV

Thanks for considering a contribution! This document explains how to get set up and what
we expect from pull requests.

## Contributor License Agreement (required)

Before any pull request can be merged you must agree to the
[Contributor License Agreement](CLA.md). You do this by ticking the CLA checkbox in the
pull-request template. PRs without an accepted CLA will not be merged.

## Development setup

**Prerequisites:** Go 1.25+, Node.js 20+, Docker (optional, for image builds).

```bash
# 1. Backend deps
go mod download

# 2. Frontend deps
cd web && npm install && cd ..

# 3. Run the API (serves the last built frontend from internal/ui/dist)
go run ./cmd/tidydav

# 4. In a second terminal, run the SvelteKit dev server with hot reload.
#    It proxies /api, /auth and /ics to the Go process on :8080.
cd web && npm run dev
```

For a production-style build, run `npm run build` in `web/` (writes
`internal/ui/dist`) and then `go build ./cmd/tidydav` — the frontend is embedded into the
single binary.

Recommended editor is **VS Code** (or Cursor); the repo ships `.vscode/` recommendations
and an `.editorconfig`.

## Coding conventions

- **Go:** `gofmt` + `goimports`; keep `go vet ./...` and `golangci-lint run` clean. No
  `panic()` in production paths. No global mutable state except the single `app.App`.
  Keep `CGO_ENABLED=0` — do not introduce CGO dependencies.
- **Frontend:** Svelte 5 runes (`$state`/`$derived`/`$effect`), TypeScript `strict`,
  styling via the design tokens only. No component libraries.
- **Secrets/config:** never hardcode — use `TIDYDAV_*` environment variables.

See [CLAUDE.md](CLAUDE.md) for the full architecture and conventions.

## Tests

Every change that adds or changes behavior must include at least one unit test.

```bash
go test ./...
cd web && npm run check   # svelte-check / type checks
```

## Commits & pull requests

- Use [Conventional Commits](https://www.conventionalcommits.org/) in **English**:
  `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:` …
- Branch off `main` (e.g. `feat/feed-dedup`), keep one logical change per PR.
- Update `CHANGELOG.md` under **Unreleased**.
- Fill in the pull-request template, including the CLA checkbox.
- PRs are squash-merged; make sure the PR title is a valid Conventional Commit.

## Reporting bugs & requesting features

Use the issue templates for [bug reports](.github/ISSUE_TEMPLATE/bug_report.md) and
[feature requests](.github/ISSUE_TEMPLATE/feature_request.md). For security issues, please
do **not** open a public issue — see the security policy (to be added) or contact the
maintainer privately.
