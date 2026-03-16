import { TopBar } from './TopBar';
import type { Session } from '../lib/types';

interface ChatTopBarProps {
  onCreateSession: () => void;
  sessions: Session[];
  onSwitchSession: (id: string) => void;
  sessionsOpen: boolean;
  onToggleSessions: () => void;
}

export function ChatTopBar({
  onCreateSession,
  sessions,
  onSwitchSession: _onSwitchSession,
  sessionsOpen,
  onToggleSessions,
}: ChatTopBarProps) {
  const runningCount = sessions.filter((s) => s.status !== 'idle').length;
  const unreadCount = sessions.filter((s) => s.unreadCount > 0).length;

  const btnBase = 'h-8 px-2.5 rounded-md text-xs flex items-center gap-1.5 bg-surface border border-border-subtle text-fg-muted hover:text-fg transition-colors';

  return (
    <div data-testid="chat-topbar">
      <TopBar title="Chat">
        {/* + New */}
        <button
          data-testid="chat-new-btn"
          onClick={onCreateSession}
          className="h-8 px-2.5 rounded-md text-xs flex items-center gap-1.5 bg-[#7c3aed] text-white hover:bg-[#6d28d9] transition-colors"
        >
          <svg width="11" height="11" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
            <line x1="8" y1="2" x2="8" y2="14" />
            <line x1="2" y1="8" x2="14" y2="8" />
          </svg>
          New
        </button>

        {/* Running */}
        <button data-testid="chat-running-btn" className={btnBase}>
          <svg width="12" height="12" viewBox="0 0 16 16" fill="none" className="animate-spin">
            <circle cx="8" cy="8" r="6" stroke="#f59e0b" strokeWidth="1.5" strokeDasharray="28" strokeDashoffset="8" strokeLinecap="round" />
          </svg>
          <span>Running</span>
          {runningCount > 0 && (
            <span className="min-w-[18px] h-[18px] flex items-center justify-center rounded-full bg-amber-500/20 text-amber-400 text-[10px] font-medium px-1">
              {runningCount}
            </span>
          )}
        </button>

        {/* Unread */}
        <button data-testid="chat-unread-btn" className={btnBase}>
          <svg width="11" height="11" viewBox="0 0 16 16" fill="none">
            <circle cx="8" cy="8" r="5" fill="#7c3aed" />
          </svg>
          <span>Unread</span>
          {unreadCount > 0 && (
            <span className="min-w-[18px] h-[18px] flex items-center justify-center rounded-full bg-purple-500/20 text-purple-400 text-[10px] font-medium px-1">
              {unreadCount}
            </span>
          )}
        </button>

        {/* History */}
        <button
          data-testid="chat-history-btn"
          onClick={onToggleSessions}
          className={`${btnBase} ${sessionsOpen ? 'bg-surface/80 text-fg border-fg-muted' : ''}`}
        >
          <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
            <circle cx="8" cy="8" r="6" />
            <polyline points="8,4.5 8,8 11,9.5" />
          </svg>
          History
        </button>
      </TopBar>
    </div>
  );
}
