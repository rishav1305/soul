import { useState, useRef, useCallback, useEffect } from 'react';
import type { PlannerTask, TaskActivity, TaskComment } from '../../lib/types.ts';
import ChatView from '../chat/ChatView.tsx';

const MIN_HEIGHT_PX = 48;
const MAX_HEIGHT_RATIO = 0.6;
const DEFAULT_HEIGHT_RATIO = 0.25;

interface HorizontalRailProps {
  position: 'bottom' | 'top';
  expanded: boolean;
  railHeight: number;
  chatSplit: number; // 0-100, default 60
  tasks: PlannerTask[];
  activeProduct: string | null;
  lastMessageSnippet: string;
  activeTaskCount: number;
  blockedTaskCount: number;
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
  onToggle,
  onHeightChange,
  onTaskClick,
}: HorizontalRailProps) {
  const [activeTab, setActiveTab] = useState<'chat' | 'tasks'>('chat');
  const containerRef = useRef<HTMLDivElement>(null);
  const dragRef = useRef<{ startY: number; startH: number } | null>(null);

  const maxHeight = typeof window !== 'undefined' ? Math.floor(window.innerHeight * MAX_HEIGHT_RATIO) : 400;
  const collapsedHeight = MIN_HEIGHT_PX;

  const filteredTasks = activeProduct
    ? tasks.filter((t) => t.product === activeProduct)
    : tasks;

  const activeTasks = filteredTasks.filter(
    (t) => t.stage === 'active' || t.stage === 'blocked' || t.stage === 'validation',
  );

  // Drag handle
  const onDragStart = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault();
      dragRef.current = { startY: e.clientY, startH: railHeight };

      const onMove = (ev: MouseEvent) => {
        if (!dragRef.current) return;
        const delta = position === 'bottom'
          ? dragRef.current.startY - ev.clientY
          : ev.clientY - dragRef.current.startY;
        const newH = Math.min(maxHeight, Math.max(collapsedHeight, dragRef.current.startH + delta));
        onHeightChange(newH);
      };

      const onUp = () => {
        dragRef.current = null;
        document.removeEventListener('mousemove', onMove);
        document.removeEventListener('mouseup', onUp);
      };

      document.addEventListener('mousemove', onMove);
      document.addEventListener('mouseup', onUp);
    },
    [railHeight, maxHeight, collapsedHeight, position, onHeightChange],
  );

  const currentHeight = expanded ? railHeight : collapsedHeight;

  // Chat pane width
  const chatPct = Math.max(10, Math.min(90, chatSplit));
  const taskPct = 100 - chatPct;

  const positionClass = position === 'bottom'
    ? 'bottom-0 left-0 right-0 border-t'
    : 'top-0 left-0 right-0 border-b';

  return (
    <div
      ref={containerRef}
      className={`fixed ${positionClass} border-border-subtle bg-surface z-[500] flex flex-col transition-[height] duration-200 ease-in-out`}
      style={{ height: currentHeight }}
    >
      {/* Drag handle — top edge for bottom rail, bottom edge for top rail */}
      {expanded && (
        <div
          onMouseDown={onDragStart}
          className={`absolute left-0 right-0 h-1.5 cursor-row-resize hover:bg-soul/30 transition-colors z-10 ${
            position === 'bottom' ? 'top-0' : 'bottom-0'
          }`}
        />
      )}

      {/* Collapsed bar (always visible as strip) */}
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
            <button
              type="button"
              onClick={() => setActiveTab('chat')}
              className={`flex items-center gap-1.5 px-3 h-7 rounded text-xs font-display font-medium transition-colors cursor-pointer ${
                activeTab === 'chat'
                  ? 'bg-elevated text-fg'
                  : 'text-fg-muted hover:text-fg hover:bg-elevated/50'
              }`}
            >
              <span className="text-soul text-[10px]">&#9670;</span>
              Chat
            </button>
            <button
              type="button"
              onClick={() => setActiveTab('tasks')}
              className={`flex items-center gap-1.5 px-3 h-7 rounded text-xs font-display font-medium transition-colors cursor-pointer ${
                activeTab === 'tasks'
                  ? 'bg-elevated text-fg'
                  : 'text-fg-muted hover:text-fg hover:bg-elevated/50'
              }`}
            >
              <svg width="10" height="10" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M3 4l2 2 4-4" /><path d="M3 10l2 2 4-4" />
              </svg>
              Tasks
              {activeTasks.length > 0 && (
                <span className="ml-0.5 px-1 py-0 rounded text-[9px] bg-stage-active/15 text-stage-active">
                  {activeTasks.length}
                </span>
              )}
            </button>

            <div className="flex-1" />

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

          {/* Content area */}
          <div className="flex-1 overflow-hidden flex">
            {activeTab === 'chat' ? (
              /* Full-width chat */
              <div className="flex-1 overflow-hidden">
                <ChatView />
              </div>
            ) : (
              /* Full-width task list */
              <div className="flex-1 overflow-y-auto px-3 py-2">
                {activeTasks.length === 0 ? (
                  <p className="text-xs text-fg-muted py-4 text-center">No active tasks</p>
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
                          <span className="text-[10px] text-fg-muted font-mono shrink-0">#{t.id}</span>
                        </div>
                      </button>
                    ))}
                  </div>
                )}
              </div>
            )}
          </div>
        </>
      )}
    </div>
  );
}
