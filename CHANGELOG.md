# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Initial project scaffold: Go backend skeleton, embedded SvelteKit frontend, SQLite
  store with migrations, configuration system and authentication groundwork.
- ICS transform pipeline (`internal/ics`, `internal/pipeline`) over go-ical: parse/
  serialize helpers and sequential rules — filter (black/whitelist), dedup, rename,
  field strip, timezone normalization and expiry — with substring ("DAU") or regex
  matching.
- ICS proxy with caching (`internal/proxy`, `feed_cache` table): TTL-honored upstream
  fetch with ETag revalidation and stale-on-error fallback to the last good copy.
- Output feeds (`feeds` table, `internal/feed`): configurable feed definitions served at
  `/ics/<secret>` — fetch and merge multiple sources (de-duplicated by UID), apply the
  rule pipeline and serialize, secured by secret-id with optional HTTP Basic Auth. Rule
  pipelines are built from JSON config (`pipeline.RuleConfig`).
- Feed management API (`/api/feeds`, session-authenticated): list/create/get/update/delete
  owner-scoped feeds plus a `/api/feeds/preview` endpoint returning original-vs-transformed
  events for a diff view. Secrets/passwords are write-only in the API.
- Web UI for feeds: list view with copyable ICS URLs, a create/edit form with a source
  list and a per-type rule editor, and a live original-vs-transformed preview.
- Notifications (`internal/notify`): webhook, ntfy and Gotify senders with a
  failure-tolerant dispatcher and config constructor.
- Audit log (`audit_log` table, `internal/audit`): admin-visible record of feed
  create/update/delete. The first registered user becomes admin; read via
  `GET /api/audit`.
- Web UI: admin audit-log page and a session-aware navigation (sign in/out, Feeds,
  Audit) backed by the session store.
- Web UI: email+password registration page (shown when registration is enabled) and a
  post-login/post-register redirect to the feeds view.
- In-process scheduler (`internal/scheduler`) running error-tolerant interval jobs; wired
  to purge expired sessions hourly.
- Pipeline match reporting: filter and rename rules record matched event summaries,
  exposed via `Pipeline.Matches()` — the foundation for rule-triggered notifications.

[Unreleased]: https://github.com/Norrodar/TidyDAV/commits/main
