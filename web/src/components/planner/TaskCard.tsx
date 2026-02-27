import type { PlannerTask, TaskSubstep } from '../../lib/types.ts';

const PRIORITY_BORDER: Record<number, string> = {
  0: 'border-l-zinc-600',
  1: 'border-l-sky-500',
  2: 'border-l-amber-500',
  3: 'border-l-red-500',
};

const SUBSTEP_LABELS: Record<TaskSubstep, string> = {
  tdd: 'TDD',
  implementing: 'Implementing',
  reviewing: 'Reviewing',
  qa_test: 'QA Test',
  e2e_test: 'E2E Test',
  security_review: 'Security Review',
};

const SUBSTEP_ORDER: TaskSubstep[] = [
  'tdd',
  'implementing',
  'reviewing',
  'qa_test',
  'e2e_test',
  'security_review',
];

interface TaskCardProps {
  task: PlannerTask;
  onClick: (task: PlannerTask) => void;
}

export default function TaskCard({ task, onClick }: TaskCardProps) {
  const borderClass = PRIORITY_BORDER[task.priority] ?? 'border-l-zinc-600';
  const substepIndex = task.substep ? SUBSTEP_ORDER.indexOf(task.substep) + 1 : 0;
  const substepLabel = task.substep ? SUBSTEP_LABELS[task.substep] : null;

  return (
    <button
      type="button"
      onClick={() => onClick(task)}
      className={`w-full text-left bg-zinc-900 rounded-lg border-l-4 ${borderClass} border border-zinc-800 p-3 hover:bg-zinc-800/80 transition-colors cursor-pointer`}
    >
      <div className="flex items-start justify-between gap-2 mb-1">
        <h4 className="text-sm font-medium text-zinc-100 leading-snug line-clamp-2">
          {task.title}
        </h4>
        <span className="text-xs text-zinc-500 shrink-0">#{task.id}</span>
      </div>

      {task.description && (
        <p className="text-xs text-zinc-400 line-clamp-2 mb-2">
          {task.description}
        </p>
      )}

      <div className="flex flex-wrap gap-1.5">
        {task.product && (
          <span className="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-zinc-800 text-zinc-300">
            {task.product}
          </span>
        )}
        {substepLabel && (
          <span className="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-sky-900/50 text-sky-300">
            {substepLabel} [{substepIndex}/6]
          </span>
        )}
        {task.blocker && (
          <span className="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-red-900/50 text-red-300">
            Blocked
          </span>
        )}
      </div>
    </button>
  );
}
