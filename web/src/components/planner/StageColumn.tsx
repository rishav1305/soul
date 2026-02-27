import type { PlannerTask, TaskStage } from '../../lib/types.ts';
import TaskCard from './TaskCard.tsx';

const STAGE_COLORS: Record<TaskStage, string> = {
  backlog: 'text-zinc-400',
  brainstorm: 'text-purple-400',
  active: 'text-sky-400',
  blocked: 'text-red-400',
  validation: 'text-amber-400',
  done: 'text-green-400',
};

const STAGE_DOT_COLORS: Record<TaskStage, string> = {
  backlog: 'bg-zinc-400',
  brainstorm: 'bg-purple-400',
  active: 'bg-sky-400',
  blocked: 'bg-red-400',
  validation: 'bg-amber-400',
  done: 'bg-green-400',
};

const STAGE_LABELS: Record<TaskStage, string> = {
  backlog: 'Backlog',
  brainstorm: 'Brainstorm',
  active: 'Active',
  blocked: 'Blocked',
  validation: 'Validation',
  done: 'Done',
};

interface StageColumnProps {
  stage: TaskStage;
  tasks: PlannerTask[];
  onTaskClick: (task: PlannerTask) => void;
}

export default function StageColumn({ stage, tasks, onTaskClick }: StageColumnProps) {
  return (
    <div className="flex flex-col min-w-[220px] w-full">
      <div className="flex items-center gap-2 px-3 py-2 shrink-0">
        <span className={`w-2 h-2 rounded-full ${STAGE_DOT_COLORS[stage]}`} />
        <h3 className={`text-sm font-semibold uppercase tracking-wide ${STAGE_COLORS[stage]}`}>
          {STAGE_LABELS[stage]}
        </h3>
        <span className="ml-auto text-xs text-zinc-500 bg-zinc-800 px-1.5 py-0.5 rounded-full">
          {tasks.length}
        </span>
      </div>
      <div className="flex-1 overflow-y-auto px-2 pb-2 space-y-2">
        {tasks.map((task) => (
          <TaskCard key={task.id} task={task} onClick={onTaskClick} />
        ))}
        {tasks.length === 0 && (
          <div className="text-xs text-zinc-600 text-center py-8">
            No tasks
          </div>
        )}
      </div>
    </div>
  );
}
