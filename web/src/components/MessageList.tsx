import { useEffect, useRef } from 'react';
import type { MessageListProps } from '../lib/types';
import { MessageBubble } from './MessageBubble';

export function MessageList({ messages, isStreaming }: MessageListProps) {
  const containerRef = useRef<HTMLDivElement>(null);

  // Auto-scroll to bottom when messages change or streaming content updates.
  useEffect(() => {
    const el = containerRef.current;
    if (el) {
      el.scrollTop = el.scrollHeight;
    }
  }, [messages, isStreaming]);

  return (
    <div
      ref={containerRef}
      data-testid="message-list"
      className="flex-1 overflow-y-auto px-4 py-4"
    >
      {messages.length === 0 ? (
        <div className="flex items-center justify-center h-full text-zinc-500">
          No messages yet
        </div>
      ) : (
        <div className="flex flex-col gap-4">
          {messages.map((msg, idx) => {
            const isLastMessage = idx === messages.length - 1;
            const isStreamingMessage =
              isStreaming && isLastMessage && msg.role === 'assistant';
            return (
              <MessageBubble
                key={msg.id}
                message={msg}
                isStreaming={isStreamingMessage}
              />
            );
          })}
        </div>
      )}
    </div>
  );
}
