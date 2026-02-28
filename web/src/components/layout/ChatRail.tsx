interface ChatRailProps {
  unreadCount: number;
  onExpand: () => void;
}

export default function ChatRail({ unreadCount, onExpand }: ChatRailProps) {
  return (
    <button
      type="button"
      onClick={onExpand}
      className="w-10 h-full bg-surface border-r border-border-subtle flex flex-col items-center py-3 gap-3 shrink-0 cursor-pointer hover:bg-elevated transition-colors"
    >
      {/* Soul icon */}
      <span className="text-lg text-soul">&#9670;</span>

      {/* Unread badge */}
      {unreadCount > 0 && (
        <span className="bg-soul-dim text-soul text-[10px] font-bold rounded-full min-w-[18px] h-[18px] flex items-center justify-center px-1">
          {unreadCount > 9 ? '9+' : unreadCount}
        </span>
      )}

      <div className="flex-1" />

      {/* Chat bubble icon */}
      <button type="button" className="w-7 h-7 flex items-center justify-center rounded text-fg-muted hover:text-fg hover:bg-elevated transition-colors" title="Expand chat">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.2">
          <rect x="1.5" y="2.5" width="13" height="11" rx="1.5" />
          <line x1="5.5" y1="2.5" x2="5.5" y2="13.5" />
        </svg>
      </button>
    </button>
  );
}
