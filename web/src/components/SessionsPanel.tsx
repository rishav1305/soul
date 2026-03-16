import { SessionList } from '../components/SessionList';
import type { Session } from '../lib/types';

interface SessionsPanelProps {
  open: boolean;
  sessions: Session[];
  activeSessionID: string | null;
  onSwitch: (id: string) => void;
  onDelete: (id: string) => void;
  onRename: (id: string, title: string) => void;
  onClose: () => void;
}

export function SessionsPanel({
  open,
  sessions,
  activeSessionID,
  onSwitch,
  onDelete,
  onRename,
  onClose,
}: SessionsPanelProps) {
  return (
    <div
      data-testid="sessions-panel"
      className="sessions-transition bg-deep border-l border-border-subtle flex flex-col overflow-hidden"
      style={{ width: open ? 220 : 0 }}
    >
      <div
        data-testid="sessions-panel-header"
        className="flex items-center justify-between px-3 py-2 border-b border-border-subtle shrink-0"
      >
        <span className="text-sm font-medium text-fg">Sessions</span>
        <button
          data-testid="sessions-panel-close"
          onClick={onClose}
          className="p-1 rounded hover:bg-white/10 text-fg-muted"
          aria-label="Close sessions panel"
        >
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
            <polyline points="6 4 10 8 6 12" />
          </svg>
        </button>
      </div>

      <div className="flex-1 overflow-y-auto min-h-0">
        <SessionList
          sessions={sessions}
          activeSessionID={activeSessionID}
          onCreate={() => {}}
          onSwitch={onSwitch}
          onDelete={onDelete}
          onRename={onRename}
        />
      </div>
    </div>
  );
}
