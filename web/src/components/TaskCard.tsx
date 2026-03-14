import { Link } from 'react-router';
import type { Task } from '../lib/types';
import { formatRelativeTime } from '../lib/utils';
import { usePerformance } from '../hooks/usePerformance';

const WORKFLOW_BADGE: Record<string, string> = {
  micro: 'bg-green-900/40 text-green-400',
  quick: 'bg-blue-900/40 text-blue-400',
  full: 'bg-purple-900/40 text-purple-400',
};

interface TaskCardProps {
  task: Task;
  onStart?: (id: number) => void;
  onStop?: (id: number) => void;
}

export function TaskCard({ task, onStart, onStop }: TaskCardProps) {
  usePerformance('TaskCard');
  const isActive = task.stage === 'active';

  return (
    <div data-testid={`task-card-${task.id}`} className="p-3 rounded-lg bg-surface hover:bg-elevated transition-colors group">
      <div className="flex items-start justify-between gap-2">
        <Link to={`/tasks/${task.id}`} className="text-sm text-fg hover:text-soul transition-colors min-w-0 truncate flex-1">
          {task.title}
        </Link>
        {task.workflow && (
          <span className={`shrink-0 px-1.5 py-0.5 text-[10px] font-medium rounded ${WORKFLOW_BADGE[task.workflow] || 'bg-zinc-800 text-zinc-400'}`}>
            {task.workflow}
          </span>
        )}
      </div>

      {task.description && (
        <p className="text-xs text-fg-muted mt-1 line-clamp-2">{task.description}</p>
      )}

      <div className="flex items-center justify-between mt-2">
        <span className="text-[10px] text-fg-muted">{formatRelativeTime(task.updatedAt)}</span>
        <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
          {(task.stage === 'backlog' || task.stage === 'blocked') && onStart && (
            <button
              data-testid={`start-task-${task.id}`}
              onClick={() => onStart(task.id)}
              className="px-2 py-0.5 text-[10px] rounded bg-green-900/40 text-green-400 hover:bg-green-900/60 transition-colors"
            >
              Start
            </button>
          )}
          {isActive && onStop && (
            <button
              data-testid={`stop-task-${task.id}`}
              onClick={() => onStop(task.id)}
              className="px-2 py-0.5 text-[10px] rounded bg-red-900/40 text-red-400 hover:bg-red-900/60 transition-colors"
            >
              Stop
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
