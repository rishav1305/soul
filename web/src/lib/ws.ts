import { getToken } from '../components/AuthGate';

/** Base WebSocket URL without auth parameters. */
function getWebSocketBaseURL(): string {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  return `${proto}//${window.location.host}/ws`;
}

/**
 * Fetches a short-lived one-time WS ticket from the server.
 * Returns null if auth is not configured or the request fails.
 * Callers should fall back to the raw token on null.
 */
export async function fetchWSTicket(): Promise<string | null> {
  const token = getToken();
  if (!token) return null;
  try {
    const resp = await fetch('/api/ws-ticket', {
      headers: { Authorization: `Bearer ${token}` },
    });
    if (!resp.ok) return null;
    const data: { ticket?: string } = await resp.json();
    return data.ticket ?? null;
  } catch {
    return null;
  }
}

/**
 * Builds the WebSocket URL with a one-time ticket (preferred) or raw token
 * (fallback). Call fetchWSTicket() first to obtain a ticket.
 */
export function getWebSocketURL(ticket?: string | null): string {
  const base = getWebSocketBaseURL();
  if (ticket) return `${base}?ticket=${encodeURIComponent(ticket)}`;
  const token = getToken();
  return token ? `${base}?token=${encodeURIComponent(token)}` : base;
}
