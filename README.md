<div align="center">

# TidyDAV

**Self-hosted CalDAV/CardDAV proxy, ICS transformer and DAV sync — in a single lightweight container.**

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go&logoColor=white)](go.mod)
[![Svelte](https://img.shields.io/badge/Svelte-5-FF3E00?logo=svelte&logoColor=white)](web/)
[![CGO](https://img.shields.io/badge/CGO-disabled-success)](Dockerfile)
[![Status](https://img.shields.io/badge/status-pre--release-orange)](CHANGELOG.md)

</div>

> ⚠️ **Pre-release.** TidyDAV is feature-complete but has no tagged release yet; the
> configuration and database schema may still change. Run it from a clone.

## What is TidyDAV?

TidyDAV cleans up and connects your calendars and contacts. It is a single static Go
binary with an embedded web UI — the only runtime dependency is SQLite. No CGO, no
Node.js at runtime, no Redis.

### ICS proxy & transformer
Pull one or more upstream ICS feeds and serve a clean, cached ICS endpoint:

- **Rule pipeline** applied in order — filter (black/whitelist), dedup, rename, strip
  fields, timezone normalization, drop expired events. Each rule matches by simple
  case-insensitive substring (for non-techies) or full Go regex.
- **Merge** several feeds into one (de-duplicated by UID).
- **Caching** in SQLite with a per-feed TTL, ETag revalidation and stale-on-error
  fallback, so a dead upstream still serves the last good copy.
- **Live preview** in the UI showing the original vs. transformed events.
- Served at `/ics/<secret>`, secured by an unguessable secret-id and optional HTTP
  Basic Auth — no session, so any calendar client can subscribe.

### DAV sync
Synchronise a CalDAV calendar or CardDAV address book between two servers as independent
jobs:

- Uni- (A→B, B→A) or **bidirectional**, matched across servers by UID.
- Conflict resolution: newest-wins (`LAST-MODIFIED`) or source-wins; a change always
  beats a delete to avoid data loss.
- ETag-aware (unchanged items are skipped); run on a per-job interval or manually.

### Notifications & audit
- Fire **webhook / ntfy / Gotify** notifications the first time a feed rule matches an
  event — evaluated on a schedule, de-duplicated, never on calendar polls (no spam).
- Admin-only **audit log** of feed and sync-job changes.

### Authentication
- **OIDC** (any provider via discovery), **email + password** (with SMTP password
  reset), and an **access mode** switch (`public` / `auth` / `both`). The first
  registered user becomes admin.

## Quickstart

From a clone (builds the image locally; works out of the box):

```bash
cd deploy
cp .env.example .env
# set TIDYDAV_SECRET_KEY (openssl rand -hex 32) and TIDYDAV_BASE_URL
docker compose -f compose.example.yaml up -d
```

Then open `TIDYDAV_BASE_URL` in your browser and register the first (admin) account. Run
TidyDAV behind a reverse proxy that terminates TLS, and persist the `/data` volume.

## Configuration

All configuration is via `TIDYDAV_*` environment variables — only `TIDYDAV_SECRET_KEY`
and `TIDYDAV_BASE_URL` are required; everything else has sensible defaults. OIDC and SMTP
are optional and enable themselves once configured. See
[`deploy/.env.example`](deploy/.env.example) for the full, documented reference, and run
the container as a non-root user via `PUID`/`PGID`.

## Known limitations

See the **Known limitations** section in [CHANGELOG.md](CHANGELOG.md) — notably: VTODO is
not yet a dedicated sync kind (CalDAV jobs still carry VTODOs), sync uses per-item ETags
rather than a CTag fast-path, cross-source merge de-duplicates by UID only, and only
filter/rename rules can trigger notifications.

## Documentation

- [CLAUDE.md](CLAUDE.md) — architecture, conventions and design system
- [CONTRIBUTING.md](CONTRIBUTING.md) — development setup and contribution process
- [CHANGELOG.md](CHANGELOG.md) — changes and known limitations

## Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md). All contributors
must agree to the [Contributor License Agreement](CLA.md).

## License

TidyDAV is licensed under the [GNU Affero General Public License v3.0](LICENSE).
