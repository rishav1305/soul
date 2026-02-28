import type { PlannerTask, TaskSubstep } from '../../lib/types.ts';

function parseMetadata(meta: string): Record<string, unknown> {
  try { return meta ? JSON.parse(meta) : {}; } catch { return {}; }
}

const PRIORITY_BORDER: Record<number, string> = {
  0: 'border-l-priority-low',
  1: 'border-l-priority-normal',
  2: 'border-l-priority-high',
  3: 'border-l-priority-critical',
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
  const borderClass = PRIORITY_BORDER[task.priority] ?? 'border-l-priority-low';
  const substepIndex = task.substep ? SUBSTEP_ORDER.indexOf(task.substep) + 1 : 0;
  const substepLabel = task.substep ? SUBSTEP_LABELS[task.substep] : null;
  const meta = parseMetadata(task.metadata);
  const isAutonomous = !!meta.autonomous;

  return (
    <button
      type="button"
      onClick={() => onClick(task)}
      className={`w-full text-left bg-elevated border-l-[3px] ${borderClass} border border-border-subtle rounded-lg p-3 hover:bg-overlay hover:border-border-default transition-all duration-150 cursor-pointer`}
    >
      <div className="flex items-start justify-between gap-2 mb-1">
        <h4 className="text-sm font-display font-medium text-fg leading-snug line-clamp-2">
          {task.title}
        </h4>
        <span className="text-[10px] text-fg-muted font-mono shrink-0">#{task.id}</span>
      </div>

      {task.description && (
        <p className="text-xs text-fg-secondary line-clamp-2 mb-2">
          {task.description}
        </p>
      )}

      <div className="flex flex-wrap gap-1.5">
        {isAutonomous && (
          <span className="inline-flex items-center gap-0.5 px-1.5 py-0.5 rounded text-[10px] font-medium bg-soul/15 text-soul">
            <svg width="10" height="10" viewBox="0 0 16 16" fill="currentColor"><path d="M8 1l2.5 5 5.5.8-4 3.9.9 5.3L8 13.3 3.1 16l.9-5.3-4-3.9L5.5 6z"/></svg>
            Auto
          </span>
        )}
        {task.product && (
          <span className="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-overlay text-fg-secondary">
            {task.product}
          </span>
        )}
        {substepLabel && (
          <span className="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-stage-active/15 text-stage-active">
            {substepLabel} [{substepIndex}/6]
          </span>
        )}
        {task.blocker && (
          <span className="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-stage-blocked/15 text-stage-blocked">
            Blocked
          </span>
        )}
      </div>
    </button>
  );
}
