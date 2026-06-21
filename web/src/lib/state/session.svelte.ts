import { api, type AccessMode, type SessionResponse, type SessionUser } from '$lib/api';

/**
 * Cross-route session singleton. This is the one place legacy-store-like global
 * state is allowed (see CLAUDE.md); it is backed by Svelte 5 runes.
 */
class SessionState {
  user = $state<SessionUser | null>(null);
  accessMode = $state<AccessMode>('auth');
  oidcEnabled = $state(false);
  oidcDisplayName = $state('SSO');
  oidcOnly = $state(false);
  registrationEnabled = $state(true);
  mailEnabled = $state(false);
  accentColor = $state<string | undefined>(undefined);
  backgroundAnimation = $state(true);
  loading = $state(true);
  error = $state<string | null>(null);

  readonly authenticated = $derived(this.user !== null);

  /** Apply a session payload from the backend. */
  apply(s: SessionResponse): void {
    this.user = s.user;
    this.accessMode = s.accessMode;
    this.oidcEnabled = s.oidcEnabled;
    this.oidcDisplayName = s.oidcDisplayName ?? 'SSO';
    this.oidcOnly = s.oidcOnly ?? false;
    this.registrationEnabled = s.registrationEnabled;
    this.mailEnabled = s.mailEnabled;
    this.accentColor = s.accentColor;
    this.backgroundAnimation = s.backgroundAnimation ?? true;
  }

  /** Fetch the current session from the backend. */
  async refresh(): Promise<void> {
    this.loading = true;
    this.error = null;
    try {
      this.apply(await api.session());
    } catch (e) {
      this.error = e instanceof Error ? e.message : 'unknown error';
    } finally {
      this.loading = false;
    }
  }
}

export const session = new SessionState();
