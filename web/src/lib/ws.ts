import { getToken } from '../components/AuthGate';

/**
 * WebSocket URL helper — computes the correct ws:// or wss:// URL
 * based on the current page protocol and host, with auth token.
 */
export function getWebSocketURL(): string {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const token = getToken();
  const params = token ? `?token=${encodeURIComponent(token)}` : '';
  return `${proto}//${window.location.host}/ws${params}`;
}
