import { useState, useCallback } from 'react';
import { useTeamTasks, type TeamTask, type TeamTaskStage } from '../../hooks/useTeamTasks.ts';
import { KanbanColumn } from './KanbanColumn.tsx';

const STAGES: TeamTaskStage[] = ['backlog', 'in-progress', 'blocked', 'done'];
const AGENTS = ['fury', 'shuri', 'happy', 'xavier', 'pepper', 'loki', 'hawkeye', 'stark', 'banner'];
const TRACKS = ['frontend', 'backend', 'content', 'pipeline', 'product', 'data', 'interview', 'biz-dev', 'infra', 'migration'];
const PRIORITIES = ['critical', 'high', 'medium', 'low'];

/**
 * TaskBoard — 4-column Kanban board with filter bar and slide-in task detail.
 * Route: /team/tasks
 */
export default function TaskBoard() {
  const { tasksByStage, selectedTask, createTask, moveTask, selectTask, filterBy, activeFilters } = useTeamTasks();
  const [showAddTask, setShowAddTask] = useState(false);
  const [newTaskTitle, setNewTaskTitle] = useState('');
  const [newTaskAssignee, setNewTaskAssignee] = useState('');
  const [newTaskPriority, setNewTaskPriority] = useState<TeamTask['priority']>('medium');

  const handleAddTask = useCallback(() => {
    const title = newTaskTitle.trim();
    if (!title) return;
    createTask(title, newTaskAssignee, newTaskPriority);
    setNewTaskTitle('');
    setNewTaskAssignee('');
    setNewTaskPriority('medium');
    setShowAddTask(false);
  }, [newTaskTitle, newTaskAssignee, newTaskPriority, createTask]);

  return (
    <div data-testid="task-board" className="flex flex-col h-full bg-deep overflow-hidden">
      {/* ── Header ── */}
      <header className="flex items-center justify-between px-6 py-3 border-b border-border-subtle shrink-0">
        <h1 className="text-sm font-semibold text-fg">Task Board</h1>
        <button
          type="button"
          data-testid="task-board-add-btn"
          onClick={() => setShowAddTask(true)}
          className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-soul text-deep rounded hover:bg-soul/85 transition-colors cursor-pointer font-medium"
          aria-label="Add new task"
        >
          <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
            <path d="M6 1v10M1 6h10" />
          </svg>
          Add Task
        </button>
      </header>

      {/* ── Filter bar ── */}
      <div
        data-testid="task-board-filters"
        className="flex items-center gap-3 px-6 py-2 border-b border-border-subtle shrink-0 overflow-x-auto"
        aria-label="Task filters"
      >
        <span className="text-[11px] text-fg-muted shrink-0">Filter:</span>

        <select
          data-testid="filter-assignee"
          value={activeFilters.assignee}
          onChange={(e) => filterBy({ ...activeFilters, assignee: e.target.value })}
          className="bg-surface border border-border-subtle rounded px-2 py-1 text-xs text-fg-secondary focus:outline-none focus:border-soul/50 cursor-pointer"
          aria-label="Filter by agent"
        >
          <option value="">All agents</option>
          {AGENTS.map((a) => <option key={a} value={a}>{a}</option>)}
        </select>

        <select
          data-testid="filter-track"
          value={activeFilters.track}
          onChange={(e) => filterBy({ ...activeFilters, track: e.target.value })}
          className="bg-surface border border-border-subtle rounded px-2 py-1 text-xs text-fg-secondary focus:outline-none focus:border-soul/50 cursor-pointer"
          aria-label="Filter by track"
        >
          <option value="">All tracks</option>
          {TRACKS.map((t) => <option key={t} value={t}>{t}</option>)}
        </select>

        <select
          data-testid="filter-priority"
          value={activeFilters.priority}
          onChange={(e) => filterBy({ ...activeFilters, priority: e.target.value })}
          className="bg-surface border border-border-subtle rounded px-2 py-1 text-xs text-fg-secondary focus:outline-none focus:border-soul/50 cursor-pointer"
          aria-label="Filter by priority"
        >
          <option value="">All priorities</option>
          {PRIORITIES.map((p) => <option key={p} value={p}>{p}</option>)}
        </select>

        {(activeFilters.assignee || activeFilters.track || activeFilters.priority) && (
          <button
            type="button"
            data-testid="filter-clear-btn"
            onClick={() => filterBy({ assignee: '', track: '', priority: '' })}
            className="text-xs text-soul hover:text-soul/80 transition-colors cursor-pointer shrink-0"
          >
            Clear filters
          </button>
        )}
      </div>

      {/* ── Add task form ── */}
      {showAddTask && (
        <div
          data-testid="add-task-form"
          className="px-6 py-3 border-b border-border-subtle bg-surface/60 flex items-center gap-3 shrink-0"
        >
          <input
            data-testid="new-task-title-input"
            type="text"
            value={newTaskTitle}
            onChange={(e) => setNewTaskTitle(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') handleAddTask();
              if (e.key === 'Escape') setShowAddTask(false);
            }}
            placeholder="Task title..."
            className="flex-1 bg-deep border border-border-default rounded px-3 py-1.5 text-sm text-fg placeholder:text-fg-muted focus:outline-none focus:border-soul/50"
            autoFocus
          />

          <select
            data-testid="new-task-assignee-select"
            value={newTaskAssignee}
            onChange={(e) => setNewTaskAssignee(e.target.value)}
            className="bg-deep border border-border-default rounded px-2 py-1.5 text-xs text-fg-secondary focus:outline-none cursor-pointer"
            aria-label="Assignee"
          >
            <option value="">Assign...</option>
            {AGENTS.map((a) => <option key={a} value={a}>{a}</option>)}
          </select>

          <select
            data-testid="new-task-priority-select"
            value={newTaskPriority}
            onChange={(e) => setNewTaskPriority(e.target.value as TeamTask['priority'])}
            className="bg-deep border border-border-default rounded px-2 py-1.5 text-xs text-fg-secondary focus:outline-none cursor-pointer"
            aria-label="Priority"
          >
            <option value="critical">P0 Critical</option>
            <option value="high">P1 High</option>
            <option value="medium">P2 Medium</option>
            <option value="low">P3 Low</option>
          </select>

          <button
            type="button"
            data-testid="new-task-submit-btn"
            onClick={handleAddTask}
            disabled={!newTaskTitle.trim()}
            className="px-3 py-1.5 text-xs bg-soul text-deep rounded hover:bg-soul/85 disabled:opacity-40 disabled:cursor-not-allowed transition-colors cursor-pointer font-medium"
          >
            Create
          </button>
          <button
            type="button"
            data-testid="new-task-cancel-btn"
            onClick={() => setShowAddTask(false)}
            className="px-3 py-1.5 text-xs text-fg-muted hover:text-fg transition-colors cursor-pointer"
          >
            Cancel
          </button>
        </div>
      )}

      {/* ── Kanban columns ── */}
      <div className="flex-1 overflow-x-auto overflow-y-hidden">
        <div className="flex h-full gap-4 p-4 min-w-[900px]">
          {STAGES.map((stage) => (
            <div key={stage} className="flex-1 min-w-0 flex flex-col overflow-hidden">
              <KanbanColumn
                stage={stage}
                tasks={tasksByStage[stage]}
                onTaskClick={selectTask}
                onAddTask={stage === 'backlog' ? () => setShowAddTask(true) : undefined}
              />
            </div>
          ))}
        </div>
      </div>

      {/* ── Task detail slide-in ── */}
      {selectedTask && (
        <div
          data-testid={`task-detail-panel-${selectedTask.id}`}
          className="fixed inset-y-0 right-0 w-96 bg-elevated border-l border-border-subtle shadow-2xl flex flex-col z-20"
          role="dialog"
          aria-label={`Task detail: ${selectedTask.title}`}
          aria-modal="true"
        >
          <div className="flex items-center justify-between px-5 py-4 border-b border-border-subtle shrink-0">
            <h2 className="text-sm font-semibold text-fg">Task Detail</h2>
            <button
              type="button"
              data-testid="task-detail-close-btn"
              onClick={() => selectTask(null)}
              className="w-7 h-7 flex items-center justify-center rounded text-fg-muted hover:text-fg hover:bg-overlay/60 transition-colors cursor-pointer"
              aria-label="Close task detail"
            >
              <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
                <path d="M3 3l8 8M11 3l-8 8" />
              </svg>
            </button>
          </div>

          <div className="flex-1 overflow-y-auto p-5 space-y-4">
            <h3 className="text-base font-medium text-fg leading-snug">{selectedTask.title}</h3>

            {selectedTask.description && (
              <p className="text-sm text-fg-secondary leading-relaxed">{selectedTask.description}</p>
            )}

            <div className="space-y-2">
              {selectedTask.assignee && (
                <div className="flex items-center justify-between text-xs">
                  <span className="text-fg-muted">Assignee</span>
                  <span className="text-fg-secondary">{selectedTask.assignee}</span>
                </div>
              )}
              <div className="flex items-center justify-between text-xs">
                <span className="text-fg-muted">Priority</span>
                <span className="text-fg-secondary capitalize">{selectedTask.priority}</span>
              </div>
              {selectedTask.track && (
                <div className="flex items-center justify-between text-xs">
                  <span className="text-fg-muted">Track</span>
                  <span className="text-fg-secondary">{selectedTask.track}</span>
                </div>
              )}
            </div>

            {/* Move buttons */}
            <div className="space-y-2">
              <p className="text-[11px] text-fg-muted uppercase tracking-wider font-medium">Move to</p>
              <div className="grid grid-cols-2 gap-2">
                {STAGES.filter((s) => s !== selectedTask.stage).map((s) => (
                  <button
                    key={s}
                    type="button"
                    data-testid={`move-task-${s}`}
                    onClick={() => {
                      moveTask(selectedTask.id, s);
                      selectTask(null);
                    }}
                    className="px-2 py-1.5 text-xs border border-border-default text-fg-secondary hover:text-fg hover:border-border-active rounded transition-colors cursor-pointer capitalize"
                  >
                    {s === 'in-progress' ? 'In Progress' : s.charAt(0).toUpperCase() + s.slice(1)}
                  </button>
                ))}
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
