import { useState, useEffect, useRef, useCallback } from 'react';
import { authFetch } from '../lib/api.ts';

/** useAgentPane -- streams pane output for a named agent via WebSocket. */

// -- Hook ---------------------------------------------------------------------

interface UseAgentPaneResult {
  lines: string[];
  loading: boolean;
  error: string | null;
}

/** Maximum number of lines to keep in the buffer. */
const MAX_LINES = 500;

/** Reconnect backoff: 1s, 2s, 4s, 8s, 15s max. */
const BACKOFF_MS = [1000, 2000, 4000, 8000, 15000];

/**
 * useAgentPane -- connects to the team WebSocket and subscribes to pane.{agent}.
 * On initial connect, fetches the full pane buffer via REST, then applies
 * real-time deltas from the WebSocket.
 */
export function useAgentPane(agentName: string): UseAgentPaneResult {
  const [lines, setLines] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const attemptRef = useRef(0);
  const mountedRef = useRef(true);
  const reconnectRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Fetch initial pane buffer via REST.
  const fetchInitialPane = useCallback(async () => {
    try {
      const res = await authFetch(`/api/team/agents/${encodeURIComponent(agentName)}/pane`);
      if (!res.ok) return; // Non-fatal: WS will provide data
      const data = await res.json() as { lines?: string[] };
      if (data.lines && mountedRef.current) {
        setLines(data.lines.slice(-MAX_LINES));
      }
    } catch {
      // Silently ignore -- WS will provide data
    }
  }, [agentName]);

  const connect = useCallback(() => {
    if (!mountedRef.current) return;

    // Build WS URL relative to current page origin.
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${proto}//${location.host}/ws/team`;

    try {
      const ws = new WebSocket(wsUrl);
      wsRef.current = ws;

      ws.onopen = () => {
        if (!mountedRef.current) { ws.close(); return; }
        setLoading(false);
        setError(null);
        attemptRef.current = 0;

        // Subscribe to this agent's pane events.
        ws.send(JSON.stringify({ action: 'subscribe', type: `pane.${agentName}` }));
      };

      ws.onmessage = (ev) => {
        if (!mountedRef.current) return;
        try {
          const msg = JSON.parse(ev.data as string) as {
            type: string;
            payload: { diff_lines?: string[]; lines?: string[] };
          };

          if (msg.type === `pane.${agentName}`) {
            const delta = msg.payload;
            if (delta.diff_lines && delta.diff_lines.length > 0) {
              setLines((prev) => [...prev, ...delta.diff_lines].slice(-MAX_LINES));
            } else if (delta.lines && delta.lines.length > 0) {
              // Full buffer push (first connection).
              setLines(delta.lines.slice(-MAX_LINES));
            }
          }
        } catch {
          // Ignore malformed messages.
        }
      };

      ws.onerror = () => {
        if (mountedRef.current) setError('WebSocket error');
      };

      ws.onclose = () => {
        wsRef.current = null;
        if (!mountedRef.current) return;

        // Reconnect with backoff.
        const delay = BACKOFF_MS[Math.min(attemptRef.current, BACKOFF_MS.length - 1)];
        attemptRef.current++;
        reconnectRef.current = setTimeout(connect, delay);
      };
    } catch {
      setLoading(false);
      setError('WebSocket connection failed');
    }
  }, [agentName]);

  useEffect(() => {
    mountedRef.current = true;
    setLoading(true);
    setLines([]);
    setError(null);
    attemptRef.current = 0;

    // Fetch initial pane buffer, then connect WS for real-time deltas.
    void fetchInitialPane();
    connect();

    return () => {
      mountedRef.current = false;
      wsRef.current?.close();
      wsRef.current = null;
      if (reconnectRef.current) clearTimeout(reconnectRef.current);
    };
  }, [agentName, connect, fetchInitialPane]);

  return { lines, loading, error };
}
