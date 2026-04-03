import type { TeamChatMessage } from '../../hooks/useTeamChat.ts';

/** ChatMessage — message bubble with avatar and sender info. */

interface ChatMessageProps {
  message: TeamChatMessage;
}

const AGENT_COLOR: Record<string, string> = {
  fury:    'bg-red-700',
  shuri:   'bg-purple-700',
  happy:   'bg-yellow-700',
  xavier:  'bg-blue-700',
  pepper:  'bg-pink-700',
  loki:    'bg-green-700',
  hawkeye: 'bg-orange-700',
  stark:   'bg-cyan-700',
  banner:  'bg-emerald-700',
  CEO:     'bg-soul/80',
  system:  'bg-zinc-700',
};

function avatarInitials(name: string): string {
  return name.slice(0, 2).toUpperCase();
}

function formatTimestamp(ts: number): string {
  return new Date(ts).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

export function ChatMessage({ message }: ChatMessageProps) {
  const isSystem = message.sender === 'system';
  const bg = AGENT_COLOR[message.sender] ?? 'bg-zinc-700';

  if (isSystem) {
    return (
      <div
        data-testid={`chat-message-system-${message.id}`}
        className="flex justify-center my-2"
      >
        <span className="text-[10px] text-fg-muted bg-surface px-3 py-1 rounded-full border border-border-subtle">
          {message.content}
        </span>
      </div>
    );
  }

  return (
    <div
      data-testid={`chat-message-${message.id}`}
      className="flex items-start gap-3 px-4 py-2 hover:bg-surface/50 transition-colors"
    >
      {/* Avatar */}
      <div
        aria-hidden="true"
        className={`shrink-0 w-7 h-7 rounded-full flex items-center justify-center text-[10px] font-bold text-white ${bg}`}
      >
        {avatarInitials(message.sender)}
      </div>

      {/* Body */}
      <div className="flex-1 min-w-0">
        <div className="flex items-baseline gap-2 mb-0.5">
          <span className="text-xs font-semibold text-fg">{message.sender}</span>
          <time
            dateTime={new Date(message.timestamp).toISOString()}
            className="text-[10px] text-fg-muted"
          >
            {formatTimestamp(message.timestamp)}
          </time>
        </div>
        <p className="text-sm text-fg-secondary leading-relaxed whitespace-pre-wrap break-words">
          {message.content}
        </p>
      </div>
    </div>
  );
}
