import type { TaskStage, PlannerTask } from '../../lib/types.ts';

const STAGES: TaskStage[] = ['backlog', 'brainstorm', 'active', 'blocked', 'validation', 'done'];

const DOT_COLORS: Record<TaskStage, string> = {
  backlog: 'bg-stage-backlog',
  brainstorm: 'bg-stage-brainstorm',
  active: 'bg-stage-active',
  blocked: 'bg-stage-blocked',
  validation: 'bg-stage-validation',
  done: 'bg-stage-done',
};

interface TaskRailProps {
  tasksByStage: Record<TaskStage, PlannerTask[]>;
  onExpand: () => void;
}

export default function TaskRail({ tasksByStage, onExpand }: TaskRailProps) {
  return (
    <button
      type="button"
      onClick={onExpand}
      className="w-10 h-full bg-surface border-l border-border-subtle flex flex-col items-center py-3 gap-2 shrink-0 cursor-pointer hover:bg-elevated transition-colors"
    >
      {/* Expand icon */}
      <span className="text-lg text-fg-muted" title="Expand tasks">
        &#8862;
      </span>

      <div className="h-2" />

      {/* Stage dots with counts */}
      {STAGES.map((stage) => (
        <div key={stage} className="flex flex-col items-center gap-0.5">
          <span className={`w-2 h-2 rounded-full ${DOT_COLORS[stage]}`} />
          <span className="text-[9px] text-fg-muted leading-none">
            {tasksByStage[stage].length}
          </span>
        </div>
      ))}
    </button>
  );
}
