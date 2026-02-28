interface ChatRailProps {
  unreadCount: number;
  onExpand: () => void;
}

export default function ChatRail({ unreadCount, onExpand }: ChatRailProps) {
  return (
    <button
      type="button"
      onClick={onExpand}
      className="w-10 h-full bg-zinc-950 border-r border-zinc-800 flex flex-col items-center py-3 gap-3 shrink-0 cursor-pointer hover:bg-zinc-900 transition-colors"
    >
      {/* Soul icon */}
      <span className="text-lg text-zinc-400">&#9670;</span>

      {/* Unread badge */}
      {unreadCount > 0 && (
        <span className="bg-sky-600 text-white text-[10px] font-bold rounded-full min-w-[18px] h-[18px] flex items-center justify-center px-1">
          {unreadCount > 9 ? '9+' : unreadCount}
        </span>
      )}

      <div className="flex-1" />

      {/* Chat bubble icon */}
      <span className="text-lg text-zinc-500" title="Expand chat">
        &#128172;
      </span>
    </button>
  );
}
