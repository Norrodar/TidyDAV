// Minimal i18n: auto-detect browser language, fall back to English.
// Add keys here as needed — missing keys fall back to the English value, then the key.

type Translations = Record<string, string>;

const en: Translations = {
  // Auth / common
  sign_in: 'Sign in',
  sign_out: 'Sign out',
  welcome_back: 'Welcome back.',
  email: 'Email',
  password: 'Password',
  signing_in: 'Signing in…',
  or: 'or',
  no_account: 'No account?',
  create_one: 'Create one',
  forgot_password: 'Forgot password?',
  loading: 'Loading…',
  save_changes: 'Save changes',
  cancel: 'Cancel',
  saving: 'Saving…',
  edit: 'Edit',
  delete: 'Delete',
  optional: 'optional',
  unchanged: 'unchanged',

  // Nav
  nav_feeds: 'Calendars',
  nav_sync: 'Sync',
  nav_audit: 'Audit',

  // Landing / dashboard
  home_headline: 'Your calendars and contacts, tidied up.',
  home_feeds_title: 'Calendars',
  home_feeds_desc:
    'Fetch upstream calendars and run them through a configurable rule pipeline — filter, dedup, rename, timezone — then re-serve clean ICS endpoints.',
  home_sync_title: 'DAV Sync',
  home_sync_desc:
    'Mirror CalDAV and CardDAV between two servers, uni- or bidirectional, with automatic conflict resolution.',
  home_open: 'Open',
  home_signin_email: 'Sign in with email',

  // Calendars list
  calendars_title: 'Calendars',
  new_calendar: 'New calendar',
  no_calendars: 'No calendars yet.',
  create_first_calendar: 'Create your first calendar',
  copy_url: 'Copy URL',
  copied: 'Copied!',
  basic_auth_hint: 'Requires HTTP Basic Auth in your calendar client.',
  delete_calendar_confirm: 'Delete calendar “{name}”?',
  source_count: '{n} source(s)',
  rule_count: '{n} rule(s)',
  basic_auth_badge: 'basic auth',

  // Calendar editor
  edit_calendar: 'Edit calendar',
  new_calendar_heading: 'New calendar',
  name: 'Name',
  name_placeholder: 'Main Calendar',
  create_calendar: 'Create calendar',
  calendar_saved: 'Calendar saved',
  calendar_created: 'Calendar created',
  calendar_deleted: 'Calendar deleted',
  save_failed: 'Save failed',
  delete_failed: 'Delete failed',

  // Sources
  sources: 'Sources',
  add_source: 'Add source',
  source_url_placeholder: 'https://example.com/feed.ics',
  use_credentials: 'Use username & password',
  username: 'Username',

  // Rules
  rules: 'Rules',
  add_rule: 'Add rule',
  no_rules: 'No rules — the merged calendar is served as-is.',
  rules_apply_order: 'Rules apply top to bottom. Disabled rules are skipped.',
  rule_enabled: 'Enabled',
  remove: 'Remove',
  rule_filter: 'Filter',
  rule_dedup: 'Deduplicate',
  rule_rename: 'Rename',
  rule_strip: 'Strip fields',
  rule_timezone: 'Timezone',
  rule_expire: 'Expire',
  help_filter: 'Keep or drop events whose chosen fields match the pattern.',
  help_dedup: 'Remove duplicate events sharing the same key fields (default: summary + date).',
  help_rename: 'Rewrite a text field by replacing matches ($1 references a regex group).',
  help_strip: 'Delete the chosen fields from every event.',
  help_timezone: 'Convert event start/end times into the target timezone.',
  help_expire: 'Drop events that ended more than N days ago.',
  field_summary: 'Title',
  field_description: 'Description',
  field_location: 'Location',
  field_categories: 'Categories',
  field_dtstart: 'Date',
  custom_fields: 'Custom fields (comma-separated)',
  match_substring: 'substring',
  match_regex: 'regex',
  filter_blacklist: 'Remove matches',
  filter_whitelist: 'Keep matches',
  pattern: 'Pattern',
  replacement: 'Replacement ($1 in regex)',
  rename_field: 'Field',
  target_timezone: 'Target, e.g. Europe/Berlin',
  default_timezone: 'Default for floating times (optional)',
  drop_older_than: 'Drop events older than',
  days: 'days',
  fields_to_match: 'Fields to match',
  key_fields: 'Key fields',
  fields_to_strip: 'Fields to remove',

  // Advanced
  advanced: 'Advanced',
  enable_advanced: 'Enable advanced settings',
  cache_ttl: 'Cache TTL (seconds)',
  basic_auth_user: 'Basic auth user',
  basic_auth_password: 'Basic auth password',
  basic_auth_disable_hint: 'leave empty to disable',

  // Notifications
  notifications: 'Notifications',
  notifications_desc:
    'Fire a notification when matching rules trigger. Checked on a schedule, and each matched event notifies only once.',
  trigger_on: 'Trigger on:',
  enable_webhook: 'Webhook',
  enable_ntfy: 'ntfy',
  enable_gotify: 'Gotify',
  webhook_url: 'Webhook URL',
  ntfy_server: 'ntfy server',
  ntfy_topic: 'ntfy topic',
  gotify_server: 'Gotify server',
  gotify_token: 'Gotify token',

  // Preview
  preview: 'Preview',
  load_preview_week: 'Load preview week',
  apply_rules: 'Apply rules to preview',
  previewing: 'Previewing…',
  refresh_preview: 'Refresh',
  original: 'Original',
  transformed: 'Transformed',
  prev_week: 'Previous week',
  next_week: 'Next week',
  this_week: 'Week of {date}',
  no_events_week: 'No events this week.',
  hide_preview: 'Hide',
  show_preview: 'Show preview',
  preview_failed: 'Preview failed',

  // Sync list
  sync_title: 'DAV sync',
  new_sync_job: 'New sync job',
  no_sync_jobs: 'No sync jobs yet.',
  create_first_sync: 'Create your first sync job',
  run_now: 'Run now',
  running: 'Running…',
  last_run: 'Last run',
  never: 'never',
  disabled: 'disabled',
  delete_sync_confirm: 'Delete sync job “{name}”?',

  // Sync editor
  edit_sync_job: 'Edit sync job',
  new_sync_heading: 'New sync job',
  sync_name_placeholder: 'Calendar sync',
  create_job: 'Create job',
  sync_job_saved: 'Sync job saved',
  sync_job_created: 'Sync job created',
  sync_job_deleted: 'Sync job deleted',
  sync_complete: 'Sync complete',
  type: 'Type',
  caldav_label: 'CalDAV (calendars)',
  carddav_label: 'CardDAV (contacts)',
  conflict: 'Conflict resolution',
  newest_wins: 'Newest wins',
  server_a_wins: 'Server A wins',
  server_a: 'Server A',
  server_b: 'Server B',
  collection_url: 'Collection URL',
  flow_a_to_b: 'Server A → Server B',
  flow_b_to_a: 'Server B → Server A',
  flow_bidirectional: 'Bidirectional',
  flow_hint: 'Click to change sync direction',
  enable_recurring: 'Run on a schedule',
  interval_minutes: 'Interval (minutes)',
  status_one_time: 'One-time sync (manual run only).',
  status_every: 'Synced every {n} minutes.',
  limit_date_range: 'Limit by date range (CalDAV only)',
  date_from: 'From',
  date_to: 'To',
  result: 'Result',
};

const de: Translations = {
  // Auth / common
  sign_in: 'Anmelden',
  sign_out: 'Abmelden',
  welcome_back: 'Willkommen zurück.',
  email: 'E-Mail',
  password: 'Passwort',
  signing_in: 'Anmeldung läuft…',
  or: 'oder',
  no_account: 'Noch kein Konto?',
  create_one: 'Jetzt erstellen',
  forgot_password: 'Passwort vergessen?',
  loading: 'Lädt…',
  save_changes: 'Änderungen speichern',
  cancel: 'Abbrechen',
  saving: 'Speichern…',
  edit: 'Bearbeiten',
  delete: 'Löschen',
  optional: 'optional',
  unchanged: 'unverändert',

  // Nav
  nav_feeds: 'Kalender',
  nav_sync: 'Sync',
  nav_audit: 'Audit',

  // Landing / dashboard
  home_headline: 'Deine Kalender und Kontakte, aufgeräumt.',
  home_feeds_title: 'Kalender',
  home_feeds_desc:
    'Upstream-Kalender abrufen und durch eine konfigurierbare Regel-Pipeline leiten – filtern, deduplizieren, umbenennen, Zeitzone – und sauber als ICS-Endpunkt bereitstellen.',
  home_sync_title: 'DAV-Sync',
  home_sync_desc:
    'CalDAV und CardDAV zwischen zwei Servern spiegeln, ein- oder bidirektional, mit automatischer Konfliktauflösung.',
  home_open: 'Öffnen',
  home_signin_email: 'Mit E-Mail anmelden',

  // Calendars list
  calendars_title: 'Kalender',
  new_calendar: 'Neuer Kalender',
  no_calendars: 'Noch keine Kalender.',
  create_first_calendar: 'Ersten Kalender erstellen',
  copy_url: 'URL kopieren',
  copied: 'Kopiert!',
  basic_auth_hint: 'Erfordert HTTP-Basic-Auth im Kalender-Client.',
  delete_calendar_confirm: 'Kalender „{name}" löschen?',
  source_count: '{n} Quelle(n)',
  rule_count: '{n} Regel(n)',
  basic_auth_badge: 'Basic Auth',

  // Calendar editor
  edit_calendar: 'Kalender bearbeiten',
  new_calendar_heading: 'Neuer Kalender',
  name: 'Name',
  name_placeholder: 'Hauptkalender',
  create_calendar: 'Kalender erstellen',
  calendar_saved: 'Kalender gespeichert',
  calendar_created: 'Kalender erstellt',
  calendar_deleted: 'Kalender gelöscht',
  save_failed: 'Speichern fehlgeschlagen',
  delete_failed: 'Löschen fehlgeschlagen',

  // Sources
  sources: 'Quellen',
  add_source: 'Quelle hinzufügen',
  source_url_placeholder: 'https://example.com/feed.ics',
  use_credentials: 'Benutzername & Passwort verwenden',
  username: 'Benutzername',

  // Rules
  rules: 'Regeln',
  add_rule: 'Regel hinzufügen',
  no_rules: 'Keine Regeln – der zusammengeführte Kalender wird unverändert ausgeliefert.',
  rules_apply_order: 'Regeln werden von oben nach unten angewandt. Deaktivierte werden übersprungen.',
  rule_enabled: 'Aktiv',
  remove: 'Entfernen',
  rule_filter: 'Filtern',
  rule_dedup: 'Duplikate entfernen',
  rule_rename: 'Umbenennen',
  rule_strip: 'Felder entfernen',
  rule_timezone: 'Zeitzone',
  rule_expire: 'Verfallen',
  help_filter: 'Events behalten oder verwerfen, deren gewählte Felder zum Muster passen.',
  help_dedup: 'Doppelte Events mit gleichen Schlüsselfeldern entfernen (Standard: Titel + Datum).',
  help_rename: 'Ein Textfeld umschreiben, indem Treffer ersetzt werden ($1 = Regex-Gruppe).',
  help_strip: 'Die gewählten Felder aus jedem Event löschen.',
  help_timezone: 'Start-/Endzeiten in die Zielzeitzone umrechnen.',
  help_expire: 'Events verwerfen, die vor mehr als N Tagen endeten.',
  field_summary: 'Titel',
  field_description: 'Beschreibung',
  field_location: 'Ort',
  field_categories: 'Kategorien',
  field_dtstart: 'Datum',
  custom_fields: 'Eigene Felder (kommagetrennt)',
  match_substring: 'Teilstring',
  match_regex: 'Regex',
  filter_blacklist: 'Treffer entfernen',
  filter_whitelist: 'Treffer behalten',
  pattern: 'Muster',
  replacement: 'Ersetzung ($1 in Regex)',
  rename_field: 'Feld',
  target_timezone: 'Ziel, z. B. Europe/Berlin',
  default_timezone: 'Standard für schwebende Zeiten (optional)',
  drop_older_than: 'Events verwerfen älter als',
  days: 'Tage',
  fields_to_match: 'Zu prüfende Felder',
  key_fields: 'Schlüsselfelder',
  fields_to_strip: 'Zu entfernende Felder',

  // Advanced
  advanced: 'Erweitert',
  enable_advanced: 'Erweiterte Einstellungen aktivieren',
  cache_ttl: 'Cache-TTL (Sekunden)',
  basic_auth_user: 'Basic-Auth-Benutzer',
  basic_auth_password: 'Basic-Auth-Passwort',
  basic_auth_disable_hint: 'leer lassen zum Deaktivieren',

  // Notifications
  notifications: 'Benachrichtigungen',
  notifications_desc:
    'Benachrichtigt, wenn passende Regeln auslösen. Geplant geprüft; jedes Event meldet nur einmal.',
  trigger_on: 'Auslösen bei:',
  enable_webhook: 'Webhook',
  enable_ntfy: 'ntfy',
  enable_gotify: 'Gotify',
  webhook_url: 'Webhook-URL',
  ntfy_server: 'ntfy-Server',
  ntfy_topic: 'ntfy-Topic',
  gotify_server: 'Gotify-Server',
  gotify_token: 'Gotify-Token',

  // Preview
  preview: 'Vorschau',
  load_preview_week: 'Vorschauwoche laden',
  apply_rules: 'Regeln auf Vorschau anwenden',
  previewing: 'Vorschau lädt…',
  refresh_preview: 'Aktualisieren',
  original: 'Original',
  transformed: 'Transformiert',
  prev_week: 'Vorherige Woche',
  next_week: 'Nächste Woche',
  this_week: 'Woche ab {date}',
  no_events_week: 'Keine Events in dieser Woche.',
  hide_preview: 'Ausblenden',
  show_preview: 'Vorschau anzeigen',
  preview_failed: 'Vorschau fehlgeschlagen',

  // Sync list
  sync_title: 'DAV-Sync',
  new_sync_job: 'Neuer Sync-Job',
  no_sync_jobs: 'Noch keine Sync-Jobs.',
  create_first_sync: 'Ersten Sync-Job erstellen',
  run_now: 'Jetzt ausführen',
  running: 'Läuft…',
  last_run: 'Letzter Lauf',
  never: 'nie',
  disabled: 'deaktiviert',
  delete_sync_confirm: 'Sync-Job „{name}" löschen?',

  // Sync editor
  edit_sync_job: 'Sync-Job bearbeiten',
  new_sync_heading: 'Neuer Sync-Job',
  sync_name_placeholder: 'Kalender-Sync',
  create_job: 'Job erstellen',
  sync_job_saved: 'Sync-Job gespeichert',
  sync_job_created: 'Sync-Job erstellt',
  sync_job_deleted: 'Sync-Job gelöscht',
  sync_complete: 'Synchronisierung abgeschlossen',
  type: 'Typ',
  caldav_label: 'CalDAV (Kalender)',
  carddav_label: 'CardDAV (Kontakte)',
  conflict: 'Konfliktauflösung',
  newest_wins: 'Neuestes gewinnt',
  server_a_wins: 'Server A gewinnt',
  server_a: 'Server A',
  server_b: 'Server B',
  collection_url: 'Collection-URL',
  flow_a_to_b: 'Server A → Server B',
  flow_b_to_a: 'Server B → Server A',
  flow_bidirectional: 'Bidirektional',
  flow_hint: 'Klicken, um die Sync-Richtung zu ändern',
  enable_recurring: 'Geplant ausführen',
  interval_minutes: 'Intervall (Minuten)',
  status_one_time: 'Einmalige Synchronisierung (nur manuell).',
  status_every: 'Synchronisierung alle {n} Minuten.',
  limit_date_range: 'Auf Datumsbereich begrenzen (nur CalDAV)',
  date_from: 'Von',
  date_to: 'Bis',
  result: 'Ergebnis',
};

const map: Record<string, Translations> = { en, de };

function detectLang(): string {
  if (typeof navigator === 'undefined') return 'en';
  const l = navigator.language ?? 'en';
  const prefix = l.split('-')[0].toLowerCase();
  return prefix in map ? prefix : 'en';
}

export const lang = detectLang();
const dict = map[lang] ?? en;

export function t(key: string): string {
  return dict[key] ?? en[key] ?? key;
}

/** Like t(), but interpolates {token} placeholders from params. */
export function tf(key: string, params: Record<string, string | number>): string {
  let out = t(key);
  for (const [k, v] of Object.entries(params)) {
    out = out.replaceAll(`{${k}}`, String(v));
  }
  return out;
}

/** Returns "Sign in with <name>" in the current language. */
export function tSignInWith(name: string): string {
  if (lang === 'de') return `Anmelden mit ${name}`;
  return `Sign in with ${name}`;
}
