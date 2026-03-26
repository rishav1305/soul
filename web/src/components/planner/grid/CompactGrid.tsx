import type { PlannerTask, TaskStage } from '../../../lib/types.ts';

function parseMetadata(meta: string): Record<string, unknown> {
  try { return meta ? JSON.parse(meta) : {}; } catch { return {}; }
}

function relativeTime(dateStr: string): string {
  if (!dateStr) return '';
  const now = Date.now();
  const then = new Date(dateStr).getTime();
  if (isNaN(then)) return '';
  const diff = now - then;
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;
  return `${Math.floor(days / 30)}mo ago`;
}

interface CompactGridProps {
  tasks: PlannerTask[];
  onTaskClick: (task: PlannerTask) => void;
}

const STAGE_BORDER_COLOR: Record<TaskStage, string> = {
  active: 'var(--color-stage-active)',
  backlog: 'var(--color-stage-backlog)',
  brainstorm: 'var(--color-stage-brainstorm)',
  blocked: 'var(--color-stage-blocked)',
  validation: 'var(--color-stage-validation)',
  done: 'var(--color-stage-done)',
};

const PRIORITY_BORDER: Record<number, string> = {
  0: 'border-l-priority-low',
  1: 'border-l-priority-normal',
  2: 'border-l-priority-high',
  3: 'border-l-priority-critical',
};

const PRIORITY_CONFIG: Record<number, { label: string; color: string }> = {
  0: { label: 'Low', color: 'text-priority-low' },
  1: { label: 'Norm', color: 'text-priority-normal' },
  2: { label: 'High', color: 'text-priority-high' },
  3: { label: 'Crit', color: 'text-priority-critical' },
};

export default function CompactGrid({ tasks, onTaskClick }: CompactGridProps) {
  // Sort by priority descending
  const sorted = [...tasks].sort((a, b) => (b.priority ?? 0) - (a.priority ?? 0));

  return (
    <div
      className="h-full overflow-y-auto p-3"
      style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))', gap: '8px', alignContent: 'start' }}
    >
      {sorted.map((task) => {
        const meta = parseMetadata(task.metadata);
        const isAutonomous = !!meta.autonomous;
        const prio = PRIORITY_CONFIG[task.priority ?? 0] ?? { label: 'Norm', color: 'text-priority-normal' };
        const desc = task.description
          ? task.description.length > 80
            ? task.description.slice(0, 80) + '\u2026'
            : task.description
          : '';
        const timeStr = relativeTime(task.createdAt);

        return (
          <button
            key={task.id}
            type="button"
            onClick={() => onTaskClick(task)}
            data-testid={`grid-task-${task.id}`}
            className={`text-left bg-elevated border-l-[3px] ${PRIORITY_BORDER[task.priority ?? 0] ?? 'border-l-priority-low'} border rounded-lg p-3 hover:bg-overlay transition-all duration-150 cursor-pointer flex flex-col gap-1.5`}
            style={{ borderColor: STAGE_BORDER_COLOR[task.stage] }}
          >
            {/* Title row */}
            <div className="font-display text-xs font-medium text-fg line-clamp-2 leading-snug">{task.title}</div>

            {/* Description preview */}
            {desc && (
              <div className="text-[10px] text-fg-secondary leading-snug line-clamp-2">{desc}</div>
            )}

            {/* Meta row */}
            <div className="flex items-center gap-1.5 flex-wrap mt-auto">
              <span className="text-[10px] text-fg-muted font-mono">#{task.id}</span>
              <span className={`text-[10px] font-medium ${prio.color}`}>{prio.label}</span>
              <span className="text-[10px] text-fg-muted uppercase tracking-wide">{task.stage}</span>
              {isAutonomous && (
                <span className="inline-flex items-center gap-0.5 px-1 py-px rounded text-[9px] font-medium bg-soul/15 text-soul ml-auto">
                  <svg width="8" height="8" viewBox="0 0 16 16" fill="currentColor"><path d="M8 1l2.5 5 5.5.8-4 3.9.9 5.3L8 13.3 3.1 16l.9-5.3-4-3.9L5.5 6z"/></svg>
                  Auto
                </span>
              )}
              {task.product && (
                <span className={`text-[9px] text-fg-muted ${isAutonomous ? '' : 'ml-auto'} truncate max-w-[70px]`}>{task.product}</span>
              )}
              {task.blocker && (
                <span className="text-[9px] font-medium text-stage-blocked">Blocked</span>
              )}
            </div>

            {/* Date */}
            {timeStr && (
              <div className="text-[9px] text-fg-muted">{timeStr}</div>
            )}
          </button>
        );
      })}
    </div>
  );
}
