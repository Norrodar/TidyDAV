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
- Rule-triggered notifications: a background notifier (`internal/notifier`, `TIDYDAV_NOTIFY_INTERVAL`,
  default 15m) evaluates each feed's configured trigger rule types and dispatches a
  webhook/ntfy/Gotify notification the first time each matched event is seen — never on
  `/ics` polls, so calendar clients don't cause spam. Configurable per feed via the API
  (`notifications`, Gotify token write-only) and a notifications section in the feed editor.
- DAV sync engine (`internal/dav`): UID-matched CalDAV/CardDAV reconciliation — uni- and
  bidirectional, with newest-wins / source-wins conflict resolution and a change-beats-delete
  safety rule. Tested against an in-memory fake client. Includes go-webdav-backed CalDAV and
  CardDAV collection clients (Basic Auth supported) and ICS/vCard UID/modified extractors.
  (Job model, scheduler wiring and UI come next.)

### Fixed

- Notifications no longer log the Gotify token (query string) or userinfo password on
  delivery failure — the URL is redacted in error messages.
- Rename rules now reject an empty pattern, which previously inserted the replacement
  between every character of a field.
- Optional SSRF hardening for the feed proxy: `TIDYDAV_ALLOW_PRIVATE_TARGETS=false`
  refuses fetches to loopback/private/link-local addresses, validated at dial time so a
  DNS rebind cannot bypass it.
- The "first user becomes admin" decision is now atomic (count + insert in one
  transaction), so two concurrent first registrations cannot both become admin.
- Previewing a saved feed reuses its stored source passwords (the editor now sends the
  feed id), so feeds with authenticated sources no longer fail to preview after editing.

[Unreleased]: https://github.com/Norrodar/TidyDAV/commits/main
