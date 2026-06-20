// Minimal i18n: auto-detect browser language, fall back to English.
// Add keys here as needed — missing keys fall back to the key itself.

type Translations = Record<string, string>;

const en: Translations = {
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

  nav_feeds: 'Feeds',
  nav_sync: 'Sync',
  nav_audit: 'Audit',

  home_headline: 'Your calendars and contacts, tidied up.',
  home_feeds_title: 'ICS Feeds',
  home_feeds_desc:
    'Fetch upstream calendars and run them through a configurable rule pipeline — filter, dedup, rename, timezone — then re-serve clean ICS endpoints.',
  home_sync_title: 'DAV Sync',
  home_sync_desc:
    'Mirror CalDAV and CardDAV between two servers, uni- or bidirectional, with automatic conflict resolution.',
  home_open: 'Open',
  home_signin_email: 'Sign in with email',
};

const de: Translations = {
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

  nav_feeds: 'Feeds',
  nav_sync: 'Sync',
  nav_audit: 'Audit',

  home_headline: 'Deine Kalender und Kontakte, aufgeräumt.',
  home_feeds_title: 'ICS-Feeds',
  home_feeds_desc:
    'Upstream-Kalender abrufen und durch eine konfigurierbare Regel-Pipeline leiten – filtern, deduplizieren, umbenennen, Zeitzone – und sauber als ICS-Endpunkt bereitstellen.',
  home_sync_title: 'DAV-Sync',
  home_sync_desc:
    'CalDAV und CardDAV zwischen zwei Servern spiegeln, ein- oder bidirektional, mit automatischer Konfliktauflösung.',
  home_open: 'Öffnen',
  home_signin_email: 'Mit E-Mail anmelden',
};

const map: Record<string, Translations> = { en, de };

function detectLang(): string {
  if (typeof navigator === 'undefined') return 'en';
  const lang = navigator.language ?? 'en';
  const prefix = lang.split('-')[0].toLowerCase();
  return prefix in map ? prefix : 'en';
}

const lang = detectLang();
const dict = map[lang] ?? en;

export function t(key: string): string {
  return dict[key] ?? en[key] ?? key;
}

/** Returns "Sign in with <name>" in the current language. */
export function tSignInWith(name: string): string {
  if (lang === 'de') return `Anmelden mit ${name}`;
  return `Sign in with ${name}`;
}
