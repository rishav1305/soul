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
  reconnect: () => void;    // Manual reconnect trigger (also clears auth circuit breaker)
  authError: boolean;       // true when auth circuit breaker has fired
}

// Exponential backoff: 1s → 2s → 4s → 8s → 15s max, with ±30% jitter.
const BASE_DELAY = 1000;
const MAX_DELAY = 15000;
const JITTER = 0.3;
const MAX_RECONNECT_ATTEMPTS = 10;

function backoffDelay(attempt: number): number {
  const exponential = Math.min(BASE_DELAY * Math.pow(2, attempt), MAX_DELAY);
  const jitter = exponential * JITTER * (Math.random() * 2 - 1); // ±30%
  return Math.max(500, exponential + jitter);
}

export function useWebSocket(options: UseWebSocketOptions = {}): UseWebSocketReturn {
  const { onMessage } = options;

  const [status, setStatus] = useState<ConnectionState>('disconnected');
  const [reconnectAttempt, setReconnectAttempt] = useState(0);
  const [authError, setAuthError] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const unmountedRef = useRef(false);
  const onMessageRef = useRef(onMessage);
  const urlRef = useRef(options.url);
  const attemptRef = useRef(0);
  const openTimeRef = useRef<number>(0);
  const consecutiveAuthFailuresRef = useRef(0);
  const authErrorRef = useRef(false);

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

    fetchWSTicket().then(({ ticket, status: ticketStatus }) => {
      if (unmountedRef.current) return;

      reportWSLifecycle('ticket_fetch', {
        status: ticketStatus,
        fallback: ticket === null && ticketStatus !== 401,
      });

      // Auth circuit breaker: two consecutive 401s mean auth is broken.
      if (ticketStatus === 401) {
        consecutiveAuthFailuresRef.current++;
        if (consecutiveAuthFailuresRef.current >= 2) {
          authErrorRef.current = true;
          setAuthError(true);
          setStatus('error');
          return;
        }
        setStatus('disconnected');
        const delay = backoffDelay(attemptRef.current);
        attemptRef.current++;
        setReconnectAttempt(attemptRef.current);
        reconnectTimerRef.current = setTimeout(connect, delay);
        return;
      }
      consecutiveAuthFailuresRef.current = 0;

      reportWSLifecycle('connect_attempt', { attempt: attemptRef.current, ticketUsed: !!ticket });

      const socket = new WebSocket(getWebSocketURL(ticket));
      wsRef.current = socket;

      socket.onopen = () => {
        openTimeRef.current = performance.now();
        reportWSLifecycle('open', { attempt: attemptRef.current });
      };

      socket.onmessage = (event: MessageEvent) => {
        try {
          const raw = JSON.parse(event.data as string) as unknown;
          const messages = Array.isArray(raw) ? raw : [raw];
          for (const parsed of messages) {
            const msg = parsed as { type: string; data?: unknown; sessionId?: string };
            if (msg.type === 'connection.ready') {
              setStatus('connected');
              attemptRef.current = 0;
              setReconnectAttempt(0);
            }
            if (onMessageRef.current) {
              onMessageRef.current(
                msg.type as OutboundMessageType,
                msg.data,
                msg.sessionId ?? '',
              );
            }
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
          if (authErrorRef.current) {
            setStatus('error');
            return;
          }
          if (attemptRef.current >= MAX_RECONNECT_ATTEMPTS) {
            setStatus('error');
            return;
          }
          const delay = backoffDelay(attemptRef.current);
          attemptRef.current++;
          setReconnectAttempt(attemptRef.current);

          reconnectTimerRef.current = setTimeout(connect, delay);
        }
      };

      socket.onerror = () => {
        reportWSLifecycle('error', { attempt: attemptRef.current });
        if (!unmountedRef.current) {
          setStatus('error');
        }
      };
    });
  }, [clearReconnectTimer]);

  const send = useCallback((type: string, payload: Record<string, unknown>) => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type, ...payload }));
    }
  }, []);

  const reconnect = useCallback(() => {
    if (unmountedRef.current) return;
    authErrorRef.current = false;
    setAuthError(false);
    consecutiveAuthFailuresRef.current = 0;
    attemptRef.current = 0;
    setReconnectAttempt(0);
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }
    clearReconnectTimer();
    connect();
  }, [connect, clearReconnectTimer]);

  useEffect(() => {
    unmountedRef.current = false;

    const onVisibilityChange = () => {
      if (
        document.visibilityState === 'visible' &&
        wsRef.current === null &&
        !authErrorRef.current &&
        attemptRef.current < MAX_RECONNECT_ATTEMPTS
      ) {
        clearReconnectTimer();
        attemptRef.current = 0;
        setReconnectAttempt(0);
        connect();
      }
    };
    document.addEventListener('visibilitychange', onVisibilityChange);

    connect();

    return () => {
      unmountedRef.current = true;
      document.removeEventListener('visibilitychange', onVisibilityChange);
      clearReconnectTimer();
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
    };
  }, [connect, clearReconnectTimer]);

  return { status, send, reconnectAttempt, reconnect, authError };
}
