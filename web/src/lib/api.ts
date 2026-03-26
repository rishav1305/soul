import { reportError } from './telemetry';
import { getToken, clearToken } from '../components/AuthGate';

const BASE = '';

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  try {
    const token = getToken();
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }
    const res = await fetch(`${BASE}${path}`, {
      headers,
      ...init,
    });
    if (res.status === 401) {
      clearToken();
      throw new Error('Unauthorized');
    }
    if (!res.ok) {
      const body = await res.json().catch(() => ({ error: res.statusText }));
      const err = new Error(body.error || `HTTP ${res.status}`) as Error & { status: number };
      err.status = res.status;
      reportError(`api.${init?.method || 'GET'}`, err);
      throw err;
    }
    if (res.status === 204) return undefined as T;
    return res.json();
  } catch (err) {
    if (err instanceof TypeError) {
      reportError('api.network', err);
    }
    throw err;
  }
}

/** Generate a v4 UUID, preferring crypto.randomUUID when available. */
export function uuid(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  // Fallback for non-secure contexts (HTTP, non-localhost)
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  bytes[6] = (bytes[6]! & 0x0f) | 0x40;
  bytes[8] = (bytes[8]! & 0x3f) | 0x80;
  return [...bytes]
    .map((b, i) =>
      [4, 6, 8, 10].includes(i) ? `-${b.toString(16).padStart(2, '0')}` : b.toString(16).padStart(2, '0'),
    )
    .join('');
}

/**
 * Drop-in fetch() replacement that injects the Authorization header.
 * Use this instead of bare fetch() for any /api/* call.
 */
export async function authFetch(input: RequestInfo | URL, init?: RequestInit): Promise<Response> {
  const token = getToken();
  const headers = new Headers(init?.headers);
  if (token && !headers.has('Authorization')) {
    headers.set('Authorization', `Bearer ${token}`);
  }
  const res = await fetch(input, { ...init, headers });
  if (res.status === 401) {
    clearToken();
  }
  return res;
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'POST', body: body ? JSON.stringify(body) : undefined }),
  patch: <T>(path: string, body: unknown) =>
    request<T>(path, { method: 'PATCH', body: JSON.stringify(body) }),
  delete: (path: string) => request<void>(path, { method: 'DELETE' }),
};
