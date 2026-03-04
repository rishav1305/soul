import { useMemo } from 'react';
import type { PlannerTask, TaskSubstep, TaskStage } from '../../lib/types.ts';

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

const STAGE_DOT_COLOR: Record<TaskStage, string> = {
  backlog: 'bg-stage-backlog',
  brainstorm: 'bg-stage-brainstorm',
  active: 'bg-stage-active',
  blocked: 'bg-stage-blocked',
  validation: 'bg-stage-validation',
  done: 'bg-stage-done',
};

interface RecentActivity {
  stage: TaskStage;
  time: string; // ISO
}

interface TaskCardProps {
  task: PlannerTask;
  onClick: (task: PlannerTask) => void;
  recentActivity?: RecentActivity;
  inlineBadgesEnabled?: boolean;
}

export default function TaskCard({ task, onClick, recentActivity, inlineBadgesEnabled = true }: TaskCardProps) {
  const borderClass = PRIORITY_BORDER[task.priority] ?? 'border-l-priority-low';
  const substepIndex = task.substep ? SUBSTEP_ORDER.indexOf(task.substep) + 1 : 0;
  const substepLabel = task.substep ? SUBSTEP_LABELS[task.substep] : null;
  const meta = parseMetadata(task.metadata);
  const isAutonomous = !!meta.autonomous;
  const prio = PRIORITY_CONFIG[task.priority] ?? PRIORITY_CONFIG[1];
  const timeStr = relativeTime(task.created_at);

  // Determine if we should show the pulsing badge (within 60s)
  const showBadge = useMemo(() => {
    if (!inlineBadgesEnabled || !recentActivity) return false;
    const elapsed = Date.now() - new Date(recentActivity.time).getTime();
    return elapsed < 60_000;
  }, [inlineBadgesEnabled, recentActivity]);

  const badgeDotClass = recentActivity ? STAGE_DOT_COLOR[recentActivity.stage] : '';

  return (
    <button
      type="button"
      onClick={() => onClick(task)}
      className={`relative w-full text-left bg-elevated border-l-[3px] ${borderClass} border border-border-subtle rounded-lg p-3 hover:bg-overlay hover:border-border-default transition-all duration-150 cursor-pointer`}
    >
      {/* Inline stage-change badge */}
      {showBadge && (
        <span
          className={`absolute top-1.5 right-1.5 w-2 h-2 rounded-full animate-soul-pulse ${badgeDotClass}`}
          title={`Stage changed to ${recentActivity?.stage}`}
        />
      )}

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
        <span className={`inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-overlay ${prio.color}`}>
          {prio.label}
        </span>
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

      {timeStr && (
        <div className="text-[9px] text-fg-muted mt-1.5">{timeStr}</div>
      )}
    </button>
  );
}
