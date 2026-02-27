import { useState, useEffect, useCallback, useRef } from 'react';
import { useWebSocket } from './useWebSocket.ts';
import type { ChatMessage, ToolCallMessage, FindingMessage, WSMessage } from '../lib/types.ts';

export function useChat() {
  const { send, onMessage, connected } = useWebSocket();
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [isStreaming, setIsStreaming] = useState(false);
  const sessionIdRef = useRef<string>(crypto.randomUUID());

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
                id: crypto.randomUUID(),
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
                id: crypto.randomUUID(),
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
          const data = msg.data as {
            tool_call_id: string;
            finding: FindingMessage;
          };
          setMessages((prev) =>
            updateLastAssistantToolCall(prev, data.tool_call_id, (tc) => ({
              ...tc,
              findings: [...(tc.findings ?? []), data.finding],
            })),
          );
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
              id: crypto.randomUUID(),
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
    (content: string) => {
      const userMessage: ChatMessage = {
        id: crypto.randomUUID(),
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
