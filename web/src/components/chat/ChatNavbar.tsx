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

      <span className="font-display text-sm font-semibold text-fg flex items-center gap-1.5">
        <span className="text-soul">&#9670;</span> Soul Chat
      </span>

      <div className="flex-1" />

      <button
        type="button"
        onClick={onCollapse}
        disabled={!canCollapse}
        className="text-fg-muted hover:text-fg-secondary disabled:opacity-20 disabled:cursor-not-allowed text-sm font-mono cursor-pointer"
        title={canCollapse ? 'Collapse chat' : 'Cannot collapse — task panel is collapsed'}
      >
        [&minus;]
      </button>
    </div>
  );
}
