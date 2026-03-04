import type { PlannerTask, TaskStage } from '../../../lib/types.ts';

function parseMetadata(meta: string): Record<string, unknown> {
  try { return meta ? JSON.parse(meta) : {}; } catch { return {}; }
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

export default function CompactGrid({ tasks, onTaskClick }: CompactGridProps) {
  // Sort by priority descending
  const sorted = [...tasks].sort((a, b) => b.priority - a.priority);

  return (
    <div
      className="h-full overflow-y-auto p-3"
      style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))', gap: '8px', alignContent: 'start' }}
    >
      {sorted.map((task) => {
        const meta = parseMetadata(task.metadata);
        const isAutonomous = !!meta.autonomous;
        return (
          <button
            key={task.id}
            type="button"
            onClick={() => onTaskClick(task)}
            className={`text-left bg-elevated border-l-[3px] ${PRIORITY_BORDER[task.priority] ?? 'border-l-priority-low'} border rounded-lg p-3 hover:bg-overlay transition-all duration-150 cursor-pointer`}
            style={{ borderColor: STAGE_BORDER_COLOR[task.stage] }}
          >
            <div className="font-display text-xs font-medium text-fg truncate">{task.title}</div>
            <div className="flex items-center gap-1.5 mt-1.5">
              <span className="text-[10px] text-fg-muted font-mono">#{task.id}</span>
              {isAutonomous && (
                <span className="inline-flex items-center gap-0.5 px-1 py-px rounded text-[9px] font-medium bg-soul/15 text-soul ml-auto">
                  <svg width="8" height="8" viewBox="0 0 16 16" fill="currentColor"><path d="M8 1l2.5 5 5.5.8-4 3.9.9 5.3L8 13.3 3.1 16l.9-5.3-4-3.9L5.5 6z"/></svg>
                  Auto
                </span>
              )}
              {task.product && !isAutonomous && (
                <span className="text-[9px] text-fg-muted ml-auto truncate max-w-[60px]">{task.product}</span>
              )}
            </div>
          </button>
        );
      })}
    </div>
  );
}
