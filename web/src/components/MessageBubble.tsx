import type { MessageBubbleProps } from '../lib/types';
import { Markdown } from './Markdown';

export function MessageBubble({ message, isStreaming }: MessageBubbleProps) {
  const isUser = message.role === 'user';

  return (
    <div
      data-testid="message-bubble"
      className={`flex flex-col max-w-[80%] ${isUser ? 'self-end items-end' : 'self-start items-start'}`}
    >
      <span className="text-xs text-zinc-500 mb-1 px-1">
        {isUser ? 'You' : 'Assistant'}
      </span>
      <div
        className={`rounded-lg px-4 py-3 ${isUser ? 'bg-zinc-800' : 'bg-zinc-900'}`}
      >
        {isUser ? (
          <p className="whitespace-pre-wrap break-words text-zinc-100">
            {message.content}
          </p>
        ) : (
          <div className="relative">
            <Markdown content={message.content} />
            {isStreaming && (
              <span className="animate-pulse inline">&#9612;</span>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
