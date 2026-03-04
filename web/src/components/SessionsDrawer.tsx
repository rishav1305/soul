import type { ChatSession } from '../lib/types.ts';

interface SessionsDrawerProps {
  open: boolean;
  onClose: () => void;
  sessions: ChatSession[];
  activeSessionId: number | null;
  onSessionSelect: (id: number) => void;
  onNewChat: () => void;
  connected: boolean;
}

export default function SessionsDrawer({
  open,
  onClose,
  sessions,
  activeSessionId,
  onSessionSelect,
  onNewChat,
  connected,
}: SessionsDrawerProps) {
  if (!open) return null;

  return (
    <>
      {/* Backdrop */}
      <div
        className="fixed inset-0 z-[800] bg-black/40"
        onClick={onClose}
      />

      {/* Drawer panel */}
      <div className="fixed left-14 top-0 h-full w-72 bg-surface border-r border-border-subtle z-[801] flex flex-col animate-slide-left shadow-2xl">
        {/* Header */}
        <div className="glass flex items-center gap-2 h-14 px-4 shrink-0">
          <span className="relative">
            <span className="absolute inset-0 -m-0.5 bg-soul/10 rounded-full blur-sm animate-soul-pulse" />
            <span className="relative text-xl text-soul">&#9670;</span>
          </span>
          <span className="font-display text-base font-semibold text-fg">Soul</span>
          <div className="flex-1" />
          <button
            type="button"
            onClick={onClose}
            className="w-8 h-8 flex items-center justify-center rounded hover:bg-elevated text-fg-muted hover:text-fg transition-colors cursor-pointer"
            title="Close"
          >
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
              <path d="M4 4l8 8M12 4l-8 8" />
            </svg>
          </button>
        </div>

        {/* New Chat */}
        <div className="px-3 py-3">
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
          <div className="px-2 py-1.5 text-xs font-display uppercase tracking-widest text-fg-muted">
            Sessions
          </div>
          {sessions.length === 0 && (
            <p className="px-3 py-2 text-sm text-fg-muted">No sessions yet</p>
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

        {/* Footer */}
        <div className="px-3 py-3 border-t border-border-subtle">
          <div className="flex items-center gap-2 text-xs text-fg-muted">
            <span className={`w-2.5 h-2.5 rounded-full ${connected ? 'bg-stage-done' : 'bg-stage-blocked'}`} />
            <span>{connected ? 'Connected' : 'Disconnected'}</span>
          </div>
        </div>
      </div>
    </>
  );
}
