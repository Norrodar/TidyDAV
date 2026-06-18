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

export const api = {
  health: () => request<HealthResponse>('/health'),
  session: () => request<SessionResponse>('/api/session'),
  login: (email: string, password: string) =>
    request<SessionResponse>('/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password })
    }),
  logout: () => request<void>('/auth/logout', { method: 'POST' })
};
