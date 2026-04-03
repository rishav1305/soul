import type { TeamTask, TeamTaskStage } from '../../hooks/useTeamTasks.ts';
import { TaskCard } from './TaskCard.tsx';

/** KanbanColumn — column container for the task board. */

interface KanbanColumnProps {
  stage: TeamTaskStage;
  tasks: TeamTask[];
  onTaskClick: (id: string) => void;
  onAddTask?: () => void;
}

const STAGE_LABEL: Record<TeamTaskStage, string> = {
  backlog:     'Backlog',
  'in-progress': 'In Progress',
  blocked:     'Blocked',
  done:        'Done',
};

const STAGE_COLOR: Record<TeamTaskStage, string> = {
  backlog:     'text-fg-muted',
  'in-progress': 'text-blue-400',
  blocked:     'text-red-400',
  done:        'text-green-400',
};

const STAGE_DOT: Record<TeamTaskStage, string> = {
  backlog:     'bg-zinc-600',
  'in-progress': 'bg-blue-500',
  blocked:     'bg-red-500',
  done:        'bg-green-500',
};

export function KanbanColumn({ stage, tasks, onTaskClick, onAddTask }: KanbanColumnProps) {
  const label = STAGE_LABEL[stage];
  const color = STAGE_COLOR[stage];
  const dot = STAGE_DOT[stage];

  return (
    <div
      data-testid={`kanban-column-${stage}`}
      className="flex flex-col bg-surface/40 rounded-xl border border-border-subtle min-w-0 w-full"
      aria-label={`${label} column, ${tasks.length} tasks`}
    >
      {/* Column header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-border-subtle shrink-0">
        <div className="flex items-center gap-2">
          <span className={`w-2 h-2 rounded-full shrink-0 ${dot}`} aria-hidden="true" />
          <span className={`text-xs font-semibold uppercase tracking-wider ${color}`}>{label}</span>
          <span className="text-[10px] text-fg-muted bg-overlay/60 px-1.5 py-0.5 rounded-full">
            {tasks.length}
          </span>
        </div>
        {stage === 'backlog' && onAddTask && (
          <button
            type="button"
            data-testid="kanban-add-task-btn"
            onClick={onAddTask}
            className="w-6 h-6 flex items-center justify-center rounded text-fg-muted hover:text-fg hover:bg-overlay/60 transition-colors cursor-pointer"
            aria-label="Add task"
            title="Add task"
          >
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
              <path d="M7 2v10M2 7h10" />
            </svg>
          </button>
        )}
      </div>

      {/* Task list */}
      <div className="flex-1 overflow-y-auto p-3 space-y-2 min-h-[120px]">
        {tasks.length === 0 ? (
          <div className="flex items-center justify-center h-16 text-[11px] text-fg-muted italic">
            No tasks
          </div>
        ) : (
          tasks.map((task) => (
            <TaskCard key={task.id} task={task} onClick={onTaskClick} />
          ))
        )}
      </div>
    </div>
  );
}
