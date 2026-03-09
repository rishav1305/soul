import { useState, useCallback, useRef } from 'react';
import type { Message, Session, OutboundMessageType, ConnectionState } from '../lib/types';
import { useWebSocket } from './useWebSocket';

interface UseChatReturn {
  messages: Message[];
  isStreaming: boolean;
  status: ConnectionState;
  authError: boolean;
  sendMessage: (content: string) => void;
  reauth: () => Promise<void>;
  sessions: Session[];
  currentSessionID: string | null;
  createSession: () => void;
  switchSession: (id: string) => void;
  deleteSession: (id: string) => void;
}

const STREAMING_MESSAGE_ID = '__streaming__';

function generateTempId(): string {
  return `temp-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
}

export function useChat(): UseChatReturn {
  const [messages, setMessages] = useState<Message[]>([]);
  const [isStreaming, setIsStreaming] = useState(false);
  const [authError, setAuthError] = useState(false);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [currentSessionID, setCurrentSessionID] = useState<string | null>(null);

  // Track session ID in a ref so the onMessage callback always sees the latest value
  // without needing to re-create (which would cause useWebSocket to reconnect).
  const sessionIDRef = useRef<string | null>(null);
  const sessionsRef = useRef<Session[]>([]);
  const sessionCreationRequestedRef = useRef(false);

  const handleMessage = useCallback(
    (type: OutboundMessageType, data: unknown, sessionID: string) => {
      switch (type) {
        case 'connection.ready': {
          // Clear auth error on reconnect.
          setAuthError(false);
          // Auto-create a session on first connect if none exists.
          if (!sessionIDRef.current && !sessionCreationRequestedRef.current) {
            sessionCreationRequestedRef.current = true;
            // sendRef will be set by the time messages arrive; we use a
            // micro-task to ensure the send function is available.
            queueMicrotask(() => {
              sendRef.current('session.create', {});
            });
          }
          break;
        }

        case 'session.created': {
          const payload = data as { session: Session } | undefined;
          if (payload?.session?.id) {
            const newSession = payload.session;
            sessionIDRef.current = newSession.id;
            setCurrentSessionID(newSession.id);
            setMessages([]);
            sessionCreationRequestedRef.current = false;

            // Add to sessions list if not already present.
            const alreadyExists = sessionsRef.current.some(s => s.id === newSession.id);
            if (!alreadyExists) {
              const updated = [newSession, ...sessionsRef.current];
              sessionsRef.current = updated;
              setSessions(updated);
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
                setMessages([]);
                sendRef.current('session.switch', { sessionId: next.id });
              } else {
                sessionIDRef.current = null;
                setCurrentSessionID(null);
                setMessages([]);
              }
            }
          }
          break;
        }

        case 'chat.token': {
          const payload = data as { token: string; messageId: string } | undefined;
          if (!payload) break;

          setMessages(prev => {
            const streamIdx = prev.findIndex(m => m.id === STREAMING_MESSAGE_ID);
            if (streamIdx === -1) {
              // First token — create the streaming placeholder.
              const placeholder: Message = {
                id: STREAMING_MESSAGE_ID,
                role: 'assistant',
                content: payload.token,
                sessionID: sessionID,
                createdAt: new Date().toISOString(),
              };
              return [...prev, placeholder];
            }

            // Append token to existing streaming message.
            const updated = [...prev];
            const existing = updated[streamIdx]!;
            updated[streamIdx] = { ...existing, content: existing.content + payload.token };
            return updated;
          });
          break;
        }

        case 'chat.done': {
          const payload = data as { messageId: string } | undefined;

          setMessages(prev =>
            prev.map(m =>
              m.id === STREAMING_MESSAGE_ID
                ? { ...m, id: payload?.messageId ?? generateTempId() }
                : m,
            ),
          );
          setIsStreaming(false);
          setAuthError(false);
          break;
        }

        case 'chat.error': {
          const payload = data as { error: string } | undefined;
          const errorContent = payload?.error ?? 'An unknown error occurred';

          if (errorContent.toLowerCase().includes('authentication')) {
            setAuthError(true);
          }

          setMessages(prev => {
            // If there's a streaming message, replace it with the error.
            const hasStreaming = prev.some(m => m.id === STREAMING_MESSAGE_ID);
            if (hasStreaming) {
              return prev.map(m =>
                m.id === STREAMING_MESSAGE_ID
                  ? { ...m, id: generateTempId(), content: `Error: ${errorContent}` }
                  : m,
              );
            }
            // Otherwise append an error message.
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

        default:
          break;
      }
    },
    [],
  );

  const { status, send } = useWebSocket({ onMessage: handleMessage });

  // Store send in a ref so the connection.ready handler can use it.
  const sendRef = useRef(send);
  sendRef.current = send;

  const sendMessage = useCallback(
    (content: string) => {
      const trimmed = content.trim();
      if (!trimmed || !sessionIDRef.current) return;

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

      send('chat.send', { sessionId: sessionIDRef.current, content: trimmed });
    },
    [send],
  );

  const createSession = useCallback(() => {
    send('session.create', {});
  }, [send]);

  const switchSession = useCallback(
    (id: string) => {
      sessionIDRef.current = id;
      setCurrentSessionID(id);
      setMessages([]);
      send('session.switch', { sessionId: id });
    },
    [send],
  );

  const deleteSession = useCallback(
    (id: string) => {
      send('session.delete', { sessionId: id });
    },
    [send],
  );

  const reauth = useCallback(async () => {
    try {
      await fetch('/api/reauth', { method: 'POST' });
      setAuthError(false);
    } catch {
      // Silently ignore — the next chat.error will re-set authError if still failing.
    }
  }, []);

  return {
    messages,
    isStreaming,
    status,
    authError,
    sendMessage,
    reauth,
    sessions,
    currentSessionID,
    createSession,
    switchSession,
    deleteSession,
  };
}
