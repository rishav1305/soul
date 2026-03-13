import React, { useState, useCallback, useEffect, useRef, useMemo } from 'react';
import type { SessionListProps, Session, SessionStatus } from '../lib/types';
import { formatRelativeTime, getTimeGroup, type TimeGroup } from '../lib/utils';
import { usePerformance } from '../hooks/usePerformance';

function StatusDot({ status, hasUnread }: { status: SessionStatus; hasUnread?: boolean }) {
  switch (status) {
    case 'running':
      return <span data-testid="status-dot" className="mt-1 w-2 h-2 rounded-full bg-green-500 animate-pulse shrink-0" />;
    case 'completed_unread':
      return <span data-testid="status-dot" className="mt-1 w-2 h-2 rounded-full bg-soul ring-2 ring-soul/30 shrink-0" />;
    default:
      if (hasUnread) {
        return <span data-testid="status-dot" className="mt-1 w-2 h-2 rounded-full bg-soul ring-2 ring-soul/30 shrink-0" />;
      }
      return <span data-testid="status-dot" className="mt-1 w-2 h-2 rounded-full bg-fg-muted shrink-0" />;
  }
}

const SessionItem = React.memo(function SessionItem({
  session,
  isActive,
  onSwitch,
  onDelete,
  onRename,
}: {
  session: Session;
  isActive: boolean;
  onSwitch: (id: string) => void;
  onDelete: (id: string) => void;
  onRename: (id: string, title: string) => void;
}) {
  usePerformance('SessionItem');

  const [confirming, setConfirming] = useState(false);
  const [editing, setEditing] = useState(false);
  const [editValue, setEditValue] = useState('');
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (confirming) {
      timerRef.current = setTimeout(() => setConfirming(false), 3000);
      return () => {
        if (timerRef.current) clearTimeout(timerRef.current);
      };
    }
  }, [confirming]);

  useEffect(() => {
    if (editing && inputRef.current) {
      inputRef.current.focus();
      inputRef.current.select();
    }
  }, [editing]);

  const handleDeleteClick = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    setConfirming(true);
  }, []);

  const handleConfirm = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    setConfirming(false);
    onDelete(session.id);
  }, [onDelete, session.id]);

  const handleCancel = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    setConfirming(false);
  }, []);

  const handleDoubleClick = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    setEditValue(session.title || 'New Session');
    setEditing(true);
  }, [session.title]);

  const handleRenameSubmit = useCallback(() => {
    const trimmed = editValue.trim();
    if (trimmed && trimmed !== session.title) {
      onRename(session.id, trimmed);
    }
    setEditing(false);
  }, [editValue, session.id, session.title, onRename]);

  const handleRenameKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      handleRenameSubmit();
    } else if (e.key === 'Escape') {
      setEditing(false);
    }
  }, [handleRenameSubmit]);

  const title = session.title || 'New Session';
  const timestamp = formatRelativeTime(session.updatedAt || session.createdAt);
  const hasUnread = !isActive && session.unreadCount > 0;
  const preview = session.lastMessage || '';
  const previewText = preview.length > 60 ? preview.slice(0, 60) + '...' : preview;

  return (
    <button
      data-testid="session-item"
      type="button"
      onClick={() => onSwitch(session.id)}
      className={`w-full text-left px-3 py-3.5 md:py-2.5 group transition-colors cursor-pointer ${
        isActive
          ? 'bg-elevated border-l-2 border-soul'
          : 'border-l-2 border-transparent hover:bg-elevated/50'
      }`}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0 flex-1 flex items-start gap-2">
          <StatusDot status={session.status} hasUnread={hasUnread} />
          <div className="min-w-0 flex-1">
            {editing ? (
              <input
                ref={inputRef}
                data-testid="session-rename-input"
                type="text"
                value={editValue}
                onChange={e => setEditValue(e.target.value)}
                onBlur={handleRenameSubmit}
                onKeyDown={handleRenameKeyDown}
                onClick={e => e.stopPropagation()}
                className="w-full text-sm font-medium bg-elevated border border-soul/40 rounded px-1 py-0.5 text-fg outline-none"
                maxLength={200}
              />
            ) : (
              <div
                data-testid="session-title"
                className={`text-sm truncate ${hasUnread ? 'font-bold text-fg' : 'font-medium text-fg'}`}
                onDoubleClick={handleDoubleClick}
              >
                {title}
              </div>
            )}
            {previewText && (
              <div data-testid="session-preview" className="text-xs text-fg-muted mt-0.5 truncate">
                {previewText}
              </div>
            )}
          </div>
        </div>
        <div className="flex flex-col items-end gap-1 shrink-0">
          <div className="flex items-center gap-1">
            {hasUnread && (
              <span
                data-testid="unread-badge"
                className="inline-flex items-center justify-center min-w-[18px] h-[18px] px-1 text-[10px] font-bold rounded-full bg-soul text-white"
              >
                {session.unreadCount > 99 ? '99+' : session.unreadCount}
              </span>
            )}
            <span className="text-[11px] text-fg-muted">{timestamp}</span>
          </div>
          {confirming ? (
            <div className="flex items-center gap-1">
              <button
                data-testid="delete-confirm-btn"
                type="button"
                onClick={handleConfirm}
                className="px-2.5 py-1.5 md:px-1.5 md:py-0.5 text-xs rounded bg-red-900/50 text-red-300 hover:bg-red-800/50 cursor-pointer"
              >
                Delete?
              </button>
              <button
                data-testid="delete-cancel-btn"
                type="button"
                onClick={handleCancel}
                className="px-2.5 py-1.5 md:px-1.5 md:py-0.5 text-xs rounded text-fg-muted hover:text-fg cursor-pointer"
              >
                Cancel
              </button>
            </div>
          ) : (
            <button
              type="button"
              onClick={handleDeleteClick}
              className="p-1 rounded text-fg-muted hover:text-red-400 hover:bg-elevated opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer"
              aria-label={`Delete session ${title}`}
            >
              <svg width="14" height="14" viewBox="0 0 14 14" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path d="M3.5 3.5L10.5 10.5M10.5 3.5L3.5 10.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
              </svg>
            </button>
          )}
        </div>
      </div>
    </button>
  );
});

export function SessionList({
  sessions,
  activeSessionID,
  onCreate,
  onSwitch,
  onDelete,
  onRename,
}: SessionListProps & { onRename: (sessionID: string, title: string) => void }) {
  const [searchQuery, setSearchQuery] = useState('');

  const filtered = useMemo(
    () => searchQuery
      ? sessions.filter(s => s.title.toLowerCase().includes(searchQuery.toLowerCase()))
      : sessions,
    [sessions, searchQuery],
  );

  const grouped = useMemo(() => {
    const groups: { group: TimeGroup; sessions: Session[] }[] = [];
    const order: TimeGroup[] = ['Today', 'Yesterday', 'Older'];
    const map = new Map<TimeGroup, Session[]>();

    for (const s of filtered) {
      const g = getTimeGroup(s.updatedAt || s.createdAt);
      if (!map.has(g)) map.set(g, []);
      map.get(g)!.push(s);
    }

    for (const g of order) {
      const items = map.get(g);
      if (items && items.length > 0) {
        groups.push({ group: g, sessions: items });
      }
    }
    return groups;
  }, [filtered]);

  return (
    <div
      data-testid="session-list"
      className="w-64 bg-surface border-r border-border-subtle flex flex-col h-full shrink-0"
    >
      <div className="flex items-center justify-between px-4 py-3 border-b border-border-subtle">
        <h2 className="text-sm font-semibold text-fg-secondary tracking-tight">
          Sessions
        </h2>
        <button
          data-testid="new-session-button"
          type="button"
          onClick={onCreate}
          className="px-3 py-2 md:px-2 md:py-1 text-xs font-medium text-fg bg-elevated hover:bg-overlay rounded transition-colors cursor-pointer"
        >
          + New
        </button>
      </div>
      {sessions.length > 5 && (
        <div className="px-3 py-2 border-b border-border-subtle">
          <input
            data-testid="session-search"
            type="text"
            value={searchQuery}
            onChange={e => setSearchQuery(e.target.value)}
            placeholder="Search sessions..."
            className="w-full px-2 py-1.5 text-sm bg-elevated border border-border-default rounded text-fg placeholder:text-fg-muted outline-none focus:border-soul/40"
          />
        </div>
      )}
      <div className="flex-1 overflow-y-auto">
        {grouped.map(({ group, sessions: items }) => (
          <div key={group}>
            <div data-testid="time-group-header" className="px-3 pt-3 pb-1 text-xs text-fg-muted uppercase tracking-wide">
              {group}
            </div>
            {items.map(session => (
              <SessionItem
                key={session.id}
                session={session}
                isActive={session.id === activeSessionID}
                onSwitch={onSwitch}
                onDelete={onDelete}
                onRename={onRename}
              />
            ))}
          </div>
        ))}
        {filtered.length === 0 && (
          <div className="px-4 py-6 text-xs text-fg-muted text-center">
            {searchQuery ? 'No matching sessions' : 'No sessions yet'}
          </div>
        )}
      </div>
    </div>
  );
}
