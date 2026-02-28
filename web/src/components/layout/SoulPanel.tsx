import type { ChatSession } from '../../lib/types.ts';

interface SoulPanelProps {
  onCollapse: () => void;
  sessions: ChatSession[];
  activeSessionId: number | null;
  onSessionSelect: (id: number) => void;
  onNewChat: () => void;
  connected: boolean;
}

export default function SoulPanel({
  onCollapse,
  sessions,
  activeSessionId,
  onSessionSelect,
  onNewChat,
  connected,
}: SoulPanelProps) {
  return (
    <div className="w-60 h-full bg-surface border-r border-border-subtle flex flex-col shrink-0">
      {/* Header */}
      <div className="glass flex items-center gap-2 h-11 px-3 shrink-0">
        <span className="relative">
          <span className="absolute inset-0 -m-0.5 bg-soul/10 rounded-full blur-sm animate-soul-pulse" />
          <span className="relative text-lg text-soul">&#9670;</span>
        </span>
        <span className="font-display text-sm font-semibold text-fg">Soul</span>
        <div className="flex-1" />
        <button
          type="button"
          onClick={onCollapse}
          className="w-7 h-7 flex items-center justify-center rounded hover:bg-elevated text-fg-muted hover:text-fg transition-colors cursor-pointer"
          title="Collapse sidebar"
        >
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.2">
            <rect x="1.5" y="2.5" width="13" height="11" rx="1.5" />
            <line x1="5.5" y1="2.5" x2="5.5" y2="13.5" />
          </svg>
        </button>
      </div>

      {/* New Chat button */}
      <div className="px-3 py-2">
        <button
          type="button"
          onClick={onNewChat}
          className="w-full bg-soul/10 hover:bg-soul/20 text-soul font-display font-semibold text-xs rounded-lg px-3 py-2 flex items-center gap-2 transition-colors cursor-pointer"
        >
          <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
            <path d="M8 3v10M3 8h10" />
          </svg>
          New Chat
        </button>
      </div>

      {/* Sessions */}
      <div className="flex-1 overflow-y-auto px-1">
        <div className="px-2 py-1 text-[10px] font-display uppercase tracking-widest text-fg-muted">
          Sessions
        </div>
        {sessions.map((s) => (
          <button
            key={s.id}
            type="button"
            onClick={() => onSessionSelect(s.id)}
            className={`w-full text-left px-3 py-1.5 rounded-lg text-xs truncate cursor-pointer transition-colors ${
              s.id === activeSessionId
                ? 'bg-elevated text-fg'
                : 'text-fg-secondary hover:bg-elevated/50 hover:text-fg'
            }`}
          >
            {s.title || `Session ${s.id}`}
          </button>
        ))}
      </div>

      {/* Footer */}
      <div className="px-3 py-2 border-t border-border-subtle">
        <div className="flex items-center gap-2 text-[10px] text-fg-muted">
          <span className={`w-2 h-2 rounded-full ${connected ? 'bg-stage-done' : 'bg-stage-blocked'}`} />
          <span>{connected ? 'Connected' : 'Disconnected'}</span>
        </div>
      </div>
    </div>
  );
}
