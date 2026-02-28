interface ChatNavbarProps {
  onCollapse: () => void;
  canCollapse: boolean;
}

export default function ChatNavbar({ onCollapse, canCollapse }: ChatNavbarProps) {
  return (
    <div className="glass flex items-center gap-2 h-11 px-4 shrink-0">
      <span className="font-display text-sm font-semibold text-fg flex items-center gap-2">
        <span className="relative">
          <span className="absolute inset-0 -m-1 bg-soul/15 rounded-full blur-md animate-soul-pulse" />
          <span className="relative text-xl text-soul">&#9670;</span>
        </span>
        Chat
      </span>

      <div className="flex-1" />

      <button
        type="button"
        onClick={onCollapse}
        disabled={!canCollapse}
        className="w-7 h-7 flex items-center justify-center rounded hover:bg-elevated text-fg-muted hover:text-fg disabled:opacity-20 disabled:cursor-not-allowed transition-colors cursor-pointer"
        title={canCollapse ? 'Collapse chat' : 'Cannot collapse — task panel is collapsed'}
      >
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.2">
          <rect x="1.5" y="2.5" width="13" height="11" rx="1.5" />
          <line x1="5.5" y1="2.5" x2="5.5" y2="13.5" />
        </svg>
      </button>
    </div>
  );
}
