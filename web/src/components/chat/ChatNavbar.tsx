interface ChatNavbarProps {
  onToggleDrawer: () => void;
  onCollapse: () => void;
  canCollapse: boolean;
}

export default function ChatNavbar({ onToggleDrawer, onCollapse, canCollapse }: ChatNavbarProps) {
  return (
    <div className="flex items-center gap-2 px-3 h-10 border-b border-zinc-800 shrink-0">
      <button
        type="button"
        onClick={onToggleDrawer}
        className="text-zinc-500 hover:text-zinc-300 text-lg cursor-pointer"
        title="Sessions"
      >
        &#9776;
      </button>

      <span className="text-sm font-semibold text-zinc-100 flex items-center gap-1.5">
        <span className="text-zinc-400">&#9670;</span> Soul Chat
      </span>

      <div className="flex-1" />

      <button
        type="button"
        onClick={onCollapse}
        disabled={!canCollapse}
        className="text-zinc-500 hover:text-zinc-300 disabled:opacity-30 disabled:cursor-not-allowed text-sm font-mono cursor-pointer"
        title={canCollapse ? 'Collapse chat' : 'Cannot collapse — task panel is collapsed'}
      >
        [&minus;]
      </button>
    </div>
  );
}
