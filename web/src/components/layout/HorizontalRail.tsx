import { useRef, useCallback, useEffect, useState } from 'react';
import type {
  PlannerTask,
  TaskStage,
  TaskView,
  GridSubView,
  TaskFilters,
  TaskActivity,
  TaskComment,
  HorizontalRailTab,
  ChatSession,
} from '../../lib/types.ts';
import ChatView from '../chat/ChatView.tsx';
import SessionDrawer from '../chat/SessionDrawer.tsx';
import TaskContent from '../planner/TaskContent.tsx';
import TaskDetail from '../planner/TaskDetail.tsx';
import NewTaskForm from '../planner/NewTaskForm.tsx';

// ── Vertical split divider (shared between collapsed bar and expanded panel) ──
interface SplitDividerProps {
  onSplitChange: (pct: number) => void;
  containerRef: React.RefObject<HTMLDivElement | null>;
}

function SplitDivider({ onSplitChange, containerRef }: SplitDividerProps) {
  const dragRef = useRef<{ startX: number; startPct: number } | null>(null);

  const onMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    const container = containerRef.current;
    if (!container) return;
    const rect = container.getBoundingClientRect();
    const currentPct = ((e.clientX - rect.left) / rect.width) * 100;
    dragRef.current = { startX: e.clientX, startPct: currentPct };

    const onMove = (me: MouseEvent) => {
      if (!dragRef.current || !container) return;
      const r = container.getBoundingClientRect();
      const pct = ((me.clientX - r.left) / r.width) * 100;
      onSplitChange(Math.round(Math.min(80, Math.max(30, pct))));
    };

    const onUp = () => {
      dragRef.current = null;
      document.removeEventListener('mousemove', onMove);
      document.removeEventListener('mouseup', onUp);
    };

    document.addEventListener('mousemove', onMove);
    document.addEventListener('mouseup', onUp);
  }, [onSplitChange, containerRef]);

  return (
    <div
      onMouseDown={onMouseDown}
      className="w-1 self-stretch cursor-col-resize shrink-0 group relative z-10"
      title="Drag to resize panels"
    >
      <div className="absolute inset-y-0 -inset-x-1 group-hover:bg-soul/30 transition-colors rounded-sm" />
    </div>
  );
}

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
  onChatSplitChange: (pct: number) => void;
  // Chat
  activeSessionId: number | null;
  sessions: ChatSession[];
  onSessionCreated?: (id: number) => void;
  onSessionSelect: (id: number) => void;
  onNewSession: () => void;
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
  // Task view/filter state (mirrors TaskPanel)
  taskView: TaskView;
  gridSubView: GridSubView;
  filters: TaskFilters;
  setTaskView: (v: TaskView) => void;
  setGridSubView: (v: GridSubView) => void;
  setFilters: (partial: Partial<TaskFilters>) => void;
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
  onChatSplitChange,
  activeSessionId,
  sessions,
  onSessionCreated,
  onSessionSelect,
  onNewSession,
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
  taskView,
  gridSubView,
  filters,
  setTaskView,
  setGridSubView,
  setFilters,
  buildContextString,
  autoInjectContext,
  showContextChip,
  inlineBadgesEnabled,
}: HorizontalRailProps) {
  const dragRef = useRef<{ startY: number; startVh: number } | null>(null);
  // Ref attached to the full-width container so SplitDivider can measure it
  const railContainerRef = useRef<HTMLDivElement | null>(null);
  const [selectedTask, setSelectedTask] = useState<PlannerTask | null>(null);
  const [showNewForm, setShowNewForm] = useState(false);
  const [historyOpen, setHistoryOpen] = useState(false);
  const [showFilterPopover, setShowFilterPopover] = useState(false);
  const filterPopoverRef = useRef<HTMLDivElement | null>(null);

  // Close filter popover on outside click
  useEffect(() => {
    if (!showFilterPopover) return;
    const handler = (e: MouseEvent) => {
      if (filterPopoverRef.current && !filterPopoverRef.current.contains(e.target as Node)) {
        setShowFilterPopover(false);
      }
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [showFilterPopover]);

  // Filter tasks for active product
  const visibleTasks = activeProduct
    ? tasks.filter((t) => t.product === activeProduct)
    : tasks;

  // Apply stage/priority filters
  const filteredTasks = visibleTasks.filter((t) => {
    if (filters.stage !== 'all' && t.stage !== filters.stage) return false;
    if (filters.priority !== 'all' && t.priority !== filters.priority) return false;
    return true;
  });

  // Group filtered tasks by stage (for kanban)
  const tasksByStage = (() => {
    const grouped: Record<TaskStage, PlannerTask[]> = {
      backlog: [], brainstorm: [], active: [], blocked: [], validation: [], done: [],
    };
    for (const t of filteredTasks) grouped[t.stage].push(t);
    return grouped;
  })();

  // Count active filters (product excluded — already scoped via activeProduct)
  const activeFilterCount = [
    filters.stage !== 'all',
    filters.priority !== 'all',
  ].filter(Boolean).length;

  // Counts for rail bar (unfiltered — show true picture in collapsed state)
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
      ref={railContainerRef}
      className="flex items-stretch h-12 bg-surface select-none shrink-0"
      style={{
        borderTopWidth: position === 'bottom' ? 1 : 0,
        borderBottomWidth: position === 'top' ? 1 : 0,
        borderColor: 'var(--color-border-subtle)',
        borderStyle: 'solid',
      }}
    >
      {/* ── Left half: Chat snippet ── */}
      <div
        className="flex items-center px-4 gap-2 min-w-0 cursor-pointer"
        style={{ width: `${chatSplitPct}%` }}
        onClick={onToggleExpand}
      >
        <span className="text-soul text-base leading-none shrink-0">&#9670;</span>
        <span className="text-xs text-fg-muted truncate">
          {lastChatSnippet ?? 'Message Soul...'}
        </span>
        {/* Expand arrow */}
        <svg
          width="14"
          height="14"
          viewBox="0 0 14 14"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.5"
          strokeLinecap="round"
          className="text-fg-muted shrink-0 ml-auto"
          style={{ transform: expanded ? 'rotate(0deg)' : (position === 'bottom' ? 'rotate(180deg)' : 'rotate(0deg)') }}
        >
          <path d="M2 10l5-5 5 5" />
        </svg>
      </div>

      {/* ── Movable split divider ── */}
      <SplitDivider onSplitChange={onChatSplitChange} containerRef={railContainerRef} />

      {/* ── Right half: Task counts ── */}
      <div
        className="flex items-center px-4 gap-2 min-w-0 cursor-pointer"
        style={{ width: `${100 - chatSplitPct}%` }}
        onClick={(e) => { e.stopPropagation(); onSetTab('tasks'); if (!expanded) onToggleExpand(); }}
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
    </div>
  );

  const expandedPanel = (
    <div
      ref={railContainerRef}
      data-testid="horizontal-rail"
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

      {/* Split tab bar — left half = Chat, right half = Tasks */}
      <div
        className="flex items-stretch border-b border-border-subtle shrink-0 h-10"
      >
        {/* Chat tab header */}
        <div
          className="flex items-center px-4 gap-0 min-w-0"
          style={{ width: `${chatSplitPct}%` }}
        >
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
          <div className="flex-1" />
          {/* History button */}
          <button
            type="button"
            onClick={() => setHistoryOpen((o) => !o)}
            className={`w-7 h-7 flex items-center justify-center rounded transition-colors cursor-pointer ${
              historyOpen
                ? 'bg-soul/15 text-soul'
                : 'hover:bg-elevated text-fg-muted hover:text-fg'
            }`}
            title="Chat history"
          >
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="8" cy="8" r="6.5" />
              <path d="M8 4.5V8l2.5 2" />
            </svg>
          </button>
          {/* Collapse button */}
          <button
            type="button"
            onClick={() => { setHistoryOpen(false); onToggleExpand(); }}
            className="w-7 h-7 flex items-center justify-center rounded hover:bg-elevated text-fg-muted hover:text-fg transition-colors cursor-pointer"
            title="Collapse"
          >
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
              <path d="M2 4l5 5 5-5" />
            </svg>
          </button>
        </div>

        {/* Movable divider — lines up with content split below */}
        <SplitDivider onSplitChange={onChatSplitChange} containerRef={railContainerRef} />

        {/* Tasks tab header */}
        <div
          className="flex items-center px-2 gap-0 min-w-0"
          style={{ width: `${100 - chatSplitPct}%` }}
        >
          {/* Tasks tab label */}
          <button
            type="button"
            onClick={() => onSetTab('tasks')}
            className={`flex items-center gap-1.5 px-2 h-full text-xs font-display font-semibold border-b-2 transition-colors cursor-pointer shrink-0 ${
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

          {/* Divider */}
          <div className="w-px h-4 bg-border-subtle mx-1 shrink-0" />

          {/* View mode buttons */}
          {(
            [
              {
                view: 'list' as TaskView,
                title: 'List view',
                icon: (
                  <svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
                    <path d="M3 4h10M3 8h10M3 12h10" />
                  </svg>
                ),
              },
              {
                view: 'kanban' as TaskView,
                title: 'Kanban view',
                icon: (
                  <svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
                    <path d="M3 3v10M8 3v7M13 3v10" />
                  </svg>
                ),
              },
              {
                view: 'grid' as TaskView,
                title: 'Grid view',
                icon: (
                  <svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
                    <rect x="2" y="2" width="5" height="5" rx="1" />
                    <rect x="9" y="2" width="5" height="5" rx="1" />
                    <rect x="2" y="9" width="5" height="5" rx="1" />
                    <rect x="9" y="9" width="5" height="5" rx="1" />
                  </svg>
                ),
              },
              {
                view: 'table' as TaskView,
                title: 'Table view',
                icon: (
                  <svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                    <rect x="2" y="2" width="12" height="12" rx="1" />
                    <path d="M2 5.5h12" />
                    <path d="M6 5.5v8.5" />
                  </svg>
                ),
              },
            ] as { view: TaskView; title: string; icon: React.ReactNode }[]
          ).map(({ view, title, icon }) => (
            <button
              key={view}
              type="button"
              onClick={() => setTaskView(view)}
              title={title}
              className={`w-6 h-6 flex items-center justify-center rounded cursor-pointer transition-colors shrink-0 ${
                taskView === view
                  ? 'bg-overlay text-fg'
                  : 'text-fg-muted hover:text-fg hover:bg-elevated'
              }`}
            >
              {icon}
            </button>
          ))}

          {/* Divider */}
          <div className="w-px h-4 bg-border-subtle mx-1 shrink-0" />

          {/* Filter popover button */}
          <div className="relative shrink-0" ref={filterPopoverRef}>
            <button
              type="button"
              onClick={() => setShowFilterPopover((o) => !o)}
              title="Filters"
              className={`relative w-6 h-6 flex items-center justify-center rounded cursor-pointer transition-colors ${
                showFilterPopover
                  ? 'bg-soul/15 text-soul'
                  : activeFilterCount > 0
                  ? 'text-soul hover:bg-soul/10'
                  : 'text-fg-muted hover:text-fg hover:bg-elevated'
              }`}
            >
              <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
                <path d="M2 3h12M4 8h8M6 13h4" />
              </svg>
              {activeFilterCount > 0 && (
                <span className="absolute -top-1 -right-1 w-3.5 h-3.5 bg-soul text-deep text-[8px] font-bold rounded-full flex items-center justify-center leading-none">
                  {activeFilterCount}
                </span>
              )}
            </button>

            {/* Filter popover */}
            {showFilterPopover && (
              <div className="absolute top-full left-0 mt-1 bg-surface border border-border-default rounded-xl shadow-xl z-50 p-3 min-w-[200px]">
                <div className="flex flex-col gap-2 text-xs">
                  {/* Stage */}
                  <div className="flex flex-col gap-1">
                    <span className="text-[10px] font-display font-semibold text-fg-muted uppercase tracking-wider">Stage</span>
                    <select
                      className="soul-select text-xs"
                      value={filters.stage}
                      onChange={(e) => setFilters({ stage: e.target.value as TaskStage | 'all' })}
                    >
                      <option value="all">All Stages</option>
                      <option value="backlog">Backlog</option>
                      <option value="brainstorm">Brainstorm</option>
                      <option value="active">Active</option>
                      <option value="blocked">Blocked</option>
                      <option value="validation">Validation</option>
                      <option value="done">Done</option>
                    </select>
                  </div>
                  {/* Priority */}
                  <div className="flex flex-col gap-1">
                    <span className="text-[10px] font-display font-semibold text-fg-muted uppercase tracking-wider">Priority</span>
                    <select
                      className="soul-select text-xs"
                      value={filters.priority === 'all' ? 'all' : String(filters.priority)}
                      onChange={(e) => {
                        const val = e.target.value;
                        setFilters({ priority: val === 'all' ? 'all' : Number(val) });
                      }}
                    >
                      <option value="all">All Priorities</option>
                      <option value="3">Critical (3)</option>
                      <option value="2">High (2)</option>
                      <option value="1">Normal (1)</option>
                      <option value="0">Low (0)</option>
                    </select>
                  </div>
                  {/* Clear */}
                  {activeFilterCount > 0 && (
                    <button
                      type="button"
                      onClick={() => { setFilters({ stage: 'all', priority: 'all' }); setShowFilterPopover(false); }}
                      className="text-soul hover:text-soul/80 text-[10px] underline cursor-pointer text-left mt-0.5"
                    >
                      Clear filters
                    </button>
                  )}
                </div>
              </div>
            )}
          </div>

          <div className="flex-1" />

          {/* New Task button */}
          <button
            type="button"
            onClick={() => setShowNewForm(true)}
            className="text-xs bg-soul/10 hover:bg-soul/20 text-soul px-2 py-0.5 rounded font-display font-semibold transition-colors cursor-pointer shrink-0 whitespace-nowrap"
          >
            + New
          </button>
        </div>
      </div>

      {/* Content split: Chat | Tasks */}
      <div className="flex flex-1 min-h-0 overflow-hidden">
        {/* Chat pane */}
        <div
          className="flex flex-col min-h-0 overflow-hidden relative"
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
          {/* History drawer — slides in over the chat pane */}
          {historyOpen && (
            <SessionDrawer
              sessions={sessions}
              activeSessionId={activeSessionId}
              onSelect={(id) => { onSessionSelect(id); setHistoryOpen(false); }}
              onNew={() => { onNewSession(); setHistoryOpen(false); }}
              onClose={() => setHistoryOpen(false)}
            />
          )}
        </div>

        {/* Movable split divider in content area */}
        <SplitDivider onSplitChange={onChatSplitChange} containerRef={railContainerRef} />

        {/* Tasks pane */}
        <div
          className="flex flex-col min-h-0 overflow-hidden"
          style={{ width: `${100 - chatSplitPct}%` }}
        >
          <TaskContent
            taskView={taskView}
            filteredTasks={filteredTasks}
            tasksByStage={tasksByStage}
            gridSubView={gridSubView}
            onGridSubViewChange={setGridSubView}
            onTaskClick={setSelectedTask}
            onClearFilters={() => setFilters({ stage: 'all', priority: 'all' })}
            taskActivities={taskActivities}
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
