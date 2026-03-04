import { useRef, useCallback, useEffect, useState } from 'react';
import type {
  PlannerTask,
  TaskStage,
  TaskActivity,
  TaskComment,
  HorizontalRailTab,
} from '../../lib/types.ts';
import ChatView from '../chat/ChatView.tsx';
import ListView from '../planner/ListView.tsx';
import TaskDetail from '../planner/TaskDetail.tsx';
import NewTaskForm from '../planner/NewTaskForm.tsx';

interface HorizontalRailProps {
  // Layout state
  expanded: boolean;
  heightVh: number;
  tab: HorizontalRailTab;
  chatSplitPct: number;
  position: 'bottom' | 'top';
  onToggleExpand: () => void;
  onSetTab: (tab: HorizontalRailTab) => void;
  onHeightChange: (vh: number) => void;
  // Chat
  activeSessionId: number | null;
  onSessionCreated?: (id: number) => void;
  lastChatSnippet?: string;
  // Tasks
  tasks: PlannerTask[];
  activeProduct: string | null;
  taskActivities: Record<number, TaskActivity[]>;
  taskStreams: Record<number, string>;
  taskComments: Record<number, TaskComment[]>;
  updateTask: (id: number, updates: Partial<PlannerTask>) => Promise<PlannerTask>;
  moveTask: (id: number, stage: TaskStage, comment: string) => Promise<PlannerTask>;
  deleteTask: (id: number) => Promise<void>;
  fetchComments: (id: number) => Promise<TaskComment[]>;
  addComment: (id: number, body: string) => Promise<TaskComment>;
  products: string[];
  createTask: (title: string, description: string, priority: number, product: string) => Promise<PlannerTask>;
  // Context injection
  buildContextString?: () => string;
  autoInjectContext?: boolean;
  showContextChip?: boolean;
  inlineBadgesEnabled?: boolean;
}

export default function HorizontalRail({
  expanded,
  heightVh,
  tab,
  chatSplitPct,
  position,
  onToggleExpand,
  onSetTab,
  onHeightChange,
  activeSessionId,
  onSessionCreated,
  lastChatSnippet,
  tasks,
  activeProduct,
  taskActivities,
  taskStreams,
  taskComments,
  updateTask,
  moveTask,
  deleteTask,
  fetchComments,
  addComment,
  products,
  createTask,
  buildContextString,
  autoInjectContext,
  showContextChip,
  inlineBadgesEnabled,
}: HorizontalRailProps) {
  const dragRef = useRef<{ startY: number; startVh: number } | null>(null);
  const [selectedTask, setSelectedTask] = useState<PlannerTask | null>(null);
  const [showNewForm, setShowNewForm] = useState(false);

  // Filter tasks for active product
  const visibleTasks = activeProduct
    ? tasks.filter((t) => t.product === activeProduct)
    : tasks;

  // Counts for rail bar
  const activeTasks = visibleTasks.filter((t) => t.stage === 'active').length;
  const blockedTasks = visibleTasks.filter((t) => t.stage === 'blocked').length;

  // ── Drag-to-resize handle ────────────────────────────────
  const onDragStart = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    dragRef.current = { startY: e.clientY, startVh: heightVh };

    const onMove = (me: MouseEvent) => {
      if (!dragRef.current) return;
      const delta = dragRef.current.startY - me.clientY;
      const deltaVh = (delta / window.innerHeight) * 100;
      const newVh = dragRef.current.startVh + (position === 'bottom' ? deltaVh : -deltaVh);
      onHeightChange(Math.min(60, Math.max(20, Math.round(newVh))));
    };

    const onUp = () => {
      dragRef.current = null;
      document.removeEventListener('mousemove', onMove);
      document.removeEventListener('mouseup', onUp);
    };

    document.addEventListener('mousemove', onMove);
    document.addEventListener('mouseup', onUp);
  }, [heightVh, onHeightChange, position]);

  // Keep selectedTask in sync with live task data
  useEffect(() => {
    if (!selectedTask) return;
    const updated = tasks.find((t) => t.id === selectedTask.id);
    if (updated && updated !== selectedTask) setSelectedTask(updated);
  }, [tasks, selectedTask]);

  const railBar = (
    <div
      className="flex items-center h-12 px-4 gap-4 bg-surface cursor-pointer select-none shrink-0"
      style={{
        borderTopWidth: position === 'bottom' ? 1 : 0,
        borderBottomWidth: position === 'top' ? 1 : 0,
        borderColor: 'var(--color-border-subtle)',
        borderStyle: 'solid',
      }}
      onClick={onToggleExpand}
    >
      {/* Soul + last snippet */}
      <div className="flex items-center gap-2 flex-1 min-w-0">
        <span className="text-soul text-base leading-none shrink-0">&#9670;</span>
        <span className="text-xs text-fg-muted truncate">
          {lastChatSnippet ?? 'Message Soul...'}
        </span>
      </div>

      {/* Task counts */}
      <div
        className="flex items-center gap-2 shrink-0"
        onClick={(e) => { e.stopPropagation(); onSetTab('tasks'); }}
      >
        {activeTasks > 0 && (
          <span className="text-[11px] px-2 py-0.5 rounded bg-stage-active/15 text-stage-active font-medium">
            · {activeTasks} active
          </span>
        )}
        {blockedTasks > 0 && (
          <span className="text-[11px] px-2 py-0.5 rounded bg-stage-blocked/15 text-stage-blocked font-medium">
            · {blockedTasks} blocked
          </span>
        )}
        {activeTasks === 0 && blockedTasks === 0 && (
          <span className="text-[11px] text-fg-muted">
            {visibleTasks.length} tasks
          </span>
        )}
      </div>

      {/* Expand arrow */}
      <svg
        width="14"
        height="14"
        viewBox="0 0 14 14"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        className="text-fg-muted shrink-0"
        style={{ transform: expanded ? 'rotate(0deg)' : (position === 'bottom' ? 'rotate(180deg)' : 'rotate(0deg)') }}
      >
        <path d="M2 10l5-5 5 5" />
      </svg>
    </div>
  );

  const expandedPanel = (
    <div
      className="flex flex-col bg-surface shrink-0"
      style={{
        height: `${heightVh}vh`,
        borderTopWidth: position === 'bottom' ? 1 : 0,
        borderBottomWidth: position === 'top' ? 1 : 0,
        borderColor: 'var(--color-border-subtle)',
        borderStyle: 'solid',
      }}
    >
      {/* Drag handle — on top edge for bottom rail */}
      {position === 'bottom' && (
        <div
          onMouseDown={onDragStart}
          className="h-1 w-full cursor-row-resize shrink-0 hover:bg-soul/30 transition-colors"
          title="Drag to resize"
        />
      )}

      {/* Tab bar */}
      <div className="flex items-center gap-0 px-4 border-b border-border-subtle shrink-0 h-10">
        <button
          type="button"
          onClick={() => onSetTab('chat')}
          className={`flex items-center gap-1.5 px-3 h-full text-xs font-display font-semibold border-b-2 transition-colors cursor-pointer ${
            tab === 'chat'
              ? 'text-soul border-soul'
              : 'text-fg-muted border-transparent hover:text-fg'
          }`}
        >
          <span className="text-base leading-none">&#9670;</span>
          Chat
        </button>
        <button
          type="button"
          onClick={() => onSetTab('tasks')}
          className={`flex items-center gap-1.5 px-3 h-full text-xs font-display font-semibold border-b-2 transition-colors cursor-pointer ${
            tab === 'tasks'
              ? 'text-soul border-soul'
              : 'text-fg-muted border-transparent hover:text-fg'
          }`}
        >
          <svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
            <path d="M3 4l2 2 4-4" />
            <path d="M3 10l2 2 4-4" />
          </svg>
          Tasks
          {visibleTasks.length > 0 && (
            <span className="text-[10px] bg-overlay px-1.5 py-0.5 rounded-full text-fg-muted">
              {visibleTasks.length}
            </span>
          )}
        </button>

        <div className="flex-1" />

        {/* New Task shortcut */}
        {tab === 'tasks' && (
          <button
            type="button"
            onClick={() => setShowNewForm(true)}
            className="text-xs bg-soul/10 hover:bg-soul/20 text-soul px-2.5 py-1 rounded font-display font-semibold transition-colors cursor-pointer"
          >
            + New
          </button>
        )}

        {/* Collapse */}
        <button
          type="button"
          onClick={onToggleExpand}
          className="ml-2 w-7 h-7 flex items-center justify-center rounded hover:bg-elevated text-fg-muted hover:text-fg transition-colors cursor-pointer"
          title="Collapse"
        >
          <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
            <path d="M2 4l5 5 5-5" />
          </svg>
        </button>
      </div>

      {/* Content split: Chat | Tasks */}
      <div className="flex flex-1 min-h-0 overflow-hidden">
        {/* Chat pane */}
        <div
          className="flex flex-col min-h-0 overflow-hidden border-r border-border-subtle"
          style={{ width: `${chatSplitPct}%` }}
        >
          <ChatView
            activeSessionId={activeSessionId}
            onSessionCreated={onSessionCreated}
            activeProduct={activeProduct}
            buildContextString={buildContextString}
            autoInjectContext={autoInjectContext}
            showContextChip={showContextChip}
          />
        </div>

        {/* Tasks pane — list view */}
        <div
          className="flex flex-col min-h-0 overflow-hidden"
          style={{ width: `${100 - chatSplitPct}%` }}
        >
          <ListView
            tasks={visibleTasks}
            onTaskClick={setSelectedTask}
          />
        </div>
      </div>

      {/* Drag handle — on bottom edge for top rail */}
      {position === 'top' && (
        <div
          onMouseDown={onDragStart}
          className="h-1 w-full cursor-row-resize shrink-0 hover:bg-soul/30 transition-colors"
          title="Drag to resize"
        />
      )}
    </div>
  );

  return (
    <>
      {expanded ? expandedPanel : railBar}

      {/* Task detail modal */}
      {selectedTask && (
        <TaskDetail
          task={tasks.find((t) => t.id === selectedTask.id) ?? selectedTask}
          onClose={() => setSelectedTask(null)}
          onMove={async (id, stage, comment) => {
            await moveTask(id, stage, comment);
            setSelectedTask(null);
          }}
          onUpdate={async (id, updates) => {
            const updated = await updateTask(id, updates);
            setSelectedTask(updated);
            return updated;
          }}
          onDelete={async (id) => {
            await deleteTask(id);
            setSelectedTask(null);
          }}
          activities={taskActivities[selectedTask.id] || []}
          streamContent={taskStreams[selectedTask.id] || ''}
          products={products}
          comments={taskComments[selectedTask.id]}
          onFetchComments={fetchComments}
          onAddComment={addComment}
        />
      )}

      {showNewForm && (
        <NewTaskForm
          onClose={() => setShowNewForm(false)}
          onCreate={async (title, description, priority, product) => {
            await createTask(title, description, priority, product);
            setShowNewForm(false);
          }}
        />
      )}
    </>
  );
}
