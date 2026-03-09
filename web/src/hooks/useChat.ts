import { useState, useCallback, useRef } from 'react';
import type { Message, OutboundMessageType, ConnectionState } from '../lib/types';
import { useWebSocket } from './useWebSocket';

interface UseChatReturn {
  messages: Message[];
  isStreaming: boolean;
  status: ConnectionState;
  sendMessage: (content: string) => void;
  currentSessionID: string | null;
  createSession: () => void;
  switchSession: (id: string) => void;
}

const STREAMING_MESSAGE_ID = '__streaming__';

function generateTempId(): string {
  return `temp-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
}

export function useChat(): UseChatReturn {
  const [messages, setMessages] = useState<Message[]>([]);
  const [isStreaming, setIsStreaming] = useState(false);
  const [currentSessionID, setCurrentSessionID] = useState<string | null>(null);

  // Track session ID in a ref so the onMessage callback always sees the latest value
  // without needing to re-create (which would cause useWebSocket to reconnect).
  const sessionIDRef = useRef<string | null>(null);
  const sessionCreationRequestedRef = useRef(false);

  const handleMessage = useCallback(
    (type: OutboundMessageType, data: unknown, sessionID: string) => {
      switch (type) {
        case 'connection.ready': {
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
          const payload = data as { session: { id: string } } | undefined;
          if (payload?.session?.id) {
            const id = payload.session.id;
            sessionIDRef.current = id;
            setCurrentSessionID(id);
            setMessages([]);
            sessionCreationRequestedRef.current = false;
          }
          break;
        }

        case 'session.updated': {
          // Currently a no-op; session metadata updates handled in future steps.
          break;
        }

        case 'session.list': {
          // Will be used by SessionList in Step 5.7. Ignore for now.
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
          break;
        }

        case 'chat.error': {
          const payload = data as { error: string } | undefined;
          const errorContent = payload?.error ?? 'An unknown error occurred';

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

  return {
    messages,
    isStreaming,
    status,
    sendMessage,
    currentSessionID,
    createSession,
    switchSession,
  };
}
