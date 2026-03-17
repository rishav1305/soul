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
  // Tracks the specific socket closed by reconnect() so its onclose fires
  // the early-return path rather than triggering a duplicate reconnect schedule.
  const manuallyClosedSocketRef = useRef<WebSocket | null>(null);
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

    // Attach all event handlers to a socket. Shared by both the custom-URL
    // path and the ticket path so retry/backoff/visibility logic is identical.
    function setupSocket(socket: WebSocket) {
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
            const msg = parsed as { type: string; data?: unknown; sessionId?: string; messageId?: string };
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
                msg.messageId,
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
        // Only clear wsRef if this socket is still the current one — reconnect()
        // may have already assigned a new socket before this onclose fires.
        if (wsRef.current === socket) {
          wsRef.current = null;
        }
        // If reconnect() explicitly closed this exact socket, skip auto-scheduling.
        // Using a socket reference (not a global flag) prevents a new socket's
        // onclose from consuming the flag before the old socket's onclose fires.
        if (manuallyClosedSocketRef.current === socket) {
          manuallyClosedSocketRef.current = null;
          return;
        }
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
          // Don't schedule timed reconnects when the tab is hidden — the
          // visibilitychange handler will reconnect when the user returns.
          if (document.visibilityState === 'hidden') {
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
    }

    // If a custom URL is provided (e.g. for tests), skip ticket fetch.
    if (urlRef.current) {
      reportWSLifecycle('connect_attempt', { attempt: attemptRef.current, ticketUsed: false });
      setupSocket(new WebSocket(urlRef.current));
      return;
    }

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

      setupSocket(new WebSocket(getWebSocketURL(ticket)));
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
    clearReconnectTimer();
    if (wsRef.current) {
      // Record which socket we're closing so its onclose skips auto-scheduling.
      manuallyClosedSocketRef.current = wsRef.current;
      wsRef.current.close();
      wsRef.current = null;
    }
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
