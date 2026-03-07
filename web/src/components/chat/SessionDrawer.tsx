import { useState, useRef, useEffect } from 'react';
import type { ChatSession } from '../../lib/types.ts';

interface SessionDrawerProps {
  sessions: ChatSession[];
  activeSessionId: number | null;
  onSelect: (id: number) => void;
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
  if (diffDay < 7) return `${diffDay}d ago`;

  const diffWeek = Math.floor(diffDay / 7);
  if (diffWeek < 5) return `${diffWeek}w ago`;

  const diffMonth = Math.floor(diffDay / 30);
  return `${diffMonth}mo ago`;
}

function duration(created: string, updated: string): string {
  const start = new Date(created).getTime();
  const end = new Date(updated).getTime();
  const diffMin = Math.floor((end - start) / 60000);
  if (diffMin < 1) return '<1m';
  if (diffMin < 60) return `${diffMin}m`;
  const h = Math.floor(diffMin / 60);
  const m = diffMin % 60;
  return m > 0 ? `${h}h${m}m` : `${h}h`;
}

function statusDot(status: ChatSession['status']) {
  switch (status) {
    case 'running':
      return <span className="w-1.5 h-1.5 rounded-full bg-stage-active animate-pulse shrink-0" />;
    case 'idle':
      return <span className="w-1.5 h-1.5 rounded-full bg-fg-muted/40 shrink-0" />;
    case 'completed':
      return <span className="w-1.5 h-1.5 rounded-full bg-stage-done shrink-0" />;
  }
}

export default function SessionDrawer({
  sessions,
  activeSessionId,
  onSelect,
  onClose,
}: SessionDrawerProps) {
  const [search, setSearch] = useState('');
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  const query = search.toLowerCase().trim();
  const filtered = query
    ? sessions.filter(s =>
        s.title.toLowerCase().includes(query) ||
        s.summary.toLowerCase().includes(query),
      )
    : sessions;

  return (
    <>
      {/* Backdrop */}
      <div
        className="absolute inset-0 z-10 bg-black/50 backdrop-blur-[3px]"
        onClick={onClose}
      />

      {/* Full-width overlay */}
      <div className="absolute inset-0 z-20 bg-surface flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between px-3 h-9 border-b border-border-subtle shrink-0">
          <div className="flex items-center gap-2">
            <svg width="11" height="11" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" className="text-soul">
              <path d="M2 4h12M2 8h8M2 12h10" />
            </svg>
            <span className="font-display text-[10px] font-semibold text-fg-secondary uppercase tracking-widest">
              History
            </span>
            <span className="text-[9px] font-mono text-fg-muted/60">
              {query ? `${filtered.length}/${sessions.length}` : sessions.length}
            </span>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="w-6 h-6 flex items-center justify-center rounded text-fg-muted hover:text-fg hover:bg-elevated transition-colors cursor-pointer"
            title="Close (Esc)"
          >
            <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round"><path d="M4 4l8 8M12 4l-8 8" /></svg>
          </button>
        </div>

        {/* Search */}
        <div className="px-3 py-2 border-b border-border-subtle shrink-0">
          <div className="relative">
            <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" className="absolute left-2.5 top-1/2 -translate-y-1/2 text-fg-muted/50">
              <circle cx="7" cy="7" r="4.5" />
              <path d="M10.5 10.5L14 14" />
            </svg>
            <input
              ref={inputRef}
              type="text"
              value={search}
              onChange={e => setSearch(e.target.value)}
              onKeyDown={e => {
                if (e.key === 'Escape') {
                  if (search) setSearch('');
                  else onClose();
                }
              }}
              placeholder="Search conversations..."
              className="w-full h-7 pl-8 pr-3 text-[11px] bg-elevated/50 border border-border-subtle rounded-md text-fg placeholder:text-fg-muted/40 outline-none focus:border-soul/40 transition-colors"
            />
          </div>
        </div>

        {/* Session list */}
        <div className="flex-1 overflow-y-auto">
          {filtered.length === 0 && (
            <div className="px-4 py-10 text-xs text-fg-muted text-center">
              {query ? (
                <>No matches for "{search}"</>
              ) : (
                <>
                  <div className="text-lg mb-2 opacity-30">◇</div>
                  No conversations yet
                </>
              )}
            </div>
          )}
          {filtered.slice(0, 30).map((session) => {
            const isActive = session.id === activeSessionId;
            return (
              <button
                key={session.id}
                type="button"
                onClick={() => {
                  onSelect(session.id);
                  onClose();
                }}
                className={`w-full text-left px-3 py-2 cursor-pointer transition-all group border-l-2 ${
                  isActive
                    ? 'bg-soul/8 border-soul'
                    : 'border-transparent hover:bg-white/[0.03]'
                }`}
              >
                {/* Single-row: status dot + title + stats */}
                <div className="flex items-center gap-1.5">
                  {statusDot(session.status)}
                  <span className={`text-[11px] leading-tight truncate min-w-0 flex-1 ${
                    isActive ? 'text-soul font-medium' : 'text-fg-secondary group-hover:text-fg'
                  }`}>
                    {session.title || 'Untitled'}
                  </span>

                  {/* Inline stats */}
                  <span className="flex items-center gap-1.5 text-[9px] font-mono text-fg-muted/50 shrink-0 ml-2">
                    {session.message_count > 0 && (
                      <span className="flex items-center gap-0.5">
                        <svg width="8" height="8" viewBox="0 0 16 16" fill="currentColor" className="opacity-60">
                          <path d="M2 3h12a1 1 0 011 1v7a1 1 0 01-1 1H5l-3 3V4a1 1 0 011-1z" />
                        </svg>
                        {session.message_count}
                      </span>
                    )}
                    {session.model && (
                      <>
                        <span className="opacity-30">·</span>
                        <span>{session.model}</span>
                      </>
                    )}
                    {session.message_count > 1 && (
                      <>
                        <span className="opacity-30">·</span>
                        <span>{duration(session.created_at, session.updated_at)}</span>
                      </>
                    )}
                    <span className="opacity-30">·</span>
                    <span className="opacity-70">{relativeTime(session.updated_at)}</span>
                  </span>
                </div>

                {/* Summary — compact single line below */}
                {session.summary && (
                  <div className="text-[10px] leading-snug text-fg-muted/60 mt-0.5 ml-3 truncate">
                    {session.summary}
                  </div>
                )}
              </button>
            );
          })}
        </div>
      </div>
    </>
  );
}
