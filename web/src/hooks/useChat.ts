import { useState, useEffect, useCallback, useRef } from 'react';
import { useWebSocket } from './useWebSocket.ts';
import { uuid } from '../lib/api.ts';
import type { ChatMessage, ToolCallMessage, WSMessage, SendOptions, TokenUsage } from '../lib/types.ts';

export function useChat() {
  const { send, onMessage, connected } = useWebSocket();
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [isStreaming, setIsStreaming] = useState(false);
  const [tokenUsage, setTokenUsage] = useState<TokenUsage | null>(null);
  // DB integer session ID — null means no session created yet
  const [sessionId, setSessionId] = useState<number | null>(null);
  // Keep a ref for stable access in callbacks without triggering re-renders
  const sessionIdRef = useRef<number | null>(null);
  const isStreamingRef = useRef(false);
  const lastModelRef = useRef<string | undefined>(undefined);

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
                model: lastModelRef.current,
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
                model: lastModelRef.current,
              },
            ];
          });
          break;
        }

        case 'chat.done': {
          const data = msg.data as { input_tokens?: number; output_tokens?: number; context_pct?: number } | undefined;
          if (data?.input_tokens || data?.output_tokens) {
            setTokenUsage({
              inputTokens: data.input_tokens ?? 0,
              outputTokens: data.output_tokens ?? 0,
              contextPct: data.context_pct ?? 0,
            });
          }
          setIsStreaming(false);
          break;
        }

        case 'pm.notification': {
          const data = msg.data as { severity: string; task_ids: number[]; check: string };
          setMessages((prev) => [
            ...prev,
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
          ]);
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
          setMessages((prev) => {
            const updated = updateAssistantToolCall(prev, data.id, (tc) => ({
              ...tc,
              status: 'complete',
              progress: 100,
              output: data.output,
            }));
            // Insert paragraph break at the seam between tool output and
            // upcoming text so they don't run together (e.g. "file.The").
            // Done here once per tool, NOT per token.
            const last = updated[updated.length - 1];
            if (last?.role === 'assistant' && last.content && !last.content.endsWith('\n')) {
              return [
                ...updated.slice(0, -1),
                { ...last, content: last.content + '\n\n' },
              ];
            }
            return updated;
          });
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
      lastModelRef.current = options?.model;
      const userMessage: ChatMessage = {
        id: uuid(),
        role: 'user',
        content,
        timestamp: new Date(),
        model: options?.model,
      };
      setMessages((prev) => [...prev, userMessage]);
      setIsStreaming(true);
      setTokenUsage(null);

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

  const retryFromMessage = useCallback(
    (messageId: string) => {
      const msg = messages.find((m) => m.id === messageId && m.role === 'user');
      if (!msg) return;
      const idx = messages.indexOf(msg);
      setMessages(messages.slice(0, idx));
      setTimeout(() => sendMessage(msg.content), 0);
    },
    [messages, sendMessage],
  );

  const editMessage = useCallback(
    (messageId: string, newContent: string) => {
      const idx = messages.findIndex((m) => m.id === messageId && m.role === 'user');
      if (idx === -1) return;
      setMessages(messages.slice(0, idx));
      setTimeout(() => sendMessage(newContent), 0);
    },
    [messages, sendMessage],
  );

  const stopStreaming = useCallback(() => {
    if (!isStreamingRef.current) return;
    send({ type: 'chat.stop' });
    setIsStreaming(false);
  }, [send]);

  return { messages, setMessages, sendMessage, isStreaming, connected, sessionId, setSessionId, tokenUsage, retryFromMessage, editMessage, stopStreaming };
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
