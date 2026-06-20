# TidyDAV

Self-hosted CalDAV/CardDAV proxy, ICS transformer and DAV sync service shipped as a
single lightweight Docker container.

## Project Overview

TidyDAV does three things:

1. **ICS Proxy & Transformer** — fetches one or more upstream ICS feeds, runs them
   through a sequential rule pipeline (filter, dedup, rename, strip, timezone, expiry),
   optionally merges multiple feeds, and re-serves a clean ICS endpoint. Feeds are
   cached in SQLite so a dead upstream still serves the last good copy.
2. **DAV Sync** — synchronises CalDAV (calendars), CardDAV (contacts) and VTODO (tasks)
   between two external DAV servers as independent jobs (uni- or bidirectional, with
   conflict resolution).
3. **Notifications & Audit** — fires webhook/ntfy/Gotify notifications on rule matches
   and keeps an admin-only audit log of config changes.

Single binary, no runtime dependency except SQLite. No CGO. No Node.js at runtime.

## Tech Stack

| Layer | Choice | Rationale |
|---|---|---|
| Backend | Go, `net/http`, no HTTP framework | Stdlib router (Go 1.22+ `ServeMux` patterns) is enough; fewer deps, smaller binary. |
| Build | `CGO_ENABLED=0` everywhere | Static binary, trivial cross-compile, minimal container. Non-negotiable. |
| Database | SQLite via `modernc.org/sqlite` | Pure-Go driver, no CGO. Only runtime data dependency. |
| ICS / vCard | `github.com/emersion/go-ical`, `github.com/emersion/go-vcard` | Pure-Go, well-maintained parse/serialise. |
| DAV client | `github.com/emersion/go-webdav` (caldav/carddav) | Pure-Go CalDAV/CardDAV client; ETag/CTag aware. |
| Auth | `github.com/coreos/go-oidc/v3` + `golang.org/x/oauth2`, `golang.org/x/crypto/bcrypt` | OIDC discovery for any provider; bcrypt for passwords. |
| Frontend | SvelteKit + Svelte 5 (runes), TypeScript | Modern reactivity (`$state`/`$derived`/`$effect`), strict TS. |
| Frontend integration | `adapter-static` (SPA) → `embed.FS` | Static assets embedded in the Go binary; no Node at runtime. |
| CSS | Own design system via CSS custom properties | No component libraries (hard rule). |
| Container | Two-stage Dockerfile (Node build → Go build) | Minimal final image (scratch/distroless-style). |

## Directory Structure

```
TidyDAV/
├── cmd/tidydav/main.go        # entrypoint: load config, wire app, start server
├── internal/
│   ├── app/                   # App struct = the single initialised state container
│   ├── config/                # env parsing + validation (TIDYDAV_* vars)
│   ├── server/                # http.Server, ServeMux routing
│   │   └── middleware/        # auth, request-id, logging, recover
│   ├── auth/                  # sessions, oidc, password, secret-id
│   ├── store/                 # SQLite access layer
│   │   └── migrations/        # NNN_name.sql, embedded, applied on boot
│   ├── ics/                   # ICS read/transform helpers over go-ical
│   ├── proxy/                 # upstream fetch + SQLite cache (TTL, stale-on-error)
│   ├── pipeline/              # rule pipeline: filter, dedup, rename, strip, timezone, expire
│   ├── dav/                   # sync engine: client, sync, conflict
│   ├── notify/                # webhook, ntfy, gotify
│   ├── audit/                 # audit log writer/reader
│   ├── scheduler/             # in-process interval job runner
│   └── ui/                    # //go:embed all:dist  (built SvelteKit SPA)
├── web/                       # SvelteKit project (Svelte 5, TS) — builds into internal/ui/dist
├── deploy/                    # compose.example.yaml, .env.example
├── .github/                   # ISSUE_TEMPLATE/, PULL_REQUEST_TEMPLATE.md
├── Dockerfile
├── go.mod / go.sum
├── CLAUDE.md / CLA.md / CONTRIBUTING.md / CHANGELOG.md / README.md / LICENSE
```

**Frontend embedding:** SvelteKit `adapter-static` (SPA, `fallback: index.html`) writes
`pages`/`assets` to `../internal/ui/dist`. `internal/ui/embed.go` does
`//go:embed all:dist`. A committed placeholder `internal/ui/dist/index.html` keeps
`go build` working without a frontend build; the Docker/CI frontend build overwrites it.

## Architecture Principles

- **Single initialised state.** One `app.App` struct holds config, DB handle, loggers,
  schedulers. Passed explicitly. No other global mutable state (hard rule).
- **`internal/` for everything app-specific.** Nothing is exported for external import.
- **Errors are values.** Return `error`, wrap with `fmt.Errorf("...: %w", err)`. No
  `panic()` in production paths (hard rule). A failing feed or sync job is logged and
  surfaced — it never crashes the server. The HTTP recover middleware is a safety net,
  not a control-flow tool.
- **Context first.** Exported funcs that do I/O take `ctx context.Context` as first arg.
- **Small packages, small interfaces.** Define interfaces where consumed.
- **Routing surface:**
  - `/api/*` — JSON API for the web UI, session-authenticated.
  - `/ics/*` — transformed ICS output, secured by secret-id (query/path), optional HTTP
    Basic Auth. No session (calendar clients can't do OIDC).
  - `/auth/*` — login/register/reset/logout, OIDC login + callback.
  - everything else → SPA fallback (`index.html`).

## Coding Conventions

**Go**
- `gofmt` + `goimports`; `go vet` and `golangci-lint` clean before commit.
- Package names: short, lowercase, singular, no underscores.
- Exported identifiers documented with a `// Name ...` comment.
- Constructors `New...` return `(*T, error)`; no init-time panics.
- SQL lives in `store/`; the rest of the app talks to typed `store` methods.
- Times stored as UTC (RFC3339 / unix); convert at the edges.

**Frontend**
- Svelte 5 runes only (`$state`, `$derived`, `$effect`). No legacy stores unless a
  cross-route singleton is genuinely needed.
- TypeScript `strict`. Components `PascalCase.svelte`. Route files lowercase.
- Styling only via design tokens (CSS custom properties) in scoped `<style>` blocks or
  the global token sheet. No utility-class frameworks, no component libraries.
- API access through a single typed client in `web/src/lib/api.ts`.

**General**
- Commit messages in English, Conventional Commits (`feat:`, `fix:`, `chore:` …).
- No hardcoded secrets or config values — everything via `TIDYDAV_*` env vars.

## Environment Variables

Prefix `TIDYDAV_`. Only two are required.

### Instance
| Var | Req | Default | Description |
|---|---|---|---|
| `TIDYDAV_SECRET_KEY` | **yes** | — | Key for signing sessions/cookies. Generate: `openssl rand -hex 32`. |
| `TIDYDAV_BASE_URL` | **yes** | — | Public base URL, e.g. `https://dav.example.com`. Used for OIDC redirect, ICS links, email links. |
| `TIDYDAV_DB_PATH` | no | `/data/tidydav.db` | SQLite file path. |
| `TIDYDAV_LISTEN_ADDR` | no | `:8080` | Listen address. |
| `TIDYDAV_LOG_LEVEL` | no | `info` | `debug` \| `info` \| `warn` \| `error`. |
| `TIDYDAV_NOTIFY_INTERVAL` | no | `15m` | How often the background notifier scans feeds for rule matches (Go duration). |

### Access mode
| Var | Req | Default | Description |
|---|---|---|---|
| `TIDYDAV_ACCESS_MODE` | no | `auth` | `public` (everyone gets a secret-id), `auth` (OIDC / email+pw only), `both`. |
| `TIDYDAV_ALLOW_REGISTRATION` | no | `true` | Allow email+password self-registration (only relevant when auth is enabled). |
| `TIDYDAV_ALLOW_PRIVATE_TARGETS` | no | `true` | Allow the feed proxy to reach loopback/private/link-local hosts. Set `false` on multi-user/public instances (SSRF mitigation). |

### OIDC (optional — enabled when issuer + client id are set)
| Var | Req | Default | Description |
|---|---|---|---|
| `TIDYDAV_OIDC_ISSUER_URL` | no | — | Discovery URL (any provider). |
| `TIDYDAV_OIDC_CLIENT_ID` | no | — | Client ID. |
| `TIDYDAV_OIDC_CLIENT_SECRET` | no | — | Client secret. |
| `TIDYDAV_OIDC_SCOPES` | no | `openid profile email` | Space-separated scopes. |

### SMTP (optional — enables password reset & email verification)
| Var | Req | Default | Description |
|---|---|---|---|
| `TIDYDAV_SMTP_HOST` | no | — | SMTP server host. |
| `TIDYDAV_SMTP_PORT` | no | `587` | SMTP port. |
| `TIDYDAV_SMTP_USERNAME` | no | — | SMTP username. |
| `TIDYDAV_SMTP_PASSWORD` | no | — | SMTP password. |
| `TIDYDAV_SMTP_FROM` | no | — | From address. |
| `TIDYDAV_SMTP_ENCRYPTION` | no | `starttls` | `starttls` \| `tls` \| `none`. |

### Container user
| Var | Req | Default | Description |
|---|---|---|---|
| `PUID` | no | `1000` | User id the process runs as (filesystem permissions). |
| `PGID` | no | `1000` | Group id. |
| `TZ` | no | `UTC` | Container timezone. |

Config validation fails fast on boot with a clear message if a required var is missing
or an enum value is invalid.

## Design System (Dark mode only, v1)

Apple-leaning: clarity, depth, consistency, generous whitespace, one accent, restrained
palette, soft purposeful motion. Fonts from Google Fonts: **Inter** (UI) and **JetBrains
Mono** (IDs/code). Inter is a humanist grotesque that matches the SF aesthetic; a mono
face keeps secret-ids/URLs legible.

Design tokens (defined once in `web/src/lib/styles/tokens.css`):

```css
:root {
  /* Surfaces — layered near-black for depth */
  --bg-base:      #0a0a0c;
  --bg-elevated:  #161618;
  --bg-overlay:   rgba(28, 28, 32, 0.72); /* blur layers */
  --separator:    rgba(255, 255, 255, 0.10);

  /* Text — opacity ramp on white */
  --text-primary:   rgba(255, 255, 255, 0.92);
  --text-secondary: rgba(255, 255, 255, 0.58);
  --text-tertiary:  rgba(255, 255, 255, 0.36);

  /* Accent (single) */
  --accent:       #0a84ff;
  --accent-hover: #3a9bff;
  --accent-text:  #ffffff;

  /* Semantic — status only */
  --success: #30d158;
  --warning: #ff9f0a;
  --danger:  #ff453a;

  /* Radii */
  --radius-sm: 6px;  --radius-md: 10px;  --radius-lg: 16px;  --radius-full: 9999px;

  /* Spacing (4px base) */
  --space-1: 4px;  --space-2: 8px;  --space-3: 12px; --space-4: 16px;
  --space-5: 24px; --space-6: 32px; --space-7: 48px; --space-8: 64px;

  /* Type scale (1rem = 16px base) */
  --text-xs: 0.75rem;  --text-sm: 0.875rem; --text-base: 1rem;  --text-lg: 1.125rem;
  --text-xl: 1.375rem; --text-2xl: 1.75rem; --text-3xl: 2.25rem;
  --font-ui:   'Inter', system-ui, sans-serif;
  --font-mono: 'JetBrains Mono', ui-monospace, monospace;
  --weight-regular: 400; --weight-medium: 500; --weight-semibold: 600;

  /* Depth */
  --shadow-sm: 0 1px 2px rgba(0,0,0,0.4);
  --shadow-md: 0 4px 16px rgba(0,0,0,0.5);
  --blur: 20px;

  /* Motion */
  --ease: cubic-bezier(0.4, 0, 0.2, 1);
  --dur-fast: 150ms; --dur-base: 200ms; --dur-slow: 300ms;
}
```

Rationale: a single near-black surface ramp + white-opacity text ramp gives Apple-style
depth without extra hues; `#0a84ff` is calm and high-contrast on dark; semantic colors
are reserved strictly for status; no bounce, short eased transitions.

## Test Strategy

- **Every feature ships with ≥1 unit test** (hard rule).
- Go: table-driven `_test.go` next to code. Pipeline rules tested against small ICS
  fixtures (golden in/out). Handlers via `httptest`. Store via in-memory SQLite
  (`:memory:`). DAV sync logic tested against a fake/in-memory DAV client.
- Frontend: Vitest for unit-testable logic (`web/src/lib`). E2E (Playwright) optional,
  post-v1.
- CI (GitHub Actions): `go vet`, `golangci-lint`, `go test ./...`, frontend
  `npm ci && npm run check && npm run build`, then a full Docker build.

## PR Requirements & CLA

- All contributors **must sign the CLA** (see `CLA.md`) before a PR is merged — stated in
  `PULL_REQUEST_TEMPLATE.md` and `CONTRIBUTING.md`.
- PRs must: pass CI, include tests for new behavior, update `CHANGELOG.md` (Unreleased),
  and keep `CGO_ENABLED=0` (no CGO deps introduced).
- One logical change per PR; English Conventional-Commit titles.

## Local Development & Deployment

- **Recommended editor: VS Code** (or Cursor — same extensions/settings). The repo ships
  `.editorconfig`, `.vscode/extensions.json` (Go, Svelte, ESLint, Prettier) and
  `.vscode/settings.json` (format-on-save, gopls). JetBrains works too but isn't
  preconfigured.
- **Dev loop:** Go API on `:8080`; SvelteKit dev server (`npm run dev`) proxies `/api`,
  `/auth`, `/ics` to the Go process for hot reload. Production = `npm run build` →
  `internal/ui/dist` → `go build`.
- **Deployment target: Linux server / NAS via Docker.** Run as non-root using `PUID`/
  `PGID`; persist `/data` (SQLite) as a volume; terminate TLS at a reverse proxy and set
  `TIDYDAV_BASE_URL` to the public URL. `compose.example.yaml` is the reference.

## Hard Rules (do not violate)

- No component libraries (no shadcn/DaisyUI/Bootstrap/Flowbite).
- No `panic()` in production paths.
- No hardcoded secrets/config.
- No global mutable state except the single initialised `app.App`.
- No feature without at least one unit test.
- No docs written before the corresponding code exists.
- No CGO, no CGO-requiring dependency.
