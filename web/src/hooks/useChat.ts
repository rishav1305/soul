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
  const sessionIdRef = useRef<string>(uuid());

  useEffect(() => {
    const unsubscribe = onMessage((msg: WSMessage) => {
      switch (msg.type) {
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
            updateLastAssistantToolCall(prev, data.id, (tc) => ({
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
            updateLastAssistantToolCall(prev, data.id, (tc) => ({
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
        session_id: sessionIdRef.current,
        content,
        data: options,
      });
    },
    [send],
  );

  return { messages, sendMessage, isStreaming, connected };
}

function updateLastAssistantToolCall(
  messages: ChatMessage[],
  toolCallId: string,
  updater: (tc: ToolCallMessage) => ToolCallMessage,
): ChatMessage[] {
  const last = messages[messages.length - 1];
  if (!last || last.role !== 'assistant' || !last.toolCalls) return messages;

  const updatedToolCalls = last.toolCalls.map((tc) =>
    tc.id === toolCallId ? updater(tc) : tc,
  );

  return [
    ...messages.slice(0, -1),
    { ...last, toolCalls: updatedToolCalls },
  ];
}
