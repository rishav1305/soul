import { useState, useRef, useEffect, useCallback } from 'react';
import { TopBar } from './TopBar';
import type { Session } from '../lib/types';
import { formatRelativeTime } from '../lib/utils';

interface ChatTopBarProps {
  onCreateSession: () => void;
  sessions: Session[];
  onSwitchSession: (id: string) => void;
  sessionsOpen: boolean;
  onToggleSessions: () => void;
}

// Dropdown popover positioned below its anchor button
function Dropdown({ open, onClose, children }: { open: boolean; onClose: () => void; children: React.ReactNode }) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const handler = (e: PointerEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose();
    };
    document.addEventListener('pointerdown', handler);
    return () => document.removeEventListener('pointerdown', handler);
  }, [open, onClose]);

  if (!open) return null;
  return (
    <div ref={ref} className="mt-1.5 z-50 w-72 max-sm:fixed max-sm:left-2 max-sm:right-2 max-sm:w-auto sm:absolute sm:top-full sm:right-0 bg-deep border border-border-default rounded-xl shadow-xl shadow-black/40 overflow-hidden">
      {children}
    </div>
  );
}

function SessionRow({ session, onClick }: { session: Session; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="w-full flex items-center gap-2.5 px-3 py-2 text-left hover:bg-elevated/50 transition-colors cursor-pointer"
    >
      <div className="flex-1 min-w-0">
        <div className="text-xs text-fg truncate">{session.title || 'Untitled'}</div>
        {session.lastMessage && (
          <div className="text-[10px] text-fg-muted truncate mt-0.5">{session.lastMessage}</div>
        )}
      </div>
      <span className="text-[9px] text-fg-muted shrink-0">{formatRelativeTime(session.updatedAt)}</span>
    </button>
  );
}

export function ChatTopBar({
  onCreateSession,
  sessions,
  onSwitchSession,
  sessionsOpen,
  onToggleSessions,
}: ChatTopBarProps) {
  const [runningOpen, setRunningOpen] = useState(false);
  const [unreadOpen, setUnreadOpen] = useState(false);

  const runningSessions = sessions.filter(s => s.status !== 'idle');
  const unreadSessions = sessions.filter(s => s.unreadCount > 0);
  const runningCount = runningSessions.length;
  const unreadCount = unreadSessions.length;

  const handleSwitchFromDropdown = useCallback((id: string) => {
    onSwitchSession(id);
    setRunningOpen(false);
    setUnreadOpen(false);
  }, [onSwitchSession]);

  const closeRunning = useCallback(() => setRunningOpen(false), []);
  const closeUnread = useCallback(() => setUnreadOpen(false), []);

  // Mobile: square icon-only buttons (w-8 h-8). Desktop: h-8 with padding and text.
  const btnBase = 'w-8 h-8 sm:w-auto sm:h-8 sm:px-2.5 rounded-md text-xs flex items-center justify-center sm:justify-start gap-1.5 bg-surface border border-border-subtle text-fg-muted hover:text-fg transition-colors cursor-pointer';

  return (
    <div data-testid="chat-topbar">
      <TopBar title="Chat">
        {/* + New */}
        <button
          data-testid="chat-new-btn"
          onClick={onCreateSession}
          aria-label="New chat session"
          className="w-8 h-8 sm:w-auto sm:h-8 sm:px-2.5 rounded-md text-xs flex items-center justify-center sm:justify-start gap-1.5 bg-[#7c3aed] text-white hover:bg-[#6d28d9] transition-colors cursor-pointer"
        >
          <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" className="shrink-0">
            <line x1="8" y1="2" x2="8" y2="14" />
            <line x1="2" y1="8" x2="14" y2="8" />
          </svg>
          <span className="hidden sm:inline">New</span>
        </button>

        {/* Running — with dropdown */}
        <div className="relative">
          <button
            data-testid="chat-running-btn"
            onClick={() => { setRunningOpen(prev => !prev); setUnreadOpen(false); }}
            aria-label={`Running sessions${runningCount > 0 ? ` (${runningCount})` : ''}`}
            aria-expanded={runningOpen}
            className={`${btnBase} relative ${runningOpen ? 'bg-elevated text-fg' : ''}`}
          >
            <svg width="12" height="12" viewBox="0 0 16 16" fill="none" className={`shrink-0 ${runningCount > 0 ? 'animate-spin' : ''}`}>
              <circle cx="8" cy="8" r="6" stroke="#f59e0b" strokeWidth="1.5" strokeDasharray="28" strokeDashoffset="8" strokeLinecap="round" />
            </svg>
            <span className="hidden sm:inline">Running</span>
            {runningCount > 0 && (
              <span className="absolute -top-1.5 -right-1.5 sm:static sm:top-auto sm:right-auto min-w-[16px] h-[16px] sm:min-w-[18px] sm:h-[18px] flex items-center justify-center rounded-full bg-amber-500/20 text-amber-400 text-[9px] sm:text-[10px] font-medium px-0.5 sm:px-1">
                {runningCount}
              </span>
            )}
          </button>
          <Dropdown open={runningOpen} onClose={closeRunning}>
            <div className="px-3 pt-2.5 pb-1.5 text-[10px] text-fg-muted uppercase tracking-wider font-medium">
              Active Streams
            </div>
            {runningSessions.length === 0 ? (
              <div className="px-3 py-4 text-xs text-fg-muted text-center">No active streams</div>
            ) : (
              <div className="max-h-60 overflow-y-auto">
                {runningSessions.map(s => (
                  <SessionRow key={s.id} session={s} onClick={() => handleSwitchFromDropdown(s.id)} />
                ))}
              </div>
            )}
          </Dropdown>
        </div>

        {/* Unread — with dropdown */}
        <div className="relative">
          <button
            data-testid="chat-unread-btn"
            onClick={() => { setUnreadOpen(prev => !prev); setRunningOpen(false); }}
            aria-label={`Unread sessions${unreadCount > 0 ? ` (${unreadCount})` : ''}`}
            aria-expanded={unreadOpen}
            className={`${btnBase} relative ${unreadOpen ? 'bg-elevated text-fg' : ''}`}
          >
            <svg width="11" height="11" viewBox="0 0 16 16" fill="none" className="shrink-0">
              <circle cx="8" cy="8" r="5" fill="#7c3aed" />
            </svg>
            <span className="hidden sm:inline">Unread</span>
            {unreadCount > 0 && (
              <span className="absolute -top-1.5 -right-1.5 sm:static sm:top-auto sm:right-auto min-w-[16px] h-[16px] sm:min-w-[18px] sm:h-[18px] flex items-center justify-center rounded-full bg-purple-500/20 text-purple-400 text-[9px] sm:text-[10px] font-medium px-0.5 sm:px-1">
                {unreadCount}
              </span>
            )}
          </button>
          <Dropdown open={unreadOpen} onClose={closeUnread}>
            <div className="px-3 pt-2.5 pb-1.5 flex items-center justify-between">
              <span className="text-[10px] text-fg-muted uppercase tracking-wider font-medium">Unread Sessions</span>
            </div>
            {unreadSessions.length === 0 ? (
              <div className="px-3 py-4 text-xs text-fg-muted text-center">All caught up</div>
            ) : (
              <div className="max-h-60 overflow-y-auto">
                {unreadSessions.map(s => (
                  <SessionRow key={s.id} session={s} onClick={() => handleSwitchFromDropdown(s.id)} />
                ))}
              </div>
            )}
          </Dropdown>
        </div>

        {/* History */}
        <button
          data-testid="chat-history-btn"
          onClick={onToggleSessions}
          aria-label={sessionsOpen ? 'Hide session history' : 'Show session history'}
          aria-expanded={sessionsOpen}
          className={`${btnBase} ${sessionsOpen ? 'bg-elevated text-fg' : ''}`}
        >
          <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" className="shrink-0">
            <circle cx="8" cy="8" r="6" />
            <polyline points="8,4.5 8,8 11,9.5" />
          </svg>
          <span className="hidden sm:inline">History</span>
        </button>
      </TopBar>
    </div>
  );
}
