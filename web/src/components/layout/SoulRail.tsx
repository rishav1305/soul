interface SoulRailProps {
  onExpand: () => void;
}

export default function SoulRail({ onExpand }: SoulRailProps) {
  return (
    <div className="w-10 h-full bg-surface border-r border-border-subtle flex flex-col items-center py-3 gap-3 shrink-0">
      {/* Soul logo */}
      <span className="relative">
        <span className="absolute inset-0 -m-1 bg-soul/10 rounded-full blur-sm animate-soul-pulse" />
        <span className="relative text-4xl text-soul">&#9670;</span>
      </span>

      <div className="w-5 border-t border-border-subtle" />

      {/* Chat indicator */}
      <button type="button" className="w-7 h-7 flex items-center justify-center rounded text-fg-muted hover:text-fg hover:bg-elevated transition-colors cursor-pointer" title="Chat">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
          <path d="M2 3h12v8H5l-3 3V3z" />
        </svg>
      </button>

      {/* Tasks shortcut */}
      <button type="button" className="w-7 h-7 flex items-center justify-center rounded text-fg-muted hover:text-fg hover:bg-elevated transition-colors cursor-pointer" title="Tasks">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
          <path d="M3 4l2 2 4-4" />
          <path d="M3 10l2 2 4-4" />
        </svg>
      </button>

      <div className="flex-1" />

      {/* Settings */}
      <button type="button" className="w-7 h-7 flex items-center justify-center rounded text-fg-muted hover:text-fg hover:bg-elevated transition-colors cursor-pointer" title="Settings">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
          <circle cx="8" cy="8" r="2" />
          <path d="M8 1v2M8 13v2M1 8h2M13 8h2M3.05 3.05l1.41 1.41M11.54 11.54l1.41 1.41M3.05 12.95l1.41-1.41M11.54 4.46l1.41-1.41" />
        </svg>
      </button>

      {/* Expand — VS Code sidebar-left icon */}
      <button
        type="button"
        onClick={onExpand}
        className="w-7 h-7 flex items-center justify-center rounded hover:bg-elevated text-fg-muted hover:text-fg transition-colors cursor-pointer"
        title="Expand sidebar"
      >
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.2">
          <rect x="1.5" y="2.5" width="13" height="11" rx="1.5" />
          <line x1="5.5" y1="2.5" x2="5.5" y2="13.5" />
        </svg>
      </button>
    </div>
  );
}
