import { useEffect, useRef, useState, useCallback } from 'react';
import type { ConnectionState, OutboundMessageType } from '../lib/types';
import { getWebSocketURL } from '../lib/ws';

interface UseWebSocketOptions {
  url?: string;
  onMessage?: (type: OutboundMessageType, data: unknown, sessionID: string) => void;
  reconnectInterval?: number;
}

interface UseWebSocketReturn {
  status: ConnectionState;
  send: (type: string, payload: Record<string, unknown>) => void;
}

export function useWebSocket(options: UseWebSocketOptions = {}): UseWebSocketReturn {
  const {
    url = getWebSocketURL(),
    onMessage,
    reconnectInterval = 3000,
  } = options;

  const [status, setStatus] = useState<ConnectionState>('disconnected');
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const unmountedRef = useRef(false);
  const onMessageRef = useRef(onMessage);

  // Keep onMessage ref current to avoid stale closures.
  onMessageRef.current = onMessage;

  const clearReconnectTimer = useCallback(() => {
    if (reconnectTimerRef.current !== null) {
      clearTimeout(reconnectTimerRef.current);
      reconnectTimerRef.current = null;
    }
  }, []);

  const connect = useCallback(() => {
    if (unmountedRef.current) return;

    clearReconnectTimer();

    setStatus('connecting');

    const socket = new WebSocket(url);
    wsRef.current = socket;

    socket.onopen = () => {
      // Status transitions to 'connected' only when we receive
      // the connection.ready message from the server.
    };

    socket.onmessage = (event: MessageEvent) => {
      try {
        const parsed = JSON.parse(event.data as string) as {
          type: string;
          data?: unknown;
          sessionId?: string;
        };

        // Transition to 'connected' when we get connection.ready.
        if (parsed.type === 'connection.ready') {
          setStatus('connected');
        }

        if (onMessageRef.current) {
          onMessageRef.current(
            parsed.type as OutboundMessageType,
            parsed.data,
            parsed.sessionId ?? '',
          );
        }
      } catch {
        // Ignore malformed messages.
      }
    };

    socket.onclose = () => {
      wsRef.current = null;
      if (!unmountedRef.current) {
        setStatus('disconnected');
        reconnectTimerRef.current = setTimeout(connect, reconnectInterval);
      }
    };

    socket.onerror = () => {
      // The error event is always followed by close, so we set 'error'
      // briefly — the close handler will then schedule reconnection.
      if (!unmountedRef.current) {
        setStatus('error');
      }
    };
  }, [url, reconnectInterval, clearReconnectTimer]);

  useEffect(() => {
    unmountedRef.current = false;
    connect();

    return () => {
      unmountedRef.current = true;
      clearReconnectTimer();
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
    };
  }, [connect, clearReconnectTimer]);

  const send = useCallback((type: string, payload: Record<string, unknown>) => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type, ...payload }));
    }
  }, []);

  return { status, send };
}
