import { useState, useEffect, useCallback, useRef } from 'react';
import { authFetch } from '../lib/api.ts';

/** Shape of a team chat message. */
export interface TeamChatMessage {
  id: string;
  sender: string;
  content: string;
  timestamp: number;
  conferenceRound?: number;
}

// -- Backend → Frontend mapping -----------------------------------------------

/** Map a backend ChatMessage to the frontend TeamChatMessage shape. */
function mapMessage(raw: Record<string, unknown>): TeamChatMessage {
  const ts = raw.timestamp
    ? new Date(String(raw.timestamp)).getTime()
    : Date.now();

  return {
    id: String(raw.id ?? `msg-${Date.now()}-${Math.random()}`),
    sender: String(raw.from ?? raw.sender ?? 'system'),
    content: String(raw.body ?? raw.content ?? ''),
    timestamp: ts,
  };
}

// -- Hook ---------------------------------------------------------------------

interface UseTeamChatResult {
  messages: TeamChatMessage[];
  conferenceActive: boolean;
  sendMessage: (content: string, sender?: string, to?: string) => void;
  startConference: (topic: string) => void;
  endConference: () => void;
}

/**
 * useTeamChat -- manages team chat backed by the real API + WebSocket.
 *
 * - History: GET /api/team/chat/history
 * - Send: POST /api/team/chat
 * - Real-time: WebSocket subscription to chat.message
 */
export function useTeamChat(): UseTeamChatResult {
  const [messages, setMessages] = useState<TeamChatMessage[]>([]);
  const [conferenceActive, setConferenceActive] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const mountedRef = useRef(true);
  const reconnectRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Fetch chat history on mount.
  useEffect(() => {
    mountedRef.current = true;

    const fetchHistory = async () => {
      try {
        const res = await authFetch('/api/team/chat/history?limit=100');
        if (!res.ok) return;
        const data: Record<string, unknown>[] = await res.json();
        if (mountedRef.current && Array.isArray(data)) {
          setMessages(data.map(mapMessage));
        }
      } catch {
        // Non-fatal: start with empty history.
      }
    };

    void fetchHistory();

    return () => { mountedRef.current = false; };
  }, []);

  // Connect WebSocket for real-time messages.
  useEffect(() => {
    let attempt = 0;
    const BACKOFF = [1000, 2000, 4000, 8000, 15000];

    const connect = () => {
      if (!mountedRef.current) return;

      const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
      const ws = new WebSocket(`${proto}//${location.host}/ws/team`);
      wsRef.current = ws;

      ws.onopen = () => {
        attempt = 0;
        // Subscribe to chat messages.
        ws.send(JSON.stringify({ action: 'subscribe', type: 'chat.message' }));
      };

      ws.onmessage = (ev) => {
        if (!mountedRef.current) return;
        try {
          const msg = JSON.parse(ev.data as string) as {
            type: string;
            payload: Record<string, unknown>;
          };
          if (msg.type === 'chat.message') {
            const chatMsg = mapMessage(msg.payload);
            setMessages((prev) => {
              // Deduplicate by ID.
              if (prev.some((m) => m.id === chatMsg.id)) return prev;
              return [...prev, chatMsg];
            });
          }
        } catch {
          // Ignore malformed messages.
        }
      };

      ws.onclose = () => {
        wsRef.current = null;
        if (!mountedRef.current) return;
        const delay = BACKOFF[Math.min(attempt, BACKOFF.length - 1)];
        attempt++;
        reconnectRef.current = setTimeout(connect, delay);
      };

      ws.onerror = () => {
        // onclose will fire after this.
      };
    };

    connect();

    return () => {
      mountedRef.current = false;
      wsRef.current?.close();
      wsRef.current = null;
      if (reconnectRef.current) clearTimeout(reconnectRef.current);
    };
  }, []);

  /**
   * sendMessage — post a message to the team chat backend.
   *
   * @param content  Message text (may contain @mentions).
   * @param sender   Display name of the sender (default: 'CEO').
   * @param to       Target agent name or '*' for broadcast (default: '*').
   *                 Pass a specific agent name to send a directed DM.
   */
  const sendMessage = useCallback((content: string, sender = 'CEO', to = '*') => {
    // Optimistic local append.
    const localMsg: TeamChatMessage = {
      id: `local-${Date.now()}`,
      sender,
      content,
      timestamp: Date.now(),
    };
    setMessages((prev) => [...prev, localMsg]);

    // Fire-and-forget POST to backend.
    authFetch('/api/team/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        to,
        body: content,
        from: sender,
        priority: 'P2',
      }),
    }).catch(() => {
      // Message already shown locally; backend failure is non-fatal for UX.
    });
  }, []);

  const startConference = useCallback((topic: string) => {
    setConferenceActive(true);
    const systemMsg: TeamChatMessage = {
      id: `sys-${Date.now()}`,
      sender: 'system',
      content: `--- Conference started: ${topic} ---`,
      timestamp: Date.now(),
    };
    setMessages((prev) => [...prev, systemMsg]);
  }, []);

  const endConference = useCallback(() => {
    setConferenceActive(false);
    const systemMsg: TeamChatMessage = {
      id: `sys-${Date.now()}`,
      sender: 'system',
      content: '--- Conference ended ---',
      timestamp: Date.now(),
    };
    setMessages((prev) => [...prev, systemMsg]);
  }, []);

  return { messages, conferenceActive, sendMessage, startConference, endConference };
}
