import type { TeamTask } from '../../hooks/useTeamTasks.ts';

/** TaskCard — kanban card with priority indicator and drag handle placeholder. */

interface TaskCardProps {
  task: TeamTask;
  onClick: (id: string) => void;
}

const PRIORITY_COLOR: Record<string, string> = {
  critical: 'bg-red-500',
  high:     'bg-orange-500',
  medium:   'bg-yellow-500',
  low:      'bg-zinc-500',
};

const PRIORITY_LABEL: Record<string, string> = {
  critical: 'P0',
  high:     'P1',
  medium:   'P2',
  low:      'P3',
};

const ASSIGNEE_COLOR: Record<string, string> = {
  fury:    'bg-red-700',
  shuri:   'bg-purple-700',
  happy:   'bg-yellow-700',
  xavier:  'bg-blue-700',
  pepper:  'bg-pink-700',
  loki:    'bg-green-700',
  hawkeye: 'bg-orange-700',
  stark:   'bg-cyan-700',
  banner:  'bg-emerald-700',
};

function formatDate(ts: number | undefined): string {
  if (!ts) return '';
  const d = new Date(ts);
  return d.toLocaleDateString([], { month: 'short', day: 'numeric' });
}

export function TaskCard({ task, onClick }: TaskCardProps) {
  const priorityColor = PRIORITY_COLOR[task.priority] ?? PRIORITY_COLOR.medium;
  const priorityLabel = PRIORITY_LABEL[task.priority] ?? 'P2';
  const assigneeBg = ASSIGNEE_COLOR[task.assignee ?? ''] ?? 'bg-zinc-600';

  return (
    <button
      type="button"
      data-testid={`team-task-card-${task.id}`}
      onClick={() => onClick(task.id)}
      className="w-full text-left bg-surface hover:bg-elevated border border-border-subtle hover:border-border-default rounded-lg p-3 transition-all cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-soul/50 group"
      aria-label={`Task: ${task.title}, priority ${task.priority}`}
    >
      {/* Priority + title row */}
      <div className="flex items-start gap-2 mb-2">
        {/* Priority dot */}
        <span
          data-testid={`task-priority-dot-${task.id}`}
          className={`mt-0.5 shrink-0 w-2 h-2 rounded-full ${priorityColor}`}
          aria-label={`Priority: ${task.priority}`}
        />
        <span className="text-sm text-fg leading-snug line-clamp-2">{task.title}</span>
      </div>

      {/* Meta row */}
      <div className="flex items-center justify-between gap-2 mt-2">
        {/* Assignee badge */}
        <div className="flex items-center gap-1.5">
          {task.assignee && (
            <div
              className={`w-5 h-5 rounded-full flex items-center justify-center text-[9px] font-bold text-white ${assigneeBg}`}
              aria-label={`Assigned to ${task.assignee}`}
            >
              {task.assignee.slice(0, 2).toUpperCase()}
            </div>
          )}
          {task.track && (
            <span className="text-[10px] text-fg-muted bg-overlay/60 px-1.5 py-0.5 rounded">
              {task.track}
            </span>
          )}
        </div>

        {/* Due date + priority label */}
        <div className="flex items-center gap-1.5">
          {task.dueDate && (
            <span className="text-[10px] text-fg-muted">{formatDate(task.dueDate)}</span>
          )}
          <span className={`text-[9px] font-bold px-1 py-0.5 rounded bg-opacity-20 ${priorityColor} text-white`}>
            {priorityLabel}
          </span>
        </div>
      </div>
    </button>
  );
}
