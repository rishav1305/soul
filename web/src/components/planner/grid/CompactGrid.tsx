import type { PlannerTask, TaskStage } from '../../../lib/types.ts';

interface CompactGridProps {
  tasks: PlannerTask[];
  onTaskClick: (task: PlannerTask) => void;
}

const STAGE_COLORS: Record<TaskStage, string> = {
  active: 'bg-stage-active',
  backlog: 'bg-stage-backlog',
  brainstorm: 'bg-stage-brainstorm',
  blocked: 'bg-stage-blocked',
  validation: 'bg-stage-validation',
  done: 'bg-stage-done',
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
      {sorted.map((task) => (
        <button
          key={task.id}
          type="button"
          onClick={() => onTaskClick(task)}
          className={`text-left bg-elevated border-l-[3px] ${PRIORITY_BORDER[task.priority] ?? 'border-l-priority-low'} border border-border-subtle rounded-lg p-3 hover:bg-overlay hover:border-border-default transition-all duration-150 cursor-pointer`}
        >
          <div className="font-display text-xs font-medium text-fg truncate">{task.title}</div>
          <div className="flex items-center gap-1.5 mt-1.5">
            <span className="text-[10px] text-fg-muted font-mono">#{task.id}</span>
            <span className={`w-1.5 h-1.5 rounded-full ${STAGE_COLORS[task.stage]} shrink-0`} />
            <span className="text-[10px] text-fg-secondary uppercase">{task.stage}</span>
          </div>
        </button>
      ))}
    </div>
  );
}
