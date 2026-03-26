/**
 * useWebSocketContext — context-based WebSocket hook for the AppShell.
 * Ported from soul-v1. Provides a single shared WS connection across all
 * AppShell components via React context.
 *
 * Distinct from hooks/useWebSocket.ts (v2 per-hook ticket-auth pattern).
 *
 * Usage:
 *   1. Wrap app root with <WebSocketContext.Provider value={useWebSocketProvider()}>
 *   2. Any component calls useWebSocket() — reads from the shared context
 */
import { createContext, useContext, useEffect, useRef, useState, useCallback } from 'react';
import { WSClient } from '../lib/ws-client.ts';
import type { WSMessage } from '../lib/types.ts';

function getWSUrl(): string {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  return `${proto}//${window.location.host}/ws`;
}

export interface WebSocketContextValue {
  send: (msg: WSMessage) => void;
  onMessage: (handler: (msg: WSMessage) => void) => () => void;
  connected: boolean;
}

export const WebSocketContext = createContext<WebSocketContextValue | null>(null);

export function useWebSocketProvider(): WebSocketContextValue {
  const [connected, setConnected] = useState(false);
  const clientRef = useRef<WSClient | null>(null);

  // Create client eagerly during render so child components can register
  // handlers in their own effects before connect() fires.
  if (!clientRef.current) {
    clientRef.current = new WSClient(getWSUrl(), setConnected);
  }

  useEffect(() => {
    const client = clientRef.current!;
    client.connect();
    return () => {
      client.disconnect();
    };
  }, []);

  const send = useCallback((msg: WSMessage) => {
    clientRef.current?.send(msg);
  }, []);

  const onMessage = useCallback((handler: (msg: WSMessage) => void) => {
    return clientRef.current?.onMessage(handler) ?? (() => {});
  }, []);

  return { send, onMessage, connected };
}

/** Consume the shared WebSocket context. Must be inside <WebSocketContext.Provider>. */
export function useWebSocketCtx(): WebSocketContextValue {
  const ctx = useContext(WebSocketContext);
  if (!ctx) {
    throw new Error('useWebSocketCtx must be used within a WebSocketContext.Provider');
  }
  return ctx;
}
