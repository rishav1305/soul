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
      return <span className="text-green-400" title="Running">&#9679;</span>;
    case 'idle':
      return <span className="text-zinc-400" title="Idle">&#9675;</span>;
    case 'completed':
      return <span className="text-zinc-600" title="Completed">&#10003;</span>;
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
        className="absolute inset-0 z-10"
        onClick={onClose}
      />

      {/* Drawer */}
      <div className="absolute left-0 top-10 bottom-0 z-20 w-52 bg-zinc-900 border-r border-zinc-800 flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between px-3 py-2 border-b border-zinc-800">
          <span className="text-xs font-semibold text-zinc-300 uppercase tracking-wide">
            Sessions
          </span>
          <button
            type="button"
            onClick={onClose}
            className="text-zinc-500 hover:text-zinc-300 text-sm cursor-pointer"
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
            className="w-full text-xs font-medium text-white bg-sky-600 hover:bg-sky-500 rounded px-2 py-1.5 cursor-pointer"
          >
            + New Chat
          </button>
        </div>

        {/* Session list */}
        <div className="flex-1 overflow-y-auto">
          {sessions.length === 0 && (
            <div className="px-3 py-4 text-xs text-zinc-600 text-center">
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
              className={`w-full text-left px-3 py-2 flex items-start gap-2 cursor-pointer hover:bg-zinc-800/60 ${
                session.id === activeSessionId ? 'bg-zinc-800' : ''
              }`}
            >
              <span className="text-xs mt-0.5 shrink-0">
                {statusIcon(session.status)}
              </span>
              <div className="min-w-0 flex-1">
                <div className="text-xs text-zinc-200 truncate">
                  {session.title || 'Untitled'}
                </div>
                <div className="text-[10px] text-zinc-500">
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
