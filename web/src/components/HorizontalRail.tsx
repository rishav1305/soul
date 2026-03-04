import { useState, useRef, useCallback } from 'react';
import type { PlannerTask, TaskActivity, ChatSession } from '../lib/types.ts';
import ChatView from './chat/ChatView.tsx';

const MIN_HEIGHT_PX = 48;
const MAX_HEIGHT_RATIO = 0.6;

interface ContextChip {
  label: string;
  onInject: () => void;
  onDismiss: () => void;
}

interface HorizontalRailProps {
  position: 'bottom' | 'top';
  expanded: boolean;
  railHeight: number;
  chatSplit: number; // 0-100, default 60 (chat % of width)
  tasks: PlannerTask[];
  activeProduct: string | null;
  lastMessageSnippet: string;
  activeTaskCount: number;
  blockedTaskCount: number;
  contextChips?: ContextChip[];
  // Session header
  sessions: ChatSession[];
  activeSessionId: number | null;
  onNewSession: () => void;
  // Auto-inject context for new sessions
  autoInjectContext: boolean;
  contextString: string;
  onToggle: () => void;
  onHeightChange: (h: number) => void;
  // Task actions
  onTaskClick: (task: PlannerTask) => void;
  taskActivities: Record<number, TaskActivity[]>;
}

export default function HorizontalRail({
  position,
  expanded,
  railHeight,
  chatSplit,
  tasks,
  activeProduct,
  lastMessageSnippet,
  activeTaskCount,
  blockedTaskCount,
  contextChips = [],
  sessions,
  activeSessionId,
  onNewSession,
  autoInjectContext,
  contextString,
  onToggle,
  onHeightChange,
  onTaskClick,
}: HorizontalRailProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const dragHeightRef = useRef<{ startY: number; startH: number } | null>(null);
  const dragSplitRef = useRef<{ startX: number; startPct: number } | null>(null);
  const [localSplit, setLocalSplit] = useState<number | null>(null);

  const effectiveSplit = localSplit ?? chatSplit;

  const maxHeight = typeof window !== 'undefined' ? Math.floor(window.innerHeight * MAX_HEIGHT_RATIO) : 400;
  const collapsedHeight = MIN_HEIGHT_PX;

  const filteredTasks = activeProduct
    ? tasks.filter((t) => t.product === activeProduct)
    : tasks;

  const activeTasks = filteredTasks.filter(
    (t) => t.stage === 'active' || t.stage === 'blocked' || t.stage === 'validation',
  );

  // Active session name
  const activeSession = sessions.find((s) => s.id === activeSessionId);
  const sessionName = activeSession?.title || (activeSessionId ? `Session ${activeSessionId}` : 'New Chat');

  // Drag handle for rail height
  const onHeightDragStart = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault();
      dragHeightRef.current = { startY: e.clientY, startH: railHeight };

      const onMove = (ev: MouseEvent) => {
        if (!dragHeightRef.current) return;
        const delta = position === 'bottom'
          ? dragHeightRef.current.startY - ev.clientY
          : ev.clientY - dragHeightRef.current.startY;
        const newH = Math.min(maxHeight, Math.max(collapsedHeight + 40, dragHeightRef.current.startH + delta));
        onHeightChange(newH);
      };

      const onUp = () => {
        dragHeightRef.current = null;
        document.removeEventListener('mousemove', onMove);
        document.removeEventListener('mouseup', onUp);
      };

      document.addEventListener('mousemove', onMove);
      document.addEventListener('mouseup', onUp);
    },
    [railHeight, maxHeight, collapsedHeight, position, onHeightChange],
  );

  // Drag handle for chat/task split
  const onSplitDragStart = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault();
      if (!containerRef.current) return;
      const rect = containerRef.current.getBoundingClientRect();
      dragSplitRef.current = { startX: e.clientX, startPct: effectiveSplit };

      const onMove = (ev: MouseEvent) => {
        if (!dragSplitRef.current || !containerRef.current) return;
        const rect2 = containerRef.current.getBoundingClientRect();
        const pct = ((ev.clientX - rect2.left) / rect2.width) * 100;
        setLocalSplit(Math.min(85, Math.max(15, pct)));
      };
      void rect;

      const onUp = () => {
        dragSplitRef.current = null;
        document.removeEventListener('mousemove', onMove);
        document.removeEventListener('mouseup', onUp);
      };

      document.addEventListener('mousemove', onMove);
      document.addEventListener('mouseup', onUp);
    },
    [effectiveSplit],
  );

  const currentHeight = expanded ? railHeight : collapsedHeight;

  const positionClass = position === 'bottom'
    ? 'bottom-0 left-0 right-0 border-t'
    : 'top-0 left-0 right-0 border-b';

  return (
    <div
      ref={containerRef}
      className={`fixed ${positionClass} border-border-subtle bg-surface z-[500] flex flex-col transition-[height] duration-200 ease-in-out`}
      style={{ height: currentHeight }}
    >
      {/* Height drag handle — top edge for bottom rail, bottom edge for top rail */}
      {expanded && (
        <div
          onMouseDown={onHeightDragStart}
          className={`absolute left-0 right-0 h-1.5 cursor-row-resize hover:bg-soul/30 transition-colors z-10 ${
            position === 'bottom' ? 'top-0' : 'bottom-0'
          }`}
        />
      )}

      {/* Collapsed bar */}
      {!expanded ? (
        <div className="flex items-center h-12 px-4 gap-3 cursor-pointer select-none" onClick={onToggle}>
          {/* Left: Soul icon + last message */}
          <span className="text-soul text-sm mr-1">&#9670;</span>
          <span className="text-xs text-fg-muted truncate flex-1 min-w-0">
            {lastMessageSnippet || 'Chat with Soul...'}
          </span>

          {/* Center: task pill */}
          {(activeTaskCount > 0 || blockedTaskCount > 0) && (
            <span className="flex items-center gap-1.5 text-xs text-fg-secondary shrink-0">
              {activeTaskCount > 0 && (
                <span className="px-1.5 py-0.5 rounded bg-stage-active/15 text-stage-active">
                  · {activeTaskCount} active
                </span>
              )}
              {blockedTaskCount > 0 && (
                <span className="px-1.5 py-0.5 rounded bg-stage-blocked/15 text-stage-blocked">
                  · {blockedTaskCount} blocked
                </span>
              )}
            </span>
          )}

          {/* Right: expand arrow */}
          <span className="text-fg-muted text-xs shrink-0">↑</span>
        </div>
      ) : (
        <>
          {/* Tab bar */}
          <div className="flex items-center h-10 px-3 border-b border-border-subtle shrink-0 gap-1">
            {/* Chat section label */}
            <span className="flex items-center gap-1.5 px-2 h-7 text-xs font-display font-medium text-fg">
              <span className="text-soul text-[10px]">&#9670;</span>
              Chat
            </span>

            {/* Session name */}
            <span className="text-xs text-fg-muted truncate max-w-[160px]">{sessionName}</span>

            {/* New session button */}
            <button
              type="button"
              onClick={onNewSession}
              className="flex items-center gap-1 px-2 h-6 rounded text-[10px] text-fg-muted hover:text-fg hover:bg-elevated transition-colors cursor-pointer ml-1"
              title="New chat"
            >
              <svg width="10" height="10" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
                <path d="M8 3v10M3 8h10" />
              </svg>
              New
            </button>

            <div className="flex-1" />

            {/* Tasks label + count */}
            <span className="flex items-center gap-1.5 px-2 h-7 text-xs font-display font-medium text-fg-muted">
              <svg width="10" height="10" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M3 4l2 2 4-4" /><path d="M3 10l2 2 4-4" />
              </svg>
              Tasks
              {activeTasks.length > 0 && (
                <span className="px-1 py-0 rounded text-[9px] bg-stage-active/15 text-stage-active">
                  {activeTasks.length}
                </span>
              )}
            </span>

            {/* Collapse */}
            <button
              type="button"
              onClick={onToggle}
              className="w-7 h-7 flex items-center justify-center rounded text-fg-muted hover:text-fg hover:bg-elevated transition-colors cursor-pointer"
              title="Collapse"
            >
              {position === 'bottom' ? '↓' : '↑'}
            </button>
          </div>

          {/* Content area — side-by-side split */}
          <div className="flex-1 overflow-hidden flex">
            {/* Chat pane */}
            <div
              className="overflow-hidden flex flex-col"
              style={{ width: `${effectiveSplit}%` }}
            >
              <ChatView
                contextChips={contextChips}
                autoInjectContext={autoInjectContext}
                contextString={contextString}
              />
            </div>

            {/* Split drag handle */}
            <div
              onMouseDown={onSplitDragStart}
              className="w-1 bg-border-subtle hover:bg-soul/40 cursor-col-resize transition-colors shrink-0 flex items-center justify-center"
            >
              <div className="w-px h-8 bg-border-default" />
            </div>

            {/* Tasks pane */}
            <div className="flex-1 overflow-y-auto px-3 py-2">
              {activeTasks.length === 0 ? (
                <p className="text-xs text-fg-muted py-4 text-center">
                  {activeProduct ? `No active tasks for ${activeProduct}` : 'No active tasks'}
                </p>
              ) : (
                <div className="space-y-1">
                  {activeTasks.map((t) => (
                    <button
                      key={t.id}
                      type="button"
                      onClick={() => onTaskClick(t)}
                      className="w-full text-left px-3 py-2 rounded-lg bg-elevated hover:bg-overlay border border-border-subtle transition-colors cursor-pointer"
                    >
                      <div className="flex items-center gap-2">
                        <span className={`w-1.5 h-1.5 rounded-full shrink-0 ${
                          t.stage === 'blocked' ? 'bg-stage-blocked' :
                          t.stage === 'validation' ? 'bg-stage-validation' :
                          'bg-stage-active'
                        }`} />
                        <span className="text-xs text-fg truncate flex-1">{t.title}</span>
                        <span className={`text-[10px] font-medium shrink-0 ${
                          t.stage === 'blocked' ? 'text-stage-blocked' :
                          t.stage === 'validation' ? 'text-stage-validation' :
                          'text-stage-active'
                        }`}>
                          {t.stage}
                        </span>
                        <span className="text-[10px] text-fg-muted font-mono shrink-0">#{t.id}</span>
                      </div>
                      {t.blocker && (
                        <p className="text-[10px] text-stage-blocked mt-1 ml-3.5 truncate">↳ {t.blocker}</p>
                      )}
                    </button>
                  ))}
                </div>
              )}
            </div>
          </div>
        </>
      )}
    </div>
  );
}
