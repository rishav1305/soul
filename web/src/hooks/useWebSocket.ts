import { useEffect, useRef, useState, useCallback } from 'react';
import type { ConnectionState, OutboundMessageType } from '../lib/types';
import { getWebSocketURL, fetchWSTicket } from '../lib/ws';
import { reportError, reportWSLifecycle } from '../lib/telemetry';

interface UseWebSocketOptions {
  url?: string;
  onMessage?: (type: OutboundMessageType, data: unknown, sessionID: string, messageId?: string) => void;
}

interface UseWebSocketReturn {
  status: ConnectionState;
  send: (type: string, payload: Record<string, unknown>) => void;
  reconnectAttempt: number;
}

// Exponential backoff: 1s → 2s → 4s → 8s → 15s max, with ±30% jitter.
const BASE_DELAY = 1000;
const MAX_DELAY = 15000;
const JITTER = 0.3;

function backoffDelay(attempt: number): number {
  const exponential = Math.min(BASE_DELAY * Math.pow(2, attempt), MAX_DELAY);
  const jitter = exponential * JITTER * (Math.random() * 2 - 1); // ±30%
  return Math.max(500, exponential + jitter);
}

export function useWebSocket(options: UseWebSocketOptions = {}): UseWebSocketReturn {
  const { onMessage } = options;

  const [status, setStatus] = useState<ConnectionState>('disconnected');
  const [reconnectAttempt, setReconnectAttempt] = useState(0);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const unmountedRef = useRef(false);
  const onMessageRef = useRef(onMessage);
  const urlRef = useRef(options.url);
  const attemptRef = useRef(0);
  const openTimeRef = useRef<number>(0);

  // Keep callback and url refs current to avoid stale closures.
  onMessageRef.current = onMessage;
  urlRef.current = options.url;

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

    // If a custom URL was provided (e.g. for testing), use it directly.
    // Otherwise fetch a short-lived ticket so the real token is never sent in the WS URL.
    // Falls back to the raw token if the ticket endpoint is unavailable.
    const connectStart = performance.now();
    const wsUrlPromise = urlRef.current
      ? Promise.resolve(urlRef.current)
      : fetchWSTicket().then((ticket) => getWebSocketURL(ticket));

    wsUrlPromise.then((wsUrl) => {
      if (unmountedRef.current) return;
      reportWSLifecycle('connect_attempt', { attempt: attemptRef.current });
      openTimeRef.current = connectStart;
      const socket = new WebSocket(wsUrl);
      wsRef.current = socket;

      socket.onopen = () => {
        reportWSLifecycle('open', { attempt: attemptRef.current });
        // Status transitions to 'connected' only when we receive
        // the connection.ready message from the server.
      };

      socket.onmessage = (event: MessageEvent) => {
        try {
          const parsed = JSON.parse(event.data as string) as {
            type: string;
            data?: unknown;
            sessionId?: string;
            messageId?: string; // top-level replay anchor set by sendToClient
          };

          // Transition to 'connected' when we get connection.ready.
          if (parsed.type === 'connection.ready') {
            setStatus('connected');
            // Reset backoff on successful connection.
            attemptRef.current = 0;
            setReconnectAttempt(0);
          }

          if (onMessageRef.current) {
            onMessageRef.current(
              parsed.type as OutboundMessageType,
              parsed.data,
              parsed.sessionId ?? '',
              parsed.messageId,
            );
          }
        } catch (err) {
          reportError('useWebSocket.parse', err);
        }
      };

      socket.onclose = (event: CloseEvent) => {
        const duration = performance.now() - openTimeRef.current;
        reportWSLifecycle('close', {
          code: event.code,
          reason: event.reason,
          wasClean: event.wasClean,
          duration_ms: Math.round(duration),
        });
        wsRef.current = null;
        if (!unmountedRef.current) {
          setStatus('disconnected');
          const delay = backoffDelay(attemptRef.current);
          attemptRef.current++;
          setReconnectAttempt(attemptRef.current);

          reconnectTimerRef.current = setTimeout(connect, delay);
        }
      };

      socket.onerror = () => {
        reportWSLifecycle('error', { attempt: attemptRef.current });
        // The error event is always followed by close, so we set 'error'
        // briefly — the close handler will then schedule reconnection.
        if (!unmountedRef.current) {
          setStatus('error');
        }
      };
    });
  }, [clearReconnectTimer]);

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
    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) {
      throw new Error('socket not open');
    }
    wsRef.current.send(JSON.stringify({ type, ...payload }));
  }, []);

  return { status, send, reconnectAttempt };
}
