import { useState, useEffect, useCallback, useRef } from 'react';
import { useWebSocket } from './useWebSocket.ts';
import type { ChatMessage, ToolCallMessage, WSMessage, SendOptions } from '../lib/types.ts';

function uuid(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  // Fallback for non-secure contexts (HTTP, non-localhost)
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;
  const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

export function useChat() {
  const { send, onMessage, connected } = useWebSocket();
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [isStreaming, setIsStreaming] = useState(false);
  // DB integer session ID — null means no session created yet
  const [sessionId, setSessionId] = useState<number | null>(null);
  // Keep a ref for stable access in callbacks without triggering re-renders
  const sessionIdRef = useRef<number | null>(null);
  const isStreamingRef = useRef(false);

  // Keep refs in sync with state
  useEffect(() => {
    sessionIdRef.current = sessionId;
  }, [sessionId]);

  useEffect(() => {
    isStreamingRef.current = isStreaming;
  }, [isStreaming]);

  // When sessionId changes (e.g., session switch from SessionDrawer),
  // load the full message history from the API.
  // Skip hydration while streaming — session.created fires mid-stream and we
  // must not overwrite the live in-progress messages with a DB snapshot.
  useEffect(() => {
    if (!sessionId || isStreamingRef.current) return;
    fetch(`/api/sessions/${sessionId}/messages`)
      .then((r) => r.json())
      .then((records: Array<{ id: number; role: string; content: string }>) => {
        const hydrated: ChatMessage[] = records.map((r) => ({
          id: String(r.id),
          role: r.role as 'user' | 'assistant',
          content: r.content,
          toolCalls: [],
          timestamp: new Date(),
        }));
        setMessages(hydrated);
      })
      .catch(() => {});
  }, [sessionId]);

  useEffect(() => {
    const unsubscribe = onMessage((msg: WSMessage) => {
      switch (msg.type) {
        case 'session.created': {
          // Server assigned a DB session ID to the current conversation.
          const data = msg.data as { session_id: number };
          setSessionId(data.session_id);
          break;
        }

        case 'chat.token': {
          const token = msg.content ?? '';
          setMessages((prev) => {
            const last = prev[prev.length - 1];
            if (last && last.role === 'assistant') {
              return [
                ...prev.slice(0, -1),
                { ...last, content: last.content + token },
              ];
            }
            // Create new assistant message if none exists
            return [
              ...prev,
              {
                id: uuid(),
                role: 'assistant',
                content: token,
                toolCalls: [],
                timestamp: new Date(),
              },
            ];
          });
          break;
        }

        case 'chat.thinking': {
          const token = msg.content ?? '';
          setMessages((prev) => {
            const last = prev[prev.length - 1];
            if (last && last.role === 'assistant') {
              return [
                ...prev.slice(0, -1),
                { ...last, thinking: (last.thinking ?? '') + token },
              ];
            }
            // Create a new assistant message shell for thinking content
            return [
              ...prev,
              {
                id: uuid(),
                role: 'assistant' as const,
                content: '',
                thinking: token,
                toolCalls: [],
                timestamp: new Date(),
              },
            ];
          });
          break;
        }

        case 'chat.done': {
          setIsStreaming(false);
          break;
        }

        case 'tool.call': {
          const data = msg.data as {
            id: string;
            name: string;
            input: unknown;
          };
          const toolCall: ToolCallMessage = {
            id: data.id,
            name: data.name,
            input: data.input,
            status: 'running',
            findings: [],
          };
          setMessages((prev) => {
            const last = prev[prev.length - 1];
            if (last && last.role === 'assistant') {
              return [
                ...prev.slice(0, -1),
                {
                  ...last,
                  toolCalls: [...(last.toolCalls ?? []), toolCall],
                },
              ];
            }
            return [
              ...prev,
              {
                id: uuid(),
                role: 'assistant',
                content: '',
                toolCalls: [toolCall],
                timestamp: new Date(),
              },
            ];
          });
          break;
        }

        case 'tool.progress': {
          const data = msg.data as { id: string; progress: number };
          setMessages((prev) =>
            updateAssistantToolCall(prev, data.id, (tc) => ({
              ...tc,
              progress: data.progress,
            })),
          );
          break;
        }

        case 'tool.finding': {
          // Findings are handled by useScanResult and shown in the side panel.
          break;
        }

        case 'tool.complete': {
          const data = msg.data as { id: string; output: string };
          setMessages((prev) =>
            updateAssistantToolCall(prev, data.id, (tc) => ({
              ...tc,
              status: 'complete',
              progress: 100,
              output: data.output,
            })),
          );
          break;
        }

        case 'error': {
          const errorContent = msg.content ?? 'An error occurred';
          setMessages((prev) => [
            ...prev,
            {
              id: uuid(),
              role: 'assistant',
              content: `Error: ${errorContent}`,
              timestamp: new Date(),
            },
          ]);
          setIsStreaming(false);
          break;
        }
      }
    });

    return unsubscribe;
  }, [onMessage]);

  const sendMessage = useCallback(
    (content: string, options?: SendOptions) => {
      const userMessage: ChatMessage = {
        id: uuid(),
        role: 'user',
        content,
        timestamp: new Date(),
      };
      setMessages((prev) => [...prev, userMessage]);
      setIsStreaming(true);

      send({
        type: 'chat.message',
        // Send DB session ID as string if we have one, otherwise 'new' to
        // signal the server to create a fresh session.
        session_id: sessionIdRef.current ? String(sessionIdRef.current) : 'new',
        content,
        data: options,
      });
    },
    [send],
  );

  return { messages, setMessages, sendMessage, isStreaming, connected, sessionId, setSessionId };
}

function updateAssistantToolCall(
  messages: ChatMessage[],
  toolCallId: string,
  updater: (tc: ToolCallMessage) => ToolCallMessage,
): ChatMessage[] {
  // Search all messages in reverse for the matching tool call
  for (let i = messages.length - 1; i >= 0; i--) {
    const msg = messages[i];
    if (msg.role !== 'assistant' || !msg.toolCalls) continue;
    const idx = msg.toolCalls.findIndex(tc => tc.id === toolCallId);
    if (idx === -1) continue;
    // Found it — update this message
    const newToolCalls = [...msg.toolCalls];
    newToolCalls[idx] = updater(newToolCalls[idx]);
    const updated = [...messages];
    updated[i] = { ...msg, toolCalls: newToolCalls };
    return updated;
  }
  return messages; // not found, no change
}
