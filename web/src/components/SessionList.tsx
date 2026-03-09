import { useState, useCallback, useEffect, useRef } from 'react';
import type { SessionListProps, Session } from '../lib/types';

function formatTimestamp(iso: string): string {
  const date = new Date(iso);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMin = Math.floor(diffMs / 60000);
  const diffHr = Math.floor(diffMs / 3600000);
  const diffDay = Math.floor(diffMs / 86400000);

  if (diffMin < 1) return 'just now';
  if (diffMin < 60) return `${diffMin}m ago`;
  if (diffHr < 24) return `${diffHr}h ago`;
  if (diffDay < 7) return `${diffDay}d ago`;
  return date.toLocaleDateString();
}

function SessionItem({
  session,
  isActive,
  onSwitch,
  onDelete,
}: {
  session: Session;
  isActive: boolean;
  onSwitch: () => void;
  onDelete: () => void;
}) {
  const [confirming, setConfirming] = useState(false);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Auto-cancel confirmation after 3 seconds.
  useEffect(() => {
    if (confirming) {
      timerRef.current = setTimeout(() => setConfirming(false), 3000);
      return () => {
        if (timerRef.current) clearTimeout(timerRef.current);
      };
    }
  }, [confirming]);

  const handleDeleteClick = useCallback(
    (e: React.MouseEvent) => {
      e.stopPropagation();
      setConfirming(true);
    },
    [],
  );

  const handleConfirm = useCallback(
    (e: React.MouseEvent) => {
      e.stopPropagation();
      setConfirming(false);
      onDelete();
    },
    [onDelete],
  );

  const handleCancel = useCallback(
    (e: React.MouseEvent) => {
      e.stopPropagation();
      setConfirming(false);
    },
    [],
  );

  const title = session.title || 'New Session';
  const timestamp = formatTimestamp(session.updatedAt || session.createdAt);

  return (
    <button
      data-testid="session-item"
      type="button"
      onClick={onSwitch}
      className={`w-full text-left px-3 py-2.5 group transition-colors cursor-pointer ${
        isActive
          ? 'bg-zinc-800 border-l-2 border-indigo-500'
          : 'border-l-2 border-transparent hover:bg-zinc-800/50'
      }`}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0 flex-1">
          <div className="text-sm font-medium text-zinc-200 truncate">
            {title}
          </div>
          <div className="text-xs text-zinc-500 mt-0.5">{timestamp}</div>
        </div>
        {confirming ? (
          <div className="flex items-center gap-1 shrink-0">
            <button
              data-testid="delete-confirm-btn"
              type="button"
              onClick={handleConfirm}
              className="px-1.5 py-0.5 text-xs rounded bg-red-900/50 text-red-300 hover:bg-red-800/50 cursor-pointer"
            >
              Delete?
            </button>
            <button
              data-testid="delete-cancel-btn"
              type="button"
              onClick={handleCancel}
              className="px-1.5 py-0.5 text-xs rounded text-zinc-500 hover:text-zinc-300 cursor-pointer"
            >
              Cancel
            </button>
          </div>
        ) : (
          <button
            type="button"
            onClick={handleDeleteClick}
            className="shrink-0 mt-0.5 p-1 rounded text-zinc-600 hover:text-red-400 hover:bg-zinc-700 opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer"
            aria-label={`Delete session ${title}`}
          >
            <svg
              width="14"
              height="14"
              viewBox="0 0 14 14"
              fill="none"
              xmlns="http://www.w3.org/2000/svg"
            >
              <path
                d="M3.5 3.5L10.5 10.5M10.5 3.5L3.5 10.5"
                stroke="currentColor"
                strokeWidth="1.5"
                strokeLinecap="round"
              />
            </svg>
          </button>
        )}
      </div>
    </button>
  );
}

export function SessionList({
  sessions,
  activeSessionID,
  onCreate,
  onSwitch,
  onDelete,
}: SessionListProps) {
  return (
    <div
      data-testid="session-list"
      className="w-64 bg-zinc-900 border-r border-zinc-800 flex flex-col h-full shrink-0"
    >
      <div className="flex items-center justify-between px-4 py-3 border-b border-zinc-800">
        <h2 className="text-sm font-semibold text-zinc-300 tracking-tight">
          Sessions
        </h2>
        <button
          data-testid="new-session-button"
          type="button"
          onClick={onCreate}
          className="px-2 py-1 text-xs font-medium text-zinc-300 bg-zinc-800 hover:bg-zinc-700 rounded transition-colors cursor-pointer"
        >
          + New
        </button>
      </div>
      <div className="flex-1 overflow-y-auto">
        {sessions.map(session => (
          <SessionItem
            key={session.id}
            session={session}
            isActive={session.id === activeSessionID}
            onSwitch={() => onSwitch(session.id)}
            onDelete={() => onDelete(session.id)}
          />
        ))}
        {sessions.length === 0 && (
          <div className="px-4 py-6 text-xs text-zinc-600 text-center">
            No sessions yet
          </div>
        )}
      </div>
    </div>
  );
}
