import type { ChatSession } from '../../lib/types.ts';

interface SessionDrawerProps {
  sessions: ChatSession[];
  activeSessionId: number | null;
  onSelect: (id: number) => void;
  onNew: () => void;
  onClose: () => void;
}

function relativeTime(dateStr: string): string {
  const now = Date.now();
  const then = new Date(dateStr).getTime();
  const diffMs = now - then;
  const diffSec = Math.floor(diffMs / 1000);

  if (diffSec < 60) return 'just now';

  const diffMin = Math.floor(diffSec / 60);
  if (diffMin < 60) return `${diffMin}m ago`;

  const diffHr = Math.floor(diffMin / 60);
  if (diffHr < 24) return `${diffHr}h ago`;

  const diffDay = Math.floor(diffHr / 24);
  if (diffDay < 30) return `${diffDay}d ago`;

  const diffMonth = Math.floor(diffDay / 30);
  return `${diffMonth}mo ago`;
}

function statusIcon(status: ChatSession['status']) {
  switch (status) {
    case 'running':
      return <span className="text-stage-active" title="Running">&#9679;</span>;
    case 'idle':
      return <span className="text-fg-muted" title="Idle">&#9675;</span>;
    case 'completed':
      return <span className="text-stage-done" title="Completed">&#10003;</span>;
  }
}

export default function SessionDrawer({
  sessions,
  activeSessionId,
  onSelect,
  onNew,
  onClose,
}: SessionDrawerProps) {
  return (
    <>
      {/* Backdrop */}
      <div
        className="absolute inset-0 z-10 bg-black/40"
        onClick={onClose}
      />

      {/* Drawer */}
      <div className="absolute left-0 top-11 bottom-0 z-20 w-52 bg-surface border-r border-border-default flex flex-col animate-slide-left">
        {/* Header */}
        <div className="flex items-center justify-between px-3 py-2 border-b border-border-default">
          <span className="font-display text-[10px] font-semibold text-fg-secondary uppercase tracking-widest">
            Sessions
          </span>
          <button
            type="button"
            onClick={onClose}
            className="text-fg-muted hover:text-fg text-sm cursor-pointer"
            title="Close"
          >
            &times;
          </button>
        </div>

        {/* New Chat button */}
        <div className="px-3 py-2">
          <button
            type="button"
            onClick={onNew}
            className="w-full text-xs font-semibold text-deep bg-soul hover:bg-soul/80 rounded px-2 py-1.5 cursor-pointer"
          >
            + New Chat
          </button>
        </div>

        {/* Session list */}
        <div className="flex-1 overflow-y-auto">
          {sessions.length === 0 && (
            <div className="px-3 py-4 text-xs text-fg-muted text-center">
              No sessions yet
            </div>
          )}
          {sessions.slice(0, 10).map((session) => (
            <button
              key={session.id}
              type="button"
              onClick={() => {
                onSelect(session.id);
                onClose();
              }}
              className={`w-full text-left px-3 py-2 flex items-start gap-2 cursor-pointer hover:bg-elevated ${
                session.id === activeSessionId ? 'bg-elevated' : ''
              }`}
            >
              <span className="text-xs mt-0.5 shrink-0">
                {statusIcon(session.status)}
              </span>
              <div className="min-w-0 flex-1">
                <div className="text-fg font-body text-xs truncate">
                  {session.title || 'Untitled'}
                </div>
                <div className="text-[10px] text-fg-muted">
                  {relativeTime(session.updated_at)}
                </div>
              </div>
            </button>
          ))}
        </div>
      </div>
    </>
  );
}
