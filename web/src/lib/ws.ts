import { getToken } from '../components/AuthGate';

/** Base WebSocket URL without auth parameters. */
function getWebSocketBaseURL(): string {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  return `${proto}//${window.location.host}/ws`;
}

/**
 * Fetches a short-lived one-time WS ticket from the server.
 * Returns {ticket, status} where:
 *   - ticket: the ticket string (null if unavailable or auth not configured)
 *   - status: HTTP status code (0 = network error/timeout, 200 = ok, 401 = auth failure, etc.)
 * Times out after 5 seconds to prevent stalling the WS connection.
 */
export async function fetchWSTicket(): Promise<{ ticket: string | null; status: number }> {
  const token = getToken();
  if (!token) return { ticket: null, status: 0 };

  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 5000);

  try {
    const resp = await fetch('/api/ws-ticket', {
      headers: { Authorization: `Bearer ${token}` },
      signal: controller.signal,
    });
    clearTimeout(timeout);
    if (!resp.ok) return { ticket: null, status: resp.status };
    const data: { ticket?: string } = await resp.json();
    return { ticket: data.ticket ?? null, status: 200 };
  } catch {
    clearTimeout(timeout);
    return { ticket: null, status: 0 };
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
