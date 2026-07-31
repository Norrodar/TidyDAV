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
- Test notifications: a "Send test" button per channel (webhook / ntfy / Gotify) in the
  calendar editor fires one immediately (`POST /api/notify/test`); saved feeds reuse
  their stored Gotify token.
- Source health in the calendar list: a colored dot per calendar (green = every source
  fetched within 24h, orange = stale, gray = never) with per-source details in the
  tooltip, backed by a `lastFetchedAt` field on each source.
- Subscriber stats (migration `010`): every `/ics/<secret>` fetch records
  `last_served_at`/`serve_count`; the list shows when a calendar client last pulled the
  feed and how often.
- Reminder rule: attaches a `VALARM` to every event so the calendar app itself
  notifies ahead of time (presets from "at the event" to "2 days before"; 6 hours
  before an all-day event lands at 18:00 the evening before — the useful setting for
  bin-collection calendars). Any alarm the source carried is replaced, so no client
  fires twice.
- Quick preview on the list pages: every saved calendar gets a "Preview" button that
  expands an inline before/after view (`GET /api/feeds/{id}/preview`), and every saved
  sync job — calendars and contacts alike — gets one showing Server A, Server B and the
  simulated result (`GET /api/sync/{id}/preview`, optional `?week=`). Long lists are
  capped to the next upcoming entries with an "… and N more" note.
- Drag-and-drop rule reordering with keyboard-accessible up/down buttons; the stored
  order is the execution order, so the pipeline respects it.
- Per-source validation (`POST /api/feeds/source-check`): the editor shows a ✓/✕ next to
  each source URL, fetching it (with or without credentials) and reporting whether it
  parses as iCalendar, with the reason in a tooltip on failure.
- Footer with tool info, the running build version (`/health`) and the latest `main`
  commit from GitHub, flagging when an update is available.
- Faint diagonal "TidyDAV" watermark in the app background.
- Animated diagonal "TidyDAV" wallpaper: very large alternating rows drift slowly in
  opposite 45° directions with a muted accent on "DAV". Cards and panels use a strong
  frosted-glass backdrop blur that darkens the wallpaper behind them.
- Calendar editor: the preview panel is shown from the start (placeholder until loaded)
  and is wider so event names fit; the rule header keeps the type, "enabled" toggle and
  delete on one tidy row; drag handle plus up/down move buttons.
- Calendar editor "Advanced" split into independently toggleable options (cache and
  link password protection), each with a one-sentence, plain-language explanation.
- Calendar editor notifications grouped under "Trigger on" / "Notify via" headings with
  a generated plain-language summary of what will happen.
- Served feeds carry their identity: the configured name goes out as `NAME` (RFC 7986)
  and `X-WR-CALNAME`, RFC 5545 escaped, so a subscribed calendar shows its name instead
  of a URL. A blank name emits neither property. Feeds with a cache TTL also publish
  `REFRESH-INTERVAL;VALUE=DURATION` and `X-PUBLISHED-TTL`, so clients poll at the rate
  the cache actually refreshes instead of guessing; without a TTL neither is sent. The
  name and refresh interval survive even when every event is filtered away. The calendar
  list shows a "checks every …" badge for cached feeds, and the editor explains both
  fields.
- Sharing a calendar link, and taking it back. The ICS URL is now shown abbreviated
  (`https://dav.example.com/ics/a3f9…`) and only revealed on request, so the secret is
  not on screen during a screen share. Next to it sit a copy button and a `webcal://`
  link that subscribes in one click on iOS, macOS and Thunderbird. Because handing out
  that link is otherwise irreversible, the editor can replace it:
  `POST /api/feeds/{id}/rotate-secret` issues a new secret, the old URL stops resolving
  immediately, and the change is audited by calendar id — never with the secret itself.
  Rotating asks for confirmation first, since every existing subscriber has to be given
  the new link. Input fields grow to 16px on small screens so iOS stops zooming the page
  on focus.
- Alert when a calendar source stops updating, plus an all-clear once it recovers.
  Because the proxy keeps serving the last good copy on every upstream error, a dead
  source was previously invisible — the calendar silently kept showing last year's
  dates. A per-calendar threshold ("warn after N hours without a successful fetch",
  a third trigger next to filter and rename) sends exactly one warning over the
  configured channels and exactly one all-clear when the source works again; an
  undeliverable message is retried on the next run. Source URLs are redacted before
  sending, and the de-duplication ledger stores a hash instead of the URL, so no
  token or password reaches a notification target or the database in clear text.
  Checks run on the notifier schedule (`TIDYDAV_NOTIFY_INTERVAL`, 15 minutes by
  default), which bounds how quickly the warning can arrive. The health dot in the
  calendar list now uses the same threshold instead of a fixed 24 hours.
- Conditional GET on `/ics/<secret>`: every served calendar carries a strong `ETag`
  derived from the rendered body, and a request repeating it in `If-None-Match` is
  answered with `304 Not Modified` and no body. Calendar clients poll every few
  minutes, so this removes the bulk of the traffic an unchanged calendar used to cost —
  worthwhile once the feed has a TTL > 0, since an uncached feed still re-fetches its
  upstream on every request. There is deliberately no render cache behind it: the tag
  describes the body just rendered, so a rule change invalidates it on the very next
  request and a stale calendar cannot be served. Comparison follows RFC 9110 §13.1.2,
  including the weak form a gzipping reverse proxy hands back. `304` reaches
  Basic-Auth-protected calendars only after successful authentication, and it still
  counts as a fetch in the subscriber statistics.
- Reset the sync state of a job (`POST /api/sync/{id}/reset`, plus a button in the
  jobs list). A run stopped by the vanish guard stays stopped by design — but the only
  way out used to be deleting the job and typing its configuration in again. A job in
  that state now reports `blocked:` instead of `error:`, explains itself in the status
  tooltip and offers the reset, which clears only the remembered state: the next run
  treats both sides as new, deletes nothing, and a one-way job refills its destination
  from the source. URLs, credentials and schedule survive.

### Changed

- The Docker image now embeds the commit SHA as its version (branch builds), so the
  footer can compare the running build against the latest commit.
- `TIDYDAV_BACKGROUND_ANIMATION` (default `true`) toggles the animated wallpaper;
  `TIDYDAV_OIDC_POST_LOGOUT_REDIRECT_URI` overrides the OIDC post-logout redirect.
- Wallpaper text is bolder, much larger, more widely spaced and drifts more slowly;
  cards and dashboard tiles darken the background less.

### Fixed

- CI publishes container images again. The lint job pinned golangci-lint v1.64.8, a
  release built against an older Go that cannot type-check a `go 1.25` module, so the
  step failed on every commit since it was introduced — and because the image build
  depends on it, nine commits' worth of fixes never reached ghcr.io. The linter is now
  on v2 (`golangci-lint-action@v8`, v2.12.2) with a migrated config, and the findings it
  surfaced are fixed rather than silenced: a preview loop that always broke on its first
  iteration reads as the single lookup it is, `max` and `cap` no longer shadow builtins,
  the SQLite driver's blank import states why it exists, unused test parameters and one
  dead test helper are gone, and imports are grouped as the config always demanded. Two
  checks stay off with the reason recorded in `.golangci.yml`: `govet.shadow`, which only
  ever flagged the idiomatic `if err := f(); err != nil`, and `misspell`'s locale, since
  the prose is British while identifiers and fixed names like `STATUS:CANCELLED` are not.
- One-way sync is now a real mirror. It never listed the destination, so an item
  deleted or edited there stayed that way forever: the source ETag was unchanged, the
  fast path skipped the item, and the state kept claiming the copy existed. Each run
  now lists the destination too and repairs what it finds — deleted copies are
  recreated, edited copies are overwritten (the source wins for one-way jobs).
  Verification compares body fingerprints (`srcHash`/`dstHash` in the sync state,
  learned from the destination's own rendering after a write) instead of ETags, so a
  server that reissues ETags on every listing does not trigger an endless rewrite: a
  run with nothing to do still writes nothing. The vanish guard that refuses to act on
  a suddenly empty collection now covers the destination side as well, sync state
  written by earlier versions is adopted without a mass re-upload, destination items
  TidyDAV does not manage stay untouched, and the date window keeps protecting
  out-of-window items from both restore and deletion. Bidirectional jobs are
  unchanged.
  A destination object that cannot be read — one corrupt or permission-protected
  resource on the far side — is skipped and reported at the end of the run instead of
  aborting it: it must not stop the items behind it or the deletion pass. The
  deletion counter reports what TidyDAV actually removed, so a copy the user had
  already deleted is not counted again. Note that a destination which hands out fresh
  ETags on every listing, or none at all, has to be read in full on every run; that is
  the price of not rewriting the whole collection instead.
- **Security:** the upstream cache was keyed by URL alone, so a copy fetched with
  credentials could be served to — or overwritten by — a request that supplied none.
  On a multi-user instance that exposed another user's private calendar to anyone who
  knew the URL. Entries are now keyed by URL *and* a hash of the credentials
  (migration `011`, which drops the old unattributable rows).
- **Security:** `TIDYDAV_ALLOW_PRIVATE_TARGETS=false` only hardened the feed proxy.
  DAV endpoints (sync and its preview) and notification targets could still reach
  internal hosts and reflect the result back. All outbound clients now share one
  policy (`internal/outbound`), which also covers the CGNAT range.
- **Security:** a Gotify delivery failure logged the full URL — including the token —
  because Go's transport errors embed the request URL. Errors are now redacted.
- **Security:** every `/ics/<secret>` request logged the secret, the feed's only
  credential, on every calendar-client poll. The path is now masked in request logs.
- **Security:** calendar source URLs were logged verbatim whenever a source was
  unreachable or unparsable. A "secret address" link carries its token in the query
  string, and a URL entered as `https://user:pass@host/cal.ics` carries the password —
  both ended up in the log in clear text, as did the URL Go embeds in transport errors.
  Source URLs are now redacted (`internal/outbound`) wherever they are logged or
  returned, using the helper the notification path already had.
- Expiry dropped recurring events whose series started long ago but still recurs —
  weekly meetings and yearly birthdays vanished from any calendar with an expire rule.
  Recurrence is now evaluated (`RRULE` `UNTIL`, `RDATE`), and series without a knowable
  end are always kept.
- Merging discarded recurrence overrides: a moved or cancelled instance shares the
  series UID, so clients showed the instance at its old time and resurrected cancelled
  ones. The dedup identity now spans `UID` + `RECURRENCE-ID`.
- A filter rule with an empty pattern matched every event, silently blanking the whole
  calendar; it is rejected now, and the editor validates every rule before sending.
- One event in a timezone Go does not know (Outlook's "W. Europe Standard Time") made
  the timezone rule fail the entire render, so `/ics` answered 502. Such values are now
  left untouched and the rest of the feed is served.
- Sync refuses to run when one side suddenly lists nothing although the previous run
  saw a full collection — a moved collection URL used to wipe the other side.
- A manually triggered sync ran in the request context: closing the browser tab
  cancelled it mid-write, leaving items created but no state recorded (duplicates on
  the next run). Runs now outlive the request, and a job can no longer be executed by
  the scheduler and by hand at the same time.
- Notifications were marked as sent before delivery, so an unreachable target lost them
  permanently; and a long-lived event was announced again every 30 days when its ledger
  entry aged out. Delivery is now confirmed first and repeat sightings refresh the entry.
- The calendar editor could not switch source credentials off or clear a stored Gotify
  token, and editing an authenticated source's URL silently dropped its password.
- "Keep a cached copy" did not actually disable caching, and its checkbox did not
  survive a round-trip when the interval was exactly 15 minutes.
- Enabling password protection without a username saved silently and left the ICS link
  public; both sides now reject it.
- The generated `VTIMEZONE` scan was unbounded: a single event dated far in the future
  cost hundreds of thousands of iterations per request. The window is clamped.
- Cached upstream bodies (up to 25 MiB each) of deleted feeds were never pruned.
- Session probes that fail because the backend is unreachable no longer look like being
  signed out; the UI says so instead of bouncing to the login page.
- Register, password-reset and audit pages were untranslated, and a failed sign-in read
  "Anmelden failed" in German. `npm run lint` passes again.
- Feeds whose upstream events lack `UID`/`DTSTAMP` (e.g. municipal waste
  calendars) failed to serve with a 502 ("feed could not be rendered") because
  go-ical refuses to encode such events. The merge step now synthesizes
  deterministic UIDs (stable across fetches, distinct for exact duplicates) and
  derives a DTSTAMP from DTSTART.
- Event titles containing unescaped commas (RFC-invalid but common, e.g.
  "Braune Tonne, Bioabfall") were truncated at the comma in the preview and in
  rule matching. `ics.Text` now treats single-value text fields as one value
  instead of a comma-separated list, while still resolving proper escapes.
- Served calendars now include a `VTIMEZONE` definition for every `TZID` their
  events reference (RFC 5545 requirement; strict clients misread local times
  otherwise): upstream definitions are passed through, and zones introduced by
  the timezone rule are generated from the Go tzdata (`ics.VTimezone`).
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
