/**
 * WebSocket URL helper — computes the correct ws:// or wss:// URL
 * based on the current page protocol and host.
 */
export function getWebSocketURL(): string {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  return `${proto}//${window.location.host}/ws`;
}
