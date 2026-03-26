import { useState, useCallback, useRef, useEffect } from 'react';
import type { Message, Session, OutboundMessageType, ConnectionState, ToolCallData, ProgressStep, ChatProduct, ThinkingConfig } from '../lib/types';
import { useWebSocket } from './useWebSocket';
import { reportError, reportWSLatency, reportUsage, reportAuthFailure } from '../lib/telemetry';
import { SendQueue } from '../lib/sendQueue';
import { getToken } from '../components/AuthGate';

interface UseChatReturn {
  messages: Message[];
  isStreaming: boolean;
  status: ConnectionState;
  authError: boolean;
  reconnectAttempt: number;
  sendMessage: (content: string, options?: { model?: string; thinking?: ThinkingConfig; attachments?: { name: string; mediaType: string; data: string }[] }) => void;
  stopGeneration: () => void;
  editAndResend: (messageId: string, newContent: string) => void;
  retryMessage: (messageId: string) => void;
  reauth: () => Promise<void>;
  reconnect: () => void;
  sessions: Session[];
  currentSessionID: string | null;
  createSession: () => void;
  switchSession: (id: string) => void;
  deleteSession: (id: string) => void;
  renameSession: (id: string, title: string) => void;
  activeProduct: ChatProduct;
  setProduct: (product: ChatProduct) => void;
}

const STREAMING_MESSAGE_ID = '__streaming__';
const STORAGE_KEY = 'soul-v2-session';

interface RawHistoryMessage {
  id: string;
  role: string;
  content: string;
  sessionId?: string;
  session_id?: string;
  createdAt?: string;
  created_at?: string;
  model?: string;
  thinking?: string;
  toolCalls?: ToolCallData[];
  usage?: { inputTokens: number; outputTokens: number; cacheReadInputTokens?: number };
}

interface ContentBlock {
  type: string;
  text?: string;
  id?: string;
  name?: string;
  input?: Record<string, unknown>;
  tool_use_id?: string;
  content?: string;
}

/**
 * Hydrate raw history messages: merge tool_use/tool_result into assistant
 * messages with proper toolCalls arrays so they render as ToolCallBlock
 * components instead of raw JSON.
 */
function hydrateHistory(raw: RawHistoryMessage[], sessionID: string): Message[] {
  const messages: Message[] = [];

  for (let i = 0; i < raw.length; i++) {
    const m = raw[i]!;
    const sid = m.sessionId || m.session_id || sessionID;
    const ts = (m.createdAt || m.created_at) ?? '';

    if (m.role === 'tool_use') {
      // Parse JSON array of content blocks stored by backend.
      let blocks: ContentBlock[] = [];
      try { blocks = JSON.parse(m.content); } catch { /* not JSON array */ }

      if (!Array.isArray(blocks)) {
        // Single tool object (legacy format)
        try {
          const single = JSON.parse(m.content) as ContentBlock;
          if (single.name) blocks = [single];
        } catch { /* fallback: render as-is */ }
      }

      // Extract text and tool_use blocks.
      let text = '';
      const toolCalls: ToolCallData[] = [];
      for (const b of blocks) {
        if (b.type === 'text' && b.text) {
          text += b.text;
        } else if ((b.type === 'tool_use' || b.name) && b.name) {
          toolCalls.push({
            id: b.id ?? `hist-${b.name}-${i}`,
            name: b.name,
            input: b.input ?? {},
            status: 'complete',
          });
        }
      }

      // Absorb subsequent tool_result messages into toolCalls outputs.
      while (i + 1 < raw.length && raw[i + 1]!.role === 'tool_result') {
        i++;
        const tr = raw[i]!;
        try {
          const parsed = JSON.parse(tr.content) as { tool_use_id?: string; content?: string };
          const tc = toolCalls.find(t => t.id === parsed.tool_use_id);
          if (tc) {
            tc.output = parsed.content;
          }
        } catch { /* skip unparseable results */ }
      }

      messages.push({
        id: m.id,
        role: 'assistant',
        content: text,
        sessionID: sid,
        createdAt: ts,
        model: m.model,
        thinking: m.thinking,
        toolCalls: toolCalls.length > 0 ? toolCalls : undefined,
        usage: m.usage,
      });
      continue;
    }

    if (m.role === 'tool_result') {
      // Orphaned tool_result (not absorbed above) — skip, already handled.
      continue;
    }

    // Normal user/assistant message.
    messages.push({
      id: m.id,
      role: m.role as Message['role'],
      content: m.content,
      sessionID: sid,
      createdAt: ts,
      model: m.model,
      thinking: m.thinking,
      toolCalls: m.toolCalls,
      usage: m.usage,
    });
  }

  return messages;
}

function generateTempId(): string {
  return `temp-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
}

export function useChat(): UseChatReturn {
  const [messages, setMessages] = useState<Message[]>([]);
  const [isStreaming, setIsStreaming] = useState(false);
  const [authError, setAuthError] = useState(false);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [currentSessionID, setCurrentSessionID] = useState<string | null>(null);
  const [activeProduct, setActiveProduct] = useState<ChatProduct>('');

  // Track session ID in a ref so the onMessage callback always sees the latest value
  // without needing to re-create (which would cause useWebSocket to reconnect).
  const sessionIDRef = useRef<string | null>(null);
  const sessionsRef = useRef<Session[]>([]);
  const isStreamingRef = useRef(false);
  const pendingExplicitCreateRef = useRef(false);
  const sendTimeRef = useRef<number>(0);
  const firstTokenTimeRef = useRef<number>(0);
  const sendQueueRef = useRef(new SendQueue());
  const lastMessageIdRef = useRef<string | null>(null);
  // Remembers the last model/thinking used so retries and edits inherit them.
  const lastSentOptionsRef = useRef<{ model?: string; thinking?: ThinkingConfig } | null>(null);

  // Keep streaming ref in sync.
  useEffect(() => { isStreamingRef.current = isStreaming; }, [isStreaming]);

  const handleMessage = useCallback(
    (type: OutboundMessageType, data: unknown, sessionID: string, messageId?: string) => {
      // Forward task events to useTaskSync via DOM event.
      // Task events are not in OutboundMessageType — check raw string before typed switch.
      if ((type as string)?.startsWith('task.')) {
        window.dispatchEvent(new CustomEvent('ws:task-event', { detail: { type, data } }));
        return;
      }

      switch (type) {
        case 'connection.ready': {
          window.dispatchEvent(new Event('ws:connected'));
          setAuthError(false);
          if (sessionIDRef.current) {
            // RECONNECT — session ID already set. Re-subscribe and refresh history.
            // Reset streaming state: any in-flight stream is gone.
            setIsStreaming(false);
            isStreamingRef.current = false;
            // Remove any orphaned streaming placeholder (no chat.done will arrive for it).
            setMessages(prev => prev.filter(m => m.id !== STREAMING_MESSAGE_ID));
            const sid = sessionIDRef.current;
            queueMicrotask(() => {
              sendRef.current('session.switch', { sessionId: sid });
            });
          } else {
            // FIRST LOAD — restore from localStorage.
            const savedId = localStorage.getItem(STORAGE_KEY);
            if (savedId) {
              sessionIDRef.current = savedId;
              setCurrentSessionID(savedId);
              queueMicrotask(() => {
                sendRef.current('session.switch', { sessionId: savedId });
              });
            }
          }
          // Recover pending message from localStorage (browser refresh during deferred creation).
          const pendingRaw = localStorage.getItem('soul-v2-pending');
          if (pendingRaw && !pendingMessageRef.current) {
            try {
              const pending = JSON.parse(pendingRaw);
              if (pending?.content) {
                pendingMessageRef.current = pending;
                if (!sessionIDRef.current) {
                  sendRef.current('session.create', {});
                }
              }
            } catch (err) { reportError('useChat.pendingRestore', err); }
          }

          // Request replay of any missed messages since last disconnect
          if (lastMessageIdRef.current && sessionIDRef.current) {
            sendRef.current('session.resume', {
              sessionId: sessionIDRef.current,
              lastMessageId: lastMessageIdRef.current,
            });
          }

          // Flush any queued messages from before disconnect
          sendQueueRef.current.restore();
          if (sendQueueRef.current.pending() > 0) {
            sendQueueRef.current.flush((payload) => {
              const { type, ...data } = payload;
              sendRef.current(type as string, data as Record<string, unknown>);
            });
          }
          break;
        }

        case 'session.created': {
          const payload = data as { session: Session } | undefined;
          if (payload?.session?.id) {
            const newSession = payload.session;

            // Add to sessions list if not already present.
            const alreadyExists = sessionsRef.current.some(s => s.id === newSession.id);
            if (!alreadyExists) {
              const updated = [newSession, ...sessionsRef.current];
              sessionsRef.current = updated;
              setSessions(updated);
            }

            // Switch to the new session if no current session (deferred creation
            // on first send) OR if the user explicitly clicked New.
            if (!sessionIDRef.current || pendingExplicitCreateRef.current) {
              pendingExplicitCreateRef.current = false;
              sessionIDRef.current = newSession.id;
              setCurrentSessionID(newSession.id);
              localStorage.setItem(STORAGE_KEY, newSession.id);
              setMessages([]);
              setIsStreaming(false);
            }
          }
          break;
        }

        case 'session.updated': {
          const payload = data as { session: Session } | undefined;
          if (payload?.session?.id) {
            const updatedSession = payload.session;
            const updated = sessionsRef.current.map(s =>
              s.id === updatedSession.id ? updatedSession : s,
            );
            // If session not in list and has messages, add it.
            if (!sessionsRef.current.some(s => s.id === updatedSession.id) && updatedSession.messageCount > 0) {
              updated.unshift(updatedSession);
            }
            sessionsRef.current = updated;
            setSessions(updated);
          }
          break;
        }

        case 'session.list': {
          const payload = data as { sessions: Session[] } | undefined;
          if (payload?.sessions) {
            sessionsRef.current = payload.sessions;
            setSessions(payload.sessions);
            // Auto-select the most recent session if none is active (e.g. new device).
            if (!sessionIDRef.current && payload.sessions.length > 0) {
              const first = payload.sessions[0]!;
              sessionIDRef.current = first.id;
              setCurrentSessionID(first.id);
              localStorage.setItem(STORAGE_KEY, first.id);
              sendRef.current('session.switch', { sessionId: first.id });
            }
          }
          break;
        }

        case 'session.deleted': {
          const payload = data as { sessionId: string } | undefined;
          if (payload?.sessionId) {
            const deletedId = payload.sessionId;
            const updated = sessionsRef.current.filter(s => s.id !== deletedId);
            sessionsRef.current = updated;
            setSessions(updated);

            // If the deleted session was active, switch to another or clear.
            if (sessionIDRef.current === deletedId) {
              if (updated.length > 0) {
                const next = updated[0]!;
                sessionIDRef.current = next.id;
                setCurrentSessionID(next.id);
                localStorage.setItem(STORAGE_KEY, next.id);
                setMessages([]);
                sendRef.current('session.switch', { sessionId: next.id });
              } else {
                sessionIDRef.current = null;
                setCurrentSessionID(null);
                localStorage.removeItem(STORAGE_KEY);
                setMessages([]);
              }
            }
          }
          break;
        }

        case 'session.history': {
          // Server sends message history when switching sessions.
          if (isStreamingRef.current) break; // Don't overwrite during streaming.
          const payload = data as { messages: Message[]; session?: { product?: string } } | undefined;
          if (payload?.messages && sessionID === sessionIDRef.current) {
            setMessages(hydrateHistory(payload.messages as unknown as RawHistoryMessage[], sessionID));
          }
          if (payload?.session?.product !== undefined) {
            setActiveProduct(payload.session.product as ChatProduct);
          }
          break;
        }

        case 'session.productSet': {
          const { product } = data as { product: string };
          setActiveProduct(product as ChatProduct);
          break;
        }

        case 'chat.thinking': {
          const payload = data as { text: string } | undefined;
          if (!payload) break;

          setMessages(prev => {
            const streamIdx = prev.findIndex(m => m.id === STREAMING_MESSAGE_ID);
            if (streamIdx === -1) {
              const placeholder: Message = {
                id: STREAMING_MESSAGE_ID,
                role: 'assistant',
                content: '',
                sessionID: sessionID,
                createdAt: new Date().toISOString(),
                thinking: payload.text,
              };
              return [...prev, placeholder];
            }
            const updated = [...prev];
            const existing = updated[streamIdx]!;
            updated[streamIdx] = {
              ...existing,
              thinking: (existing.thinking ?? '') + payload.text,
            };
            return updated;
          });
          break;
        }

        case 'chat.token': {
          if (sendTimeRef.current > 0 && firstTokenTimeRef.current === 0) {
            firstTokenTimeRef.current = performance.now();
          }
          const payload = data as { token: string; messageId: string } | undefined;
          if (!payload) break;

          // Track the top-level messageId (replay anchor) — NOT data.messageId which
          // is the Claude API message ID and does not match the replay buffer keys.
          if (messageId) {
            lastMessageIdRef.current = messageId;
          }

          setMessages(prev => {
            const streamIdx = prev.findIndex(m => m.id === STREAMING_MESSAGE_ID);
            if (streamIdx === -1) {
              const placeholder: Message = {
                id: STREAMING_MESSAGE_ID,
                role: 'assistant',
                content: payload.token,
                sessionID: sessionID,
                createdAt: new Date().toISOString(),
              };
              return [...prev, placeholder];
            }
            const updated = [...prev];
            const existing = updated[streamIdx]!;
            updated[streamIdx] = { ...existing, content: existing.content + payload.token };
            return updated;
          });
          break;
        }

        case 'chat.done': {
          const payload = data as {
            messageId: string;
            model?: string;
            usage?: { inputTokens: number; outputTokens: number; cacheReadInputTokens?: number };
          } | undefined;

          setMessages(prev =>
            prev.map(m =>
              m.id === STREAMING_MESSAGE_ID
                ? {
                    ...m,
                    id: payload?.messageId ?? generateTempId(),
                    ...(payload?.model ? { model: payload.model } : {}),
                    ...(payload?.usage ? { usage: payload.usage } : {}),
                  }
                : m,
            ),
          );
          setIsStreaming(false);
          if (sendTimeRef.current > 0) {
            const now = performance.now();
            const firstTokenMs = firstTokenTimeRef.current > 0
              ? firstTokenTimeRef.current - sendTimeRef.current
              : 0;
            const totalMs = now - sendTimeRef.current;
            reportWSLatency(Math.round(firstTokenMs), Math.round(totalMs));
            sendTimeRef.current = 0;
            firstTokenTimeRef.current = 0;
          }
          setAuthError(false);
          localStorage.removeItem('soul-v2-pending');
          break;
        }

        case 'chat.error': {
          const payload = data as { error: string } | undefined;
          const errorContent = payload?.error ?? 'An unknown error occurred';

          const errorLower = errorContent.toLowerCase();
          const isAuth = errorLower.includes('authentication') ||
                         errorLower.includes('unauthorized') ||
                         errorLower.includes('401');
          if (isAuth) {
            setAuthError(true);
            reportAuthFailure({
              source: 'api',
              reason: errorContent,
            });
          }

          setMessages(prev => {
            const hasStreaming = prev.some(m => m.id === STREAMING_MESSAGE_ID);
            if (hasStreaming) {
              return prev.map(m =>
                m.id === STREAMING_MESSAGE_ID
                  ? { ...m, id: generateTempId(), content: `Error: ${errorContent}` }
                  : m,
              );
            }
            return [
              ...prev,
              {
                id: generateTempId(),
                role: 'assistant' as const,
                content: `Error: ${errorContent}`,
                sessionID: sessionID,
                createdAt: new Date().toISOString(),
              },
            ];
          });
          setIsStreaming(false);
          break;
        }

        case 'tool.call': {
          const payload = data as { id: string; name: string; input: Record<string, unknown> } | undefined;
          if (!payload) break;

          const newTool: ToolCallData = {
            id: payload.id,
            name: payload.name,
            input: payload.input,
            status: 'running',
          };

          setMessages(prev => {
            const last = prev[prev.length - 1];
            if (last?.id === STREAMING_MESSAGE_ID) {
              const tools = [...(last.toolCalls ?? []), newTool];
              return [...prev.slice(0, -1), { ...last, toolCalls: tools }];
            }
            const placeholder: Message = {
              id: STREAMING_MESSAGE_ID,
              role: 'assistant',
              content: '',
              sessionID: sessionID,
              createdAt: new Date().toISOString(),
              toolCalls: [newTool],
            };
            return [...prev, placeholder];
          });
          break;
        }

        case 'tool.progress': {
          const payload = data as {
            id: string;
            event?: string;
            detail?: string;
            progress?: number;
            ts?: number;
          } | undefined;
          if (!payload) break;

          setMessages(prev => {
            const last = prev[prev.length - 1];
            if (last?.id !== STREAMING_MESSAGE_ID) return prev;
            const tools = [...(last.toolCalls ?? [])];
            const idx = tools.findIndex(t => t.id === payload.id);
            if (idx === -1) return prev;

            const tool = tools[idx]!;
            const updates: Partial<ToolCallData> = {};

            // Update numeric progress if provided.
            if (payload.progress !== undefined) {
              updates.progress = payload.progress;
            }

            // Append to steps if detail is provided.
            if (payload.detail || payload.event) {
              const step: ProgressStep = {
                event: (payload.event as ProgressStep['event']) ?? 'step',
                detail: payload.detail ?? '',
                progress: payload.progress,
                ts: payload.ts ?? Date.now(),
              };
              updates.steps = [...(tool.steps ?? []), step];
            }

            tools[idx] = { ...tool, ...updates };
            return [...prev.slice(0, -1), { ...last, toolCalls: tools }];
          });
          break;
        }

        case 'tool.complete': {
          const payload = data as { id: string; output: string } | undefined;
          if (!payload) break;

          setMessages(prev => {
            const last = prev[prev.length - 1];
            if (last?.id !== STREAMING_MESSAGE_ID) return prev;
            const tools = [...(last.toolCalls ?? [])];
            const idx = tools.findIndex(t => t.id === payload.id);
            if (idx === -1) return prev;
            tools[idx] = { ...tools[idx]!, status: 'complete', output: payload.output };
            return [...prev.slice(0, -1), { ...last, toolCalls: tools }];
          });
          break;
        }

        case 'tool.error': {
          const payload = data as { id: string; output: string } | undefined;
          if (!payload) break;

          setMessages(prev => {
            const last = prev[prev.length - 1];
            if (last?.id !== STREAMING_MESSAGE_ID) return prev;
            const tools = [...(last.toolCalls ?? [])];
            const idx = tools.findIndex(t => t.id === payload.id);
            if (idx === -1) return prev;
            tools[idx] = { ...tools[idx]!, status: 'error', output: payload.output };
            return [...prev.slice(0, -1), { ...last, toolCalls: tools }];
          });
          break;
        }

        default:
          break;
      }
    },
    [],
  );

  const { status, send, reconnectAttempt, reconnect, authError: wsAuthError } = useWebSocket({ onMessage: handleMessage });

  useEffect(() => {
    if (status === 'disconnected' || status === 'error') {
      window.dispatchEvent(new Event('ws:disconnected'));
    }
  }, [status]);

  // Store send in a ref so the connection.ready handler can use it.
  const sendRef = useRef(send);
  sendRef.current = send;

  const sendMessage = useCallback(
    (content: string, options?: { model?: string; thinking?: ThinkingConfig; attachments?: { name: string; mediaType: string; data: string }[] }) => {
      const trimmed = content.trim();
      if (!trimmed) return;

      // Deferred session creation: if no session, create one first then send.
      if (!sessionIDRef.current) {
        const pendingMessage = { content: trimmed, options };
        pendingMessageRef.current = pendingMessage;
        try {
          localStorage.setItem('soul-v2-pending', JSON.stringify(pendingMessage));
        } catch (err) { reportError('useChat.pendingSave', err); }
        send('session.create', {});
        return;
      }

      // Optimistic user message.
      const userMessage: Message = {
        id: generateTempId(),
        role: 'user',
        content: trimmed,
        sessionID: sessionIDRef.current,
        createdAt: new Date().toISOString(),
      };

      setMessages(prev => [...prev, userMessage]);
      setIsStreaming(true);

      // Retries and edits call sendMessage with no options — fall back to the
      // last options used so model and thinking are preserved across retries.
      const resolved = options ?? lastSentOptionsRef.current ?? undefined;
      if (options) lastSentOptionsRef.current = { model: options.model, thinking: options.thinking };

      const payload: Record<string, unknown> = { sessionId: sessionIDRef.current, content: trimmed };
      if (resolved?.model) payload.model = resolved.model;
      if (resolved?.thinking) payload.thinking = resolved.thinking;
      if (options?.attachments?.length) payload.attachments = options.attachments;
      sendTimeRef.current = performance.now();
      firstTokenTimeRef.current = 0;

      const messageId = sendQueueRef.current.enqueue({
        type: 'chat.send',
        ...payload,
      });

      if (status === 'connected') {
        sendQueueRef.current.flush((queuedPayload) => {
          const { type, ...data } = queuedPayload;
          send(type as string, data as Record<string, unknown>);
        });
      }

      reportUsage('chat.send', { model: options?.model, thinking: options?.thinking, hasAttachments: !!options?.attachments?.length, messageId });
    },
    [send, status],
  );

  // Pending message for deferred session creation.
  const pendingMessageRef = useRef<{ content: string; options?: { model?: string; thinking?: ThinkingConfig; attachments?: { name: string; mediaType: string; data: string }[] } } | null>(null);

  // After session.created, send any pending message.
  useEffect(() => {
    if (currentSessionID && pendingMessageRef.current) {
      const { content, options } = pendingMessageRef.current;
      pendingMessageRef.current = null;
      localStorage.removeItem('soul-v2-pending');
      // Small delay to ensure session is fully registered.
      setTimeout(() => sendMessage(content, options), 50);
    }
  }, [currentSessionID, sendMessage]);

  // Persist send queue to localStorage when disconnected so messages survive page reload.
  useEffect(() => {
    if (status === 'disconnected') {
      sendQueueRef.current.persist();
    }
  }, [status]);

  const stopGeneration = useCallback(() => {
    if (!sessionIDRef.current) return;
    send('chat.stop', { sessionId: sessionIDRef.current });
    setIsStreaming(false);
  }, [send]);

  const createSession = useCallback(() => {
    pendingExplicitCreateRef.current = true;
    send('session.create', {});
    reportUsage('session.create');
  }, [send]);

  const switchSession = useCallback(
    (id: string) => {
      if (id === sessionIDRef.current) return;
      sessionIDRef.current = id;
      setCurrentSessionID(id);
      localStorage.setItem(STORAGE_KEY, id);
      setMessages([]);
      setIsStreaming(false);
      send('session.switch', { sessionId: id });
      reportUsage('session.switch');
    },
    [send],
  );

  const deleteSession = useCallback(
    (id: string) => {
      send('session.delete', { sessionId: id });
      reportUsage('session.delete');
    },
    [send],
  );

  const renameSession = useCallback(
    (id: string, title: string) => {
      send('session.rename', { sessionId: id, content: title });
      reportUsage('session.rename');
    },
    [send],
  );

  const setProduct = useCallback((product: ChatProduct) => {
    if (!sessionIDRef.current) return;
    send('session.setProduct', {
      sessionId: sessionIDRef.current,
      product,
    });
    setActiveProduct(product);
  }, [send]);

  const reauth = useCallback(async () => {
    const MAX_RETRIES = 3;
    for (let attempt = 0; attempt < MAX_RETRIES; attempt++) {
      try {
        const token = getToken()?.trim();
        const resp = await fetch('/api/reauth', {
          method: 'POST',
          ...(token ? { headers: { Authorization: `Bearer ${token}` } } : {}),
        });
        if (resp.ok) {
          setAuthError(false);
          reconnect();   // also resets the WS circuit breaker
          return;
        }
      } catch (err) {
        reportError('useChat.reauth', err);
      }
      if (attempt < MAX_RETRIES - 1) {
        await new Promise(r => setTimeout(r, 1000 * Math.pow(2, attempt)));
      }
    }
    // All retries failed — keep authError true so UI shows re-auth button.
  }, [reconnect]);

  const editAndResend = useCallback((messageId: string, newContent: string) => {
    setMessages(prev => {
      const idx = prev.findIndex(m => m.id === messageId);
      if (idx === -1) return prev;
      return prev.slice(0, idx);
    });
    setTimeout(() => sendMessage(newContent), 50);
  }, [sendMessage]);

  const retryMessage = useCallback((messageId: string) => {
    const msg = messages.find(m => m.id === messageId);
    if (!msg || msg.role !== 'user') return;
    setMessages(prev => {
      const idx = prev.findIndex(m => m.id === messageId);
      if (idx === -1) return prev;
      return prev.slice(0, idx);
    });
    setTimeout(() => sendMessage(msg.content), 50);
  }, [messages, sendMessage]);

  return {
    messages,
    isStreaming,
    status,
    authError: authError || wsAuthError,
    reconnectAttempt,
    sendMessage,
    stopGeneration,
    editAndResend,
    retryMessage,
    reauth,
    reconnect,
    sessions,
    currentSessionID,
    createSession,
    switchSession,
    deleteSession,
    renameSession,
    activeProduct,
    setProduct,
  };
}
