import type { TaskStage, PlannerTask } from '../../lib/types.ts';

const STAGES: TaskStage[] = ['backlog', 'brainstorm', 'active', 'blocked', 'validation', 'done'];

const DOT_COLORS: Record<TaskStage, string> = {
  backlog: 'bg-zinc-400',
  brainstorm: 'bg-purple-400',
  active: 'bg-sky-400',
  blocked: 'bg-red-400',
  validation: 'bg-amber-400',
  done: 'bg-green-400',
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
      className="w-10 h-full bg-zinc-950 border-l border-zinc-800 flex flex-col items-center py-3 gap-2 shrink-0 cursor-pointer hover:bg-zinc-900 transition-colors"
    >
      {/* Expand icon */}
      <span className="text-lg text-zinc-400" title="Expand tasks">
        &#8862;
      </span>

      <div className="h-2" />

      {/* Stage dots with counts */}
      {STAGES.map((stage) => (
        <div key={stage} className="flex flex-col items-center gap-0.5">
          <span className={`w-2 h-2 rounded-full ${DOT_COLORS[stage]}`} />
          <span className="text-[9px] text-zinc-500 leading-none">
            {tasksByStage[stage].length}
          </span>
        </div>
      ))}
    </button>
  );
}
