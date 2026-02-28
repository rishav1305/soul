interface ChatNavbarProps {
  onToggleDrawer: () => void;
  onCollapse: () => void;
  canCollapse: boolean;
}

export default function ChatNavbar({ onToggleDrawer, onCollapse, canCollapse }: ChatNavbarProps) {
  return (
    <div className="glass flex items-center gap-2 h-11 px-4 shrink-0">
      <button
        type="button"
        onClick={onToggleDrawer}
        className="text-fg-muted hover:text-fg text-lg cursor-pointer"
        title="Sessions"
      >
        &#9776;
      </button>

      <span className="font-display text-sm font-semibold text-fg flex items-center gap-2">
        <span className="relative">
          <span className="absolute inset-0 -m-1 bg-soul/15 rounded-full blur-md animate-soul-pulse" />
          <span className="relative text-xl text-soul">&#9670;</span>
        </span>
        Soul
      </span>

      <div className="flex-1" />

      <button
        type="button"
        onClick={onCollapse}
        disabled={!canCollapse}
        className="w-7 h-7 flex items-center justify-center rounded hover:bg-elevated text-fg-muted hover:text-fg disabled:opacity-20 disabled:cursor-not-allowed transition-colors cursor-pointer"
        title={canCollapse ? 'Collapse chat' : 'Cannot collapse — task panel is collapsed'}
      >
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
          <path d="M10 3L5 8l5 5" />
        </svg>
      </button>
    </div>
  );
}
