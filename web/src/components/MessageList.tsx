import { useCallback, useEffect, useRef, useState } from 'react';
import type { MessageListProps } from '../lib/types';
import { MessageBubble } from './MessageBubble';

const SUGGESTIONS = [
  'Explain this codebase',
  'Find bugs in my code',
  'Write a test for...',
  'Refactor this function',
];

export function MessageList({ messages, isStreaming, onSend, onEdit, onRetry, searchQuery }: MessageListProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [showScrollBtn, setShowScrollBtn] = useState(false);

  const handleScroll = useCallback(() => {
    const el = containerRef.current;
    if (!el) return;
    const isNearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 80;
    setShowScrollBtn(!isNearBottom);
  }, []);

  const scrollToBottom = useCallback(() => {
    containerRef.current?.scrollTo({ top: containerRef.current.scrollHeight, behavior: 'smooth' });
  }, []);

  // Auto-scroll to bottom only when user is near bottom.
  useEffect(() => {
    const el = containerRef.current;
    if (el && !showScrollBtn) {
      el.scrollTop = el.scrollHeight;
    }
  }, [messages, isStreaming, showScrollBtn]);

  if (messages.length === 0) {
    return (
      <div
        data-testid="message-list"
        className="flex-1 flex flex-col items-center justify-center gap-8 px-4"
      >
        <div className="flex flex-col items-center gap-3">
          <span className="text-5xl animate-float-glow text-soul">
            &#9670;
          </span>
          <h1 className="text-lg font-semibold text-fg tracking-tight">What can I help you with?</h1>
          <p className="text-sm text-fg-muted text-center max-w-sm">
            Ask me anything about your codebase, or pick a suggestion below.
          </p>
        </div>
        <div className="grid grid-cols-2 gap-2.5 max-w-md w-full">
          {SUGGESTIONS.map((prompt) => (
            <button
              key={prompt}
              onClick={() => onSend?.(prompt)}
              data-testid="suggestion-button"
              className="group px-4 py-3 text-sm text-fg-muted bg-surface hover:bg-elevated rounded-xl border border-border-subtle hover:border-soul/30 hover:text-fg transition-all text-left"
            >
              <span className="opacity-70 group-hover:opacity-100 transition-opacity">{prompt}</span>
            </button>
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="relative flex-1 min-h-0">
      <div
        ref={containerRef}
        data-testid="message-list"
        className="h-full overflow-y-auto px-4 py-4"
        onScroll={handleScroll}
      >
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
                onEdit={onEdit}
                onRetry={onRetry}
                searchQuery={searchQuery}
              />
            );
          })}
        </div>
      </div>
      {isStreaming && <div className="stream-bar shrink-0" />}
      {showScrollBtn && (
        <button
          onClick={scrollToBottom}
          data-testid="scroll-to-bottom"
          className="absolute bottom-4 right-4 w-10 h-10 bg-elevated hover:bg-overlay border border-border-default rounded-full flex items-center justify-center text-fg-muted hover:text-fg shadow-lg transition-all z-10"
          aria-label="Scroll to bottom"
        >
          &#8595;
        </button>
      )}
    </div>
  );
}
