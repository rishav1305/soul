/**
 * useChatSessions — Multi-session state manager (Phase 3)
 *
 * SINGLETON via React Context. There is exactly ONE instance of this hook's
 * state in the entire app. All components that call useChatSessions() share
 * the same state via ChatSessionsContext.
 *
 * Usage:
 *   1. Wrap the app root with <ChatSessionsProvider>
 *   2. Any component can call useChatSessions() to read/write shared state
 */

import {
  useState, useEffect, useCallback, useRef,
  createContext, useContext, type ReactNode,
} from 'react';
import { useWebSocketCtx as useWebSocket } from './useWebSocketContext.ts';
import { uuid, authFetch } from '../lib/api.ts';
import type {
  ChatMessage,
  ChatSession,
  ToolCallMessage,
  WSMessage,
  SendOptions,
  TokenUsage,
} from '../lib/types.ts';

/* ── Per-session state ──────────────────────────────────── */
export interface SessionState {
  messages: ChatMessage[];
  isStreaming: boolean;
  tokenUsage: TokenUsage | null;
}

function emptySession(): SessionState {
  return { messages: [], isStreaming: false, tokenUsage: null };
}

/* ── localStorage helpers ───────────────────────────────── */
const ACTIVE_KEY = 'soul-active-session';
function loadActiveId(): number | null {
  try {
    const raw = localStorage.getItem(ACTIVE_KEY);
    const id = Number(raw);
    return Number.isFinite(id) && id > 0 ? id : null;
  } catch { return null; }
}
function saveActiveId(id: number | null): void {
  try {
    if (id === null) localStorage.removeItem(ACTIVE_KEY);
    else localStorage.setItem(ACTIVE_KEY, String(id));
  } catch { /* ignore */ }
}

/* ── Tool-call updater ──────────────────────────────────── */
function updateAssistantToolCall(
  messages: ChatMessage[],
  toolCallId: string,
  updater: (tc: ToolCallMessage) => ToolCallMessage,
): ChatMessage[] {
  for (let i = messages.length - 1; i >= 0; i--) {
    const msg = messages[i]!;
    if (msg.role !== 'assistant' || !msg.toolCalls) continue;
    const idx = msg.toolCalls.findIndex((tc) => tc.id === toolCallId);
    if (idx === -1) continue;
    const newToolCalls = [...msg.toolCalls];
    newToolCalls[idx] = updater(newToolCalls[idx]!);
    const updated = [...messages];
    updated[i] = { ...msg, toolCalls: newToolCalls };
    return updated;
  }
  return messages;
}

/* ── Return type ────────────────────────────────────────── */
export interface ChatSessionsValue {
  // Session list
  sessions: ChatSession[];
  activeSessionId: number | null;
  setActiveSessionId: (id: number) => void;
  createSession: () => void;
  fetchSessions: () => Promise<void>;

  // Active session state (convenience)
  messages: ChatMessage[];
  isStreaming: boolean;
  tokenUsage: TokenUsage | null;

  // Per-session accessor
  getSessionState: (id: number) => SessionState;

  // Actions
  sendMessage: (content: string, options?: SendOptions, sessionId?: number | null) => void;
  stopStreaming: (targetSessionId?: number) => void;
  retryFromMessage: (messageId: string, sid?: number) => void;
  editMessage: (messageId: string, newContent: string, sid?: number) => void;
  backgroundSession: (id: number) => void;

  // Status buckets
  runningSessions: ChatSession[];
  unreadSessions: ChatSession[];

  // WS
  connected: boolean;
}

/* ── Context ────────────────────────────────────────────── */
const ChatSessionsContext = createContext<ChatSessionsValue | null>(null);

/* ── Provider (mount ONCE at AppShell level) ────────────── */
export function ChatSessionsProvider({ children }: { children: ReactNode }) {
  const value = useChatSessionsInternal();
  return (
    <ChatSessionsContext.Provider value={value}>
      {children}
    </ChatSessionsContext.Provider>
  );
}

/* ── Public hook — reads from context (no state created) ── */
export function useChatSessions(): ChatSessionsValue {
  const ctx = useContext(ChatSessionsContext);
  if (!ctx) {
    throw new Error('useChatSessions must be used inside <ChatSessionsProvider>');
  }
  return ctx;
}

/* ── Internal hook — creates the single state instance ─────
   Only called by ChatSessionsProvider. Never call directly. */
function useChatSessionsInternal(): ChatSessionsValue {
  const { send, onMessage, connected } = useWebSocket();

  // Flat session list (metadata only, no messages)
  const [sessions, setSessions] = useState<ChatSession[]>([]);
  const [activeSessionId, _setActiveSessionId] = useState<number | null>(loadActiveId);

  // Per-session message/streaming state
  const [sessionStates, setSessionStates] = useState<Map<number, SessionState>>(new Map());

  // Track last model used per session for labelling new assistant messages
  const lastModelRef = useRef<Map<number, string | undefined>>(new Map());

  // Polling timers for background sessions
  const pollingRefs = useRef<Map<number, ReturnType<typeof setInterval>>>(new Map());

  // Ref for stable access without re-renders
  const activeSessionIdRef = useRef<number | null>(activeSessionId);
  useEffect(() => { activeSessionIdRef.current = activeSessionId; }, [activeSessionId]);

  /* ── Helpers ──────────────────────────────────────────── */

  const getSessionState = useCallback((id: number): SessionState => {
    return sessionStates.get(id) ?? emptySession();
  }, [sessionStates]);

  const setSessionState = useCallback((
    id: number,
    updater: (prev: SessionState) => SessionState,
  ) => {
    setSessionStates((prev) => {
      const next = new Map(prev);
      next.set(id, updater(prev.get(id) ?? emptySession()));
      return next;
    });
  }, []);

  /* ── Session list fetch ───────────────────────────────── */

  const fetchSessions = useCallback(async () => {
    try {
      const res = await authFetch('/api/sessions');
      if (!res.ok) return;
      const data = await res.json();
      // Go API returns {sessions: [...]} — unwrap to bare array
      const list: ChatSession[] = Array.isArray(data) ? data : (data.sessions ?? []);
      setSessions(list);
    } catch { /* ignore */ }
  }, []);

  useEffect(() => { fetchSessions(); }, [fetchSessions]);

  /* ── Load messages for a session from API ─────────────── */

  const loadSessionMessages = useCallback(async (id: number) => {
    try {
      const res = await authFetch(`/api/sessions/${id}/messages`);
      if (!res.ok) return;
      const raw = await res.json();
      // Go API returns {messages: [...]} — unwrap to bare array
      const records: Array<{ id: number; role: string; content: string }> = Array.isArray(raw) ? raw : (raw.messages ?? []);
      const hydrated: ChatMessage[] = records.map((r) => ({
        id: String(r.id),
        role: r.role as 'user' | 'assistant',
        content: r.content,
        toolCalls: [],
        timestamp: new Date(),
      }));
      setSessionState(id, (prev) => ({ ...prev, messages: hydrated }));
    } catch { /* ignore */ }
  }, [setSessionState]);

  /* ── Background polling ───────────────────────────────── */

  const startPolling = useCallback((id: number) => {
    if (pollingRefs.current.has(id)) return; // already polling
    const timer = setInterval(async () => {
      try {
        const res = await authFetch(`/api/sessions/${id}/messages`);
        if (!res.ok) return;
        const raw = await res.json();
        // Go API returns {messages: [...]} — unwrap to bare array
        const records: Array<{ id: number; role: string; content: string }> = Array.isArray(raw) ? raw : (raw.messages ?? []);
        const hydrated: ChatMessage[] = records.map((r) => ({
          id: String(r.id),
          role: r.role as 'user' | 'assistant',
          content: r.content,
          toolCalls: [],
          timestamp: new Date(),
        }));
        setSessionState(id, (prev) => ({ ...prev, messages: hydrated }));
      } catch { /* ignore */ }
    }, 1000);
    pollingRefs.current.set(id, timer);
  }, [setSessionState]);

  const stopPolling = useCallback((id: number) => {
    const timer = pollingRefs.current.get(id);
    if (timer) {
      clearInterval(timer);
      pollingRefs.current.delete(id);
    }
  }, []);

  // Cleanup all timers on unmount
  useEffect(() => {
    return () => {
      for (const timer of pollingRefs.current.values()) clearInterval(timer);
    };
  }, []);

  /* ── Switch active session ────────────────────────────── */

  const setActiveSessionId = useCallback((id: number) => {
    _setActiveSessionId(id);
    saveActiveId(id);
    activeSessionIdRef.current = id;
    stopPolling(id);
    // Mark read via API
    authFetch(`/api/sessions/${id}/read`, { method: 'PATCH' }).catch(() => {});
    // Update local status
    setSessions((prev) => prev.map((s) =>
      s.id === id && s.status === 'completed_unread' ? { ...s, status: 'idle' } : s,
    ));
    // Load messages if not already loaded
    loadSessionMessages(id);
  }, [stopPolling, loadSessionMessages]);

  /* ── Create new session ───────────────────────────────── */

  const createSession = useCallback(() => {
    _setActiveSessionId(null);
    saveActiveId(null);
    activeSessionIdRef.current = null;
  }, []);

  /* ── Send message ─────────────────────────────────────── */

  const sendMessage = useCallback((
    content: string,
    options?: SendOptions,
    sessionId?: number | null,
  ) => {
    const sid = sessionId !== undefined ? sessionId : activeSessionIdRef.current;

    // Optimistic user message
    const userMsg: ChatMessage = {
      id: uuid(),
      role: 'user',
      content,
      toolCalls: [],
      timestamp: new Date(),
    };

    if (sid) {
      setSessionState(sid, (prev) => ({
        ...prev,
        isStreaming: true,
        messages: [...prev.messages, userMsg],
      }));
    }

    send({
      type: 'chat.send',
      content,
      sessionId: sid ? String(sid) : undefined,
      data: options ? {
        model: options.model,
        chat_type: options.chatType,
        context: options.context,
      } : undefined,
    });
  }, [send, setSessionState]);

  /* ── WS message routing ───────────────────────────────── */

  useEffect(() => {
    const unsub = onMessage((msg: WSMessage) => {
      // Determine which session this message belongs to
      const msgSessionId = msg.sessionId ? Number(msg.sessionId) : activeSessionIdRef.current;
      if (!msgSessionId) return;

      switch (msg.type) {

        case 'session.created': {
          const data = msg.data as { session_id: number; sessionId?: number };
          const newId = data.session_id ?? data.sessionId;
          if (activeSessionIdRef.current === null) {
            _setActiveSessionId(newId);
            saveActiveId(newId);
            activeSessionIdRef.current = newId;
          }
          setSessionStates((prev) => {
            if (prev.has(newId)) return prev;
            const next = new Map(prev);
            next.set(newId, prev.get(0) ?? emptySession());
            next.delete(0);
            return next;
          });
          fetchSessions();
          break;
        }

        case 'session.status_changed': {
          const data = msg.data as { session_id?: number; sessionId?: number; status: ChatSession['status'] };
          const sid = data.session_id ?? data.sessionId;
          if (!sid) break;
          setSessions((prev) => prev.map((s) =>
            s.id === sid ? { ...s, status: data.status } : s,
          ));
          if (data.status !== 'running') {
            stopPolling(sid);
            setSessionState(sid, (prev) => ({ ...prev, isStreaming: false }));
          }
          // Start background polling if this session is running but not active
          if (data.status === 'running' && sid !== activeSessionIdRef.current) {
            startPolling(sid);
          }
          break;
        }

        case 'session.updated': {
          const data = msg.data as { session_id?: number; sessionId?: number; title: string; summary: string; model: string };
          const updatedSid = data.session_id ?? data.sessionId;
          if (!updatedSid) return;
          setSessions((prev) => prev.map((s) =>
            s.id === updatedSid
              ? { ...s, title: data.title || s.title, summary: data.summary || s.summary, model: data.model || s.model }
              : s,
          ));
          break;
        }

        case 'chat.token': {
          const token = msg.content ?? '';
          setSessionState(msgSessionId, (prev) => {
            const messages = prev.messages;
            const last = messages[messages.length - 1];
            if (last && last.role === 'assistant') {
              return {
                ...prev,
                messages: [
                  ...messages.slice(0, -1),
                  { ...last, content: last.content + token },
                ],
              };
            }
            return {
              ...prev,
              messages: [
                ...messages,
                {
                  id: uuid(),
                  role: 'assistant' as const,
                  content: token,
                  toolCalls: [],
                  timestamp: new Date(),
                  model: lastModelRef.current.get(msgSessionId),
                },
              ],
            };
          });
          break;
        }

        case 'chat.thinking': {
          const token = msg.content ?? '';
          setSessionState(msgSessionId, (prev) => {
            const messages = prev.messages;
            const last = messages[messages.length - 1];
            if (last && last.role === 'assistant') {
              return {
                ...prev,
                messages: [
                  ...messages.slice(0, -1),
                  { ...last, thinking: (last.thinking ?? '') + token },
                ],
              };
            }
            return {
              ...prev,
              messages: [
                ...messages,
                {
                  id: uuid(),
                  role: 'assistant' as const,
                  content: '',
                  thinking: token,
                  toolCalls: [],
                  timestamp: new Date(),
                  model: lastModelRef.current.get(msgSessionId),
                },
              ],
            };
          });
          break;
        }

        case 'chat.done': {
          const data = msg.data as { input_tokens?: number; output_tokens?: number; context_pct?: number } | undefined;
          setSessionState(msgSessionId, (prev) => ({
            ...prev,
            isStreaming: false,
            tokenUsage: data?.input_tokens || data?.output_tokens ? {
              inputTokens: data?.input_tokens ?? 0,
              outputTokens: data?.output_tokens ?? 0,
              contextPct: data?.context_pct ?? 0,
            } : prev.tokenUsage,
          }));
          break;
        }

        case 'pm.notification': {
          const data = msg.data as { severity: string; task_ids: number[]; check: string };
          setSessionState(msgSessionId, (prev) => ({
            ...prev,
            messages: [
              ...prev.messages,
              {
                id: uuid(),
                role: 'assistant' as const,
                content: msg.content ?? '',
                toolCalls: [],
                timestamp: new Date(),
                pmNotification: {
                  severity: data.severity as 'info' | 'warning' | 'error',
                  taskIds: data.task_ids ?? [],
                  check: data.check ?? '',
                },
              },
            ],
          }));
          break;
        }

        case 'tool.call': {
          const data = msg.data as { id: string; name: string; input: unknown };
          const toolCall: ToolCallMessage = {
            id: data.id,
            name: data.name,
            input: data.input,
            status: 'running',
            findings: [],
          };
          setSessionState(msgSessionId, (prev) => {
            const messages = prev.messages;
            const last = messages[messages.length - 1];
            if (last && last.role === 'assistant') {
              return {
                ...prev,
                messages: [
                  ...messages.slice(0, -1),
                  { ...last, toolCalls: [...(last.toolCalls ?? []), toolCall] },
                ],
              };
            }
            return {
              ...prev,
              messages: [
                ...messages,
                {
                  id: uuid(),
                  role: 'assistant' as const,
                  content: '',
                  toolCalls: [toolCall],
                  timestamp: new Date(),
                  model: lastModelRef.current.get(msgSessionId),
                },
              ],
            };
          });
          break;
        }

        case 'tool.result':
        case 'tool.complete': {
          const data = msg.data as { id: string; result?: unknown; findings?: unknown[] };
          setSessionState(msgSessionId, (prev) => ({
            ...prev,
            messages: updateAssistantToolCall(prev.messages, data.id, (tc) => ({
              ...tc,
              status: 'complete' as const,
              result: data.result,
              findings: Array.isArray(data.findings) ? data.findings as ToolCallMessage['findings'] : tc.findings,
            })),
          }));
          break;
        }

        case 'tool.error': {
          const data = msg.data as { id: string; error?: string };
          setSessionState(msgSessionId, (prev) => ({
            ...prev,
            messages: updateAssistantToolCall(prev.messages, data.id, (tc) => ({
              ...tc,
              status: 'error' as const,
              error: data.error,
            })),
          }));
          break;
        }

        case 'tool.progress': {
          const data = msg.data as { id: string; progress?: number; message?: string; findings?: unknown[] };
          setSessionState(msgSessionId, (prev) => ({
            ...prev,
            messages: updateAssistantToolCall(prev.messages, data.id, (tc) => ({
              ...tc,
              progress: data.progress,
              progressMessage: data.message,
              findings: Array.isArray(data.findings) ? data.findings as ToolCallMessage['findings'] : tc.findings,
            })),
          }));
          break;
        }

        case 'chat.model': {
          const data = msg.data as { model?: string };
          if (data?.model) {
            lastModelRef.current.set(msgSessionId, data.model);
          }
          break;
        }

        default:
          break;
      }
    });
    return unsub;
  }, [onMessage, setSessionState, fetchSessions, stopPolling, startPolling]);

  /* ── Stop streaming ───────────────────────────────────── */

  const stopStreaming = useCallback((targetSessionId?: number) => {
    const sid = targetSessionId ?? activeSessionIdRef.current;
    send({ type: 'chat.stop', sessionId: sid ? String(sid) : undefined });
    if (sid) {
      setSessionState(sid, (prev) => ({ ...prev, isStreaming: false }));
    }
  }, [send, setSessionState]);

  /* ── Retry / edit ─────────────────────────────────────── */

  const retryFromMessage = useCallback((messageId: string, sid?: number) => {
    const id = sid ?? activeSessionIdRef.current;
    if (!id) return;
    const state = sessionStates.get(id) ?? emptySession();
    const msg = state.messages.find((m) => m.id === messageId && m.role === 'user');
    if (!msg) return;
    const idx = state.messages.indexOf(msg);
    setSessionState(id, (prev) => ({ ...prev, messages: prev.messages.slice(0, idx) }));
    setTimeout(() => sendMessage(msg.content, undefined, id), 0);
  }, [sessionStates, setSessionState, sendMessage]);

  const editMessage = useCallback((messageId: string, newContent: string, sid?: number) => {
    const id = sid ?? activeSessionIdRef.current;
    if (!id) return;
    const state = sessionStates.get(id) ?? emptySession();
    const idx = state.messages.findIndex((m) => m.id === messageId && m.role === 'user');
    if (idx === -1) return;
    setSessionState(id, (prev) => ({ ...prev, messages: prev.messages.slice(0, idx) }));
    setTimeout(() => sendMessage(newContent, undefined, id), 0);
  }, [sessionStates, setSessionState, sendMessage]);

  /* ── Background session trigger ──────────────────────── */

  const backgroundSession = useCallback((id: number) => {
    startPolling(id);
  }, [startPolling]);

  /* ── Computed values ──────────────────────────────────── */

  const activeState = activeSessionId ? (sessionStates.get(activeSessionId) ?? emptySession()) : emptySession();

  const runningSessions = sessions.filter((s) => s.status === 'running');
  const unreadSessions = sessions.filter((s) => s.status === 'completed_unread');

  return {
    sessions,
    activeSessionId,
    setActiveSessionId,
    createSession,
    fetchSessions,

    messages: activeState.messages,
    isStreaming: activeState.isStreaming,
    tokenUsage: activeState.tokenUsage,

    getSessionState,

    sendMessage,
    stopStreaming,
    retryFromMessage,
    editMessage,
    backgroundSession,

    runningSessions,
    unreadSessions,

    connected,
  };
}
