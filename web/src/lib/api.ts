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
}

export interface SessionResponse {
  authenticated: boolean;
  user: SessionUser | null;
  accessMode: AccessMode;
  oidcEnabled: boolean;
  registrationEnabled: boolean;
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
  logout: () => request<void>('/auth/logout', { method: 'POST' }),

  feeds: {
    list: () => request<Feed[]>('/api/feeds'),
    get: (id: string) => request<Feed>(`/api/feeds/${id}`),
    create: (input: FeedInput) => request<Feed>('/api/feeds', jsonBody('POST', input)),
    update: (id: string, input: FeedInput) => request<Feed>(`/api/feeds/${id}`, jsonBody('PUT', input)),
    remove: (id: string) => request<void>(`/api/feeds/${id}`, { method: 'DELETE' }),
    preview: (input: FeedInput) => request<PreviewResult>('/api/feeds/preview', jsonBody('POST', input))
  }
};
