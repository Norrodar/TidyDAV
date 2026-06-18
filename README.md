<div align="center">

# TidyDAV

**Self-hosted CalDAV/CardDAV proxy, ICS transformer and DAV sync — in a single lightweight container.**

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go&logoColor=white)](go.mod)
[![Svelte](https://img.shields.io/badge/Svelte-5-FF3E00?logo=svelte&logoColor=white)](web/)
[![CGO](https://img.shields.io/badge/CGO-disabled-success)](Dockerfile)
[![Status](https://img.shields.io/badge/status-early%20scaffold-orange)](CHANGELOG.md)

</div>

> ⚠️ **Early development.** TidyDAV is being scaffolded; features land incrementally. The
> API, configuration and database schema may change without notice until the first
> tagged release.

## What is TidyDAV?

TidyDAV cleans up and connects your calendars and contacts:

- **ICS proxy & transformer** — pull one or more upstream ICS feeds, run them through a
  rule pipeline (filter, dedup, rename, strip fields, normalize timezone, drop expired
  events), optionally merge several feeds, and serve a clean, cacheable ICS endpoint.
- **DAV sync** — synchronize CalDAV calendars, CardDAV contacts and VTODO tasks between
  two servers, one- or bidirectionally, with conflict resolution.
- **Notifications & audit** — trigger webhook / ntfy / Gotify notifications on rule
  matches, and keep an admin-only audit log of configuration changes.

It is a single static Go binary with an embedded web UI. The only runtime dependency is
SQLite. No CGO, no Node.js at runtime, no Redis.

## Quickstart

> Placeholder — the container image and full docs are not published yet. From a clone:

```bash
cd deploy
cp .env.example .env
# set TIDYDAV_SECRET_KEY (openssl rand -hex 32) and TIDYDAV_BASE_URL
docker compose -f compose.example.yaml up -d
```

Then open `TIDYDAV_BASE_URL` in your browser. Put TidyDAV behind a reverse proxy that
terminates TLS.

## Configuration

All configuration is via `TIDYDAV_*` environment variables — only `TIDYDAV_SECRET_KEY`
and `TIDYDAV_BASE_URL` are required. See [`deploy/.env.example`](deploy/.env.example) for
the full, documented reference.

## Documentation

- [CLAUDE.md](CLAUDE.md) — architecture, conventions and design system
- [CONTRIBUTING.md](CONTRIBUTING.md) — development setup and contribution process
- [CHANGELOG.md](CHANGELOG.md) — release notes

## Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md). All contributors
must agree to the [Contributor License Agreement](CLA.md).

## License

TidyDAV is licensed under the [GNU Affero General Public License v3.0](LICENSE).
