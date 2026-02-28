import type { PlannerTask, TaskStage } from '../../../lib/types.ts';

interface CompactGridProps {
  tasks: PlannerTask[];
  onTaskClick: (task: PlannerTask) => void;
}

const STAGE_COLORS: Record<TaskStage, string> = {
  active: 'bg-green-500',
  backlog: 'bg-zinc-500',
  brainstorm: 'bg-violet-500',
  blocked: 'bg-red-500',
  validation: 'bg-amber-500',
  done: 'bg-sky-500',
};

const PRIORITY_BORDER: Record<number, string> = {
  0: 'border-l-zinc-600',
  1: 'border-l-zinc-400',
  2: 'border-l-amber-400',
  3: 'border-l-red-500',
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
          className={`text-left bg-zinc-900 rounded border-l-4 ${PRIORITY_BORDER[task.priority] ?? 'border-l-zinc-600'} p-2.5 hover:bg-zinc-800 cursor-pointer transition-colors`}
        >
          <div className="text-xs text-zinc-200 font-medium truncate">{task.title}</div>
          <div className="flex items-center gap-1.5 mt-1.5">
            <span className="text-[10px] text-zinc-500">#{task.id}</span>
            <span className={`w-1.5 h-1.5 rounded-full ${STAGE_COLORS[task.stage]} shrink-0`} />
            <span className="text-[10px] text-zinc-500 uppercase">{task.stage}</span>
          </div>
        </button>
      ))}
    </div>
  );
}
