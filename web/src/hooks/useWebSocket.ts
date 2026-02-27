import { useEffect, useRef, useState, useCallback } from 'react';
import { WSClient } from '../lib/ws.ts';
import type { WSMessage } from '../lib/types.ts';

function getWSUrl(): string {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  return `${proto}//${window.location.host}/ws`;
}

export function useWebSocket() {
  const [connected, setConnected] = useState(false);
  const clientRef = useRef<WSClient | null>(null);

  useEffect(() => {
    const client = new WSClient(getWSUrl(), setConnected);
    clientRef.current = client;
    client.connect();

    return () => {
      client.disconnect();
      clientRef.current = null;
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
