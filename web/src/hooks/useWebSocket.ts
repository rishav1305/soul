import { createContext, useContext, useEffect, useRef, useState, useCallback } from 'react';
import { WSClient } from '../lib/ws.ts';
import type { WSMessage } from '../lib/types.ts';

function getWSUrl(): string {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  return `${proto}//${window.location.host}/ws`;
}

interface WebSocketContextValue {
  send: (msg: WSMessage) => void;
  onMessage: (handler: (msg: WSMessage) => void) => () => void;
  connected: boolean;
}

export const WebSocketContext = createContext<WebSocketContextValue | null>(null);

export function useWebSocketProvider(): WebSocketContextValue {
  const [connected, setConnected] = useState(false);
  const clientRef = useRef<WSClient | null>(null);

  // Create client eagerly during render (not in effect) so child components
  // can register message handlers in their own effects before connect() fires.
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

export function useWebSocket(): WebSocketContextValue {
  const ctx = useContext(WebSocketContext);
  if (!ctx) {
    throw new Error('useWebSocket must be used within a WebSocketProvider');
  }
  return ctx;
}
