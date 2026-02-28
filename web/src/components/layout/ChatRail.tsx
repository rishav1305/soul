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
      <span className="text-lg text-fg-muted" title="Expand chat">
        &#128172;
      </span>
    </button>
  );
}
