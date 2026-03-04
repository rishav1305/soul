import type { ChatSession } from '../../lib/types.ts';

interface SessionsDrawerProps {
  onClose: () => void;
  sessions: ChatSession[];
  activeSessionId: number | null;
  onSessionSelect: (id: number) => void;
  onNewChat: () => void;
  connected: boolean;
}

export default function SessionsDrawer({
  onClose,
  sessions,
  activeSessionId,
  onSessionSelect,
  onNewChat,
  connected,
}: SessionsDrawerProps) {
  return (
    <div className="absolute inset-0 z-50 flex">
      {/* Backdrop */}
      <div className="absolute inset-0 bg-black/40" onClick={onClose} />

      {/* Drawer — anchored next to left rail */}
      <div className="relative z-10 ml-14 w-64 h-full bg-surface border-r border-border-default flex flex-col shadow-2xl animate-slide-left">
        {/* Header */}
        <div className="flex items-center gap-2 px-4 h-12 border-b border-border-subtle shrink-0">
          <span className="relative text-lg text-soul">&#9670;</span>
          <span className="font-display text-sm font-semibold text-fg">Soul</span>
          <div className="flex-1" />
          <button
            type="button"
            onClick={onClose}
            className="w-7 h-7 flex items-center justify-center rounded hover:bg-elevated text-fg-muted hover:text-fg transition-colors cursor-pointer"
          >
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
              <path d="M1 1l12 12M13 1L1 13" />
            </svg>
          </button>
        </div>

        {/* New Chat */}
        <div className="px-3 py-3 shrink-0">
          <button
            type="button"
            onClick={() => { onNewChat(); onClose(); }}
            className="w-full bg-soul/10 hover:bg-soul/20 text-soul font-display font-semibold text-sm rounded-lg px-4 py-2.5 flex items-center gap-2 transition-colors cursor-pointer"
          >
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
              <path d="M8 3v10M3 8h10" />
            </svg>
            New Chat
          </button>
        </div>

        {/* Sessions list */}
        <div className="flex-1 overflow-y-auto px-1">
          <div className="px-2 py-1.5 text-[10px] font-display uppercase tracking-widest text-fg-muted">
            Sessions
          </div>
          {sessions.length === 0 && (
            <p className="px-3 py-2 text-xs text-fg-muted">No sessions yet.</p>
          )}
          {sessions.map((s) => (
            <button
              key={s.id}
              type="button"
              onClick={() => { onSessionSelect(s.id); onClose(); }}
              className={`w-full text-left px-3 py-2 rounded-lg text-sm truncate cursor-pointer transition-colors ${
                s.id === activeSessionId
                  ? 'bg-elevated text-fg'
                  : 'text-fg-secondary hover:bg-elevated/50 hover:text-fg'
              }`}
            >
              {s.title || `Session ${s.id}`}
            </button>
          ))}
        </div>

        {/* Connection status */}
        <div className="px-4 py-3 border-t border-border-subtle shrink-0">
          <div className="flex items-center gap-2 text-xs text-fg-muted">
            <span className={`w-2 h-2 rounded-full shrink-0 ${connected ? 'bg-stage-done' : 'bg-stage-blocked'}`} />
            <span>{connected ? 'Connected' : 'Disconnected'}</span>
          </div>
        </div>
      </div>
    </div>
  );
}
