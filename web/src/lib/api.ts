// Single typed client for the TidyDAV backend. All UI calls go through here.

export interface HealthResponse {
  status: string;
  version: string;
}

export type AccessMode = 'public' | 'auth' | 'both';

export interface SessionUser {
  id: string;
  email: string | null;
  kind: 'oidc' | 'password' | 'secret';
  isAdmin: boolean;
  avatarUrl: string;
}

export interface SessionResponse {
  authenticated: boolean;
  user: SessionUser | null;
  accessMode: AccessMode;
  oidcEnabled: boolean;
  oidcDisplayName: string;
  oidcOnly: boolean;
  registrationEnabled: boolean;
  mailEnabled: boolean;
  accentColor?: string;
}

/** Error thrown for non-2xx API responses, carrying the HTTP status. */
export class ApiError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

/** Build a query string (incl. leading `?`) from params, skipping empty values. */
export function buildQuery(
  params: Record<string, string | number | boolean | undefined | null>
): string {
  const usp = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === '') continue;
    usp.set(key, String(value));
  }
  const query = usp.toString();
  return query ? `?${query}` : '';
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    credentials: 'same-origin',
    headers: { Accept: 'application/json', ...(init?.headers ?? {}) },
    ...init
  });

  if (!res.ok) {
    let message = res.statusText;
    try {
      const body = (await res.json()) as { error?: string };
      if (body && typeof body.error === 'string') message = body.error;
    } catch {
      // Non-JSON error body — fall back to the status text.
    }
    throw new ApiError(res.status, message);
  }

  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

// ── Feeds ────────────────────────────────────────────────────────────────────

export interface FeedSource {
  url: string;
  username?: string;
  password?: string; // write-only
  hasPassword?: boolean; // read-only
}

export type RuleType = 'filter' | 'dedup' | 'rename' | 'strip' | 'timezone' | 'expire';

export interface RuleConfig {
  type: RuleType;
  matchMode?: 'substring' | 'regex';
  pattern?: string;
  filterMode?: 'blacklist' | 'whitelist';
  fields?: string[];
  field?: string;
  replacement?: string;
  keyFields?: string[];
  target?: string;
  defaultTz?: string;
  days?: number;
}

export interface NotificationsInput {
  webhookUrl?: string;
  ntfyServer?: string;
  ntfyTopic?: string;
  gotifyServer?: string;
  gotifyToken?: string; // write-only
  triggers?: string[];
}

export interface NotificationsResponse {
  webhookUrl: string;
  ntfyServer: string;
  ntfyTopic: string;
  gotifyServer: string;
  gotifyTokenSet: boolean;
  triggers: string[];
}

export interface Feed {
  id: string;
  name: string;
  secret: string;
  icsUrl: string;
  sources: FeedSource[];
  rules: RuleConfig[];
  ttlSeconds: number;
  basicAuthUser: string;
  basicAuthEnabled: boolean;
  notifications: NotificationsResponse;
  createdAt: string;
  updatedAt: string;
}

export interface FeedInput {
  name: string;
  sources: FeedSource[];
  rules: RuleConfig[];
  ttlSeconds: number;
  basicAuthUser: string;
  basicAuthPassword?: string;
  notifications?: NotificationsInput;
}

export interface EventSummary {
  uid: string;
  summary: string;
  start: string;
  location: string;
  description: string;
}

export interface PreviewResult {
  original: EventSummary[];
  transformed: EventSummary[];
}

export interface AuditEntry {
  id: number;
  userEmail: string;
  action: string;
  target: string;
  detail: string;
  createdAt: string;
}

export type SyncKind = 'caldav' | 'carddav';
export type SyncDirection = 'a-to-b' | 'b-to-a' | 'bidirectional';
export type SyncConflict = 'newest-wins' | 'source-wins';

export interface SyncJob {
  id: string;
  name: string;
  kind: SyncKind;
  direction: SyncDirection;
  conflict: SyncConflict;
  aUrl: string;
  aUsername: string;
  aPasswordSet: boolean;
  bUrl: string;
  bUsername: string;
  bPasswordSet: boolean;
  intervalSeconds: number;
  enabled: boolean;
  lastRunAt: string;
  lastStatus: string;
  createdAt: string;
  updatedAt: string;
}

export interface SyncJobInput {
  name: string;
  kind: SyncKind;
  direction: SyncDirection;
  conflict: SyncConflict;
  aUrl: string;
  aUsername?: string;
  aPassword?: string;
  bUrl: string;
  bUsername?: string;
  bPassword?: string;
  intervalSeconds: number;
  enabled: boolean;
}

function jsonBody(method: string, body: unknown): RequestInit {
  return {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body)
  };
}

export const api = {
  health: () => request<HealthResponse>('/health'),
  session: () => request<SessionResponse>('/api/session'),
  login: (email: string, password: string) =>
    request<SessionResponse>('/auth/login', jsonBody('POST', { email, password })),
  register: (email: string, password: string) =>
    request<SessionResponse>('/auth/register', jsonBody('POST', { email, password })),
  logout: () => request<void>('/auth/logout', { method: 'POST' }),
  requestPasswordReset: (email: string) =>
    request<void>('/auth/reset/request', jsonBody('POST', { email })),
  confirmPasswordReset: (token: string, password: string) =>
    request<void>('/auth/reset/confirm', jsonBody('POST', { token, password })),

  feeds: {
    list: () => request<Feed[]>('/api/feeds'),
    get: (id: string) => request<Feed>(`/api/feeds/${id}`),
    create: (input: FeedInput) => request<Feed>('/api/feeds', jsonBody('POST', input)),
    update: (id: string, input: FeedInput) => request<Feed>(`/api/feeds/${id}`, jsonBody('PUT', input)),
    remove: (id: string) => request<void>(`/api/feeds/${id}`, { method: 'DELETE' }),
    preview: (input: FeedInput, id?: string) =>
      request<PreviewResult>('/api/feeds/preview', jsonBody('POST', id ? { ...input, id } : input))
  },

  audit: {
    list: () => request<AuditEntry[]>('/api/audit')
  },

  sync: {
    list: () => request<SyncJob[]>('/api/sync'),
    get: (id: string) => request<SyncJob>(`/api/sync/${id}`),
    create: (input: SyncJobInput) => request<SyncJob>('/api/sync', jsonBody('POST', input)),
    update: (id: string, input: SyncJobInput) => request<SyncJob>(`/api/sync/${id}`, jsonBody('PUT', input)),
    remove: (id: string) => request<void>(`/api/sync/${id}`, { method: 'DELETE' }),
    run: (id: string) => request<SyncJob>(`/api/sync/${id}/run`, { method: 'POST' })
  }
};
