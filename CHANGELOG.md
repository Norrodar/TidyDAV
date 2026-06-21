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
- DAV sync jobs (`sync_jobs` table, `internal/davsync`): per-job CalDAV/CardDAV sync between
  two servers (uni-/bidirectional, conflict policy, per-job interval, credentials), executed
  by a scheduled runner that persists sync state and the last-run status
  (`TIDYDAV_SYNC_TICK`, default 1m).
- Sync jobs API (`/api/sync`, session-authenticated): list/create/get/update/delete plus a
  manual `POST /api/sync/{id}/run`. Passwords are write-only (preserved across updates,
  masked in responses). DAV clients use a request timeout so a hung server can't stall the
  runner.
- Web UI for DAV sync: jobs list with last-run status and a "Run now" action, plus a
  create/edit form (type, direction, conflict policy, both endpoints with credentials,
  interval, enabled). Adds a Sync nav link.
- Password reset by email (`internal/mail`, `password_resets` table): an SMTP mailer
  (starttls/tls/none) and `/auth/reset/request` + `/auth/reset/confirm` endpoints. Tokens
  are hashed, expire in 1 hour and are pruned by the cleanup job; responses never reveal
  whether an email exists. The session payload exposes `mailEnabled`.
- Web UI for password reset: request and confirm pages, plus a "Forgot password?" link on
  the sign-in page (shown when SMTP is configured).
- Per-rule enable/disable: `pipeline.RuleConfig` gains an optional `enabled` flag; disabled
  rules are skipped when building the pipeline (omitted = enabled, backward compatible).
- Optional CalDAV sync date window (`window_start`/`window_end` on sync jobs, migration
  `009`): only events whose start falls in the range are synced; out-of-window items are
  neither propagated nor deleted (`dav.Options.WindowStart/WindowEnd`, `dav.ParseWindow`,
  `dav.EventInWindow`).
- Sync merge preview (`POST /api/sync/preview`, `internal/dav/preview.go`): fetches a
  date-windowed snapshot of both servers and returns each side plus the simulated merge for
  the chosen direction, without writing to either server.
- Redesigned calendar (ex "Feeds") editor: full-width source URL with an opt-in
  username/password toggle, per-rule enable switch and click-to-pick field chips for
  filter/dedup/strip, an explicitly-enabled Advanced section, per-channel notification
  toggles, and a sticky, week-navigable live preview panel.
- Redesigned sync editor: a direction toggle button between the Server A/B cards
  (→ / ← / ⇄), an "enable" switch that gates the interval with a live status line, an
  optional date range, and a three-column A/B/result merge preview.
- Internationalised UI (English + German, auto-detected from the browser) across the
  calendars and sync pages; the "Feeds" area is now labelled "Calendars"/"Kalender".
- Drag-and-drop rule reordering with keyboard-accessible up/down buttons; the stored
  order is the execution order, so the pipeline respects it.
- Per-source validation (`POST /api/feeds/source-check`): the editor shows a ✓/✕ next to
  each source URL, fetching it (with or without credentials) and reporting whether it
  parses as iCalendar, with the reason in a tooltip on failure.
- Footer with tool info, the running build version (`/health`) and the latest `main`
  commit from GitHub, flagging when an update is available.
- Faint diagonal "TidyDAV" watermark in the app background.
- Animated diagonal "TidyDAV" wallpaper: large alternating rows scroll in opposite
  45° directions with a muted accent on "DAV"; the marquee speed reacts to mouse
  activity (idles slow, speeds up while moving).
- Calendar editor: the preview panel is shown from the start (placeholder until loaded)
  and is wider so event names fit; the rule header keeps the type, "enabled" toggle and
  delete on one tidy row; drag handle plus up/down move buttons.
- Calendar editor "Advanced" split into independently toggleable options (cache and
  link password protection), each with a one-sentence, plain-language explanation.
- Calendar editor notifications grouped under "Trigger on" / "Notify via" headings with
  a generated plain-language summary of what will happen.

### Changed

- The Docker image now embeds the commit SHA as its version (branch builds), so the
  footer can compare the running build against the latest commit.

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
- The "Continue with SSO" button on the sign-in page is now shown only when OIDC is
  enabled (`oidcEnabled`), instead of always linking to an unconfigured login route.
- The home page now branches on authentication: signed-in users get links into the Feeds
  and Sync views instead of the placeholder "early scaffold" card and a sign-in button.
- The sync job editor only shows the conflict-resolution field for bidirectional jobs,
  where it actually applies.
- Native form controls (`<select>`, date pickers) now render in dark mode via
  `color-scheme: dark`.
- Credential fields (DAV usernames/passwords, basic-auth password, source passwords and
  the Gotify token) set `autocomplete` hints so browsers don't autofill stored logins.
- Copying an ICS URL now surfaces a "select the URL manually" message when the clipboard
  API is unavailable or fails, instead of silently doing nothing.
- The sync jobs list now surfaces each job's last-run time and a colored status badge.
- The feeds list shows a hint when a feed requires HTTP Basic Auth in the calendar client.
- Removed dead code: the unused user-level secret-id lookup (`UserBySecret` /
  `UserBySecretHash`) and unused `internal/ics` (`FieldURL`, `FieldAttendee`, `Start`,
  `End`) and `internal/proxy` (`Source.String`) symbols.
- Added a toast system: feed/sync create, save, delete and run actions now show a brief
  success (or error) confirmation. The rule editor shows per-rule descriptions, order
  numbers and an "apply top to bottom" hint.
- The calendar preview no longer fails to render for feeds whose events have no `UID`
  (e.g. municipal waste calendars): the preview list is keyed by index instead of
  `uid + start`, which previously collided across identical undated events.

### Known limitations

- VTODO/tasks are not a first-class sync kind — CalDAV jobs still carry VTODOs in the collection.
- No CTag/sync-token fast-path — each run does a full PROPFIND.
- Cross-source merge-dedup is UID-only — use a dedup rule for content-level deduplication.
- Sync jobs don't share credentials — each job stores its own.
- Only filter and rename rules can trigger notifications.

[Unreleased]: https://github.com/Norrodar/TidyDAV/commits/main
