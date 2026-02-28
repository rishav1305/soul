import { useState } from 'react';
import type { PlannerTask, TaskStage, TaskSubstep } from '../../../lib/types.ts';

interface GroupedListProps {
  tasks: PlannerTask[];
  onTaskClick: (task: PlannerTask) => void;
}

/** Active-first ordering */
const STAGE_ORDER: TaskStage[] = ['active', 'backlog', 'brainstorm', 'blocked', 'validation', 'done'];

const STAGE_COLORS: Record<TaskStage, string> = {
  active: 'bg-stage-active',
  backlog: 'bg-stage-backlog',
  brainstorm: 'bg-stage-brainstorm',
  blocked: 'bg-stage-blocked',
  validation: 'bg-stage-validation',
  done: 'bg-stage-done',
};

const PRIORITY_CONFIG: Record<number, { label: string; color: string }> = {
  0: { label: 'Low', color: 'text-priority-low' },
  1: { label: 'Norm', color: 'text-priority-normal' },
  2: { label: 'High', color: 'text-priority-high' },
  3: { label: 'Crit', color: 'text-priority-critical' },
};

const SUBSTEP_LABELS: Record<TaskSubstep, string> = {
  tdd: 'TDD',
  implementing: 'Impl',
  reviewing: 'Review',
  qa_test: 'QA',
  e2e_test: 'E2E',
  security_review: 'SecRev',
};

const SUBSTEP_ORDER: TaskSubstep[] = ['tdd', 'implementing', 'reviewing', 'qa_test', 'e2e_test', 'security_review'];

function substepDisplay(substep: TaskSubstep | ''): string | null {
  if (!substep) return null;
  const idx = SUBSTEP_ORDER.indexOf(substep);
  if (idx === -1) return null;
  return `${SUBSTEP_LABELS[substep]} ${idx + 1}/${SUBSTEP_ORDER.length}`;
}

function relativeTime(dateStr: string): string {
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

export default function GroupedList({ tasks, onTaskClick }: GroupedListProps) {
  // Group tasks by stage
  const grouped: Record<TaskStage, PlannerTask[]> = {
    active: [], backlog: [], brainstorm: [], blocked: [], validation: [], done: [],
  };
  for (const t of tasks) {
    grouped[t.stage].push(t);
  }

  const [collapsed, setCollapsed] = useState<Record<string, boolean>>(() => {
    const init: Record<string, boolean> = {};
    for (const stage of STAGE_ORDER) {
      init[stage] = grouped[stage].length === 0;
    }
    return init;
  });

  const toggle = (stage: string) => {
    setCollapsed((prev) => ({ ...prev, [stage]: !prev[stage] }));
  };

  return (
    <div className="h-full overflow-y-auto">
      {STAGE_ORDER.map((stage) => {
        const stageTasks = grouped[stage];
        const isCollapsed = collapsed[stage];

        return (
          <div key={stage}>
            {/* Section header */}
            <button
              type="button"
              onClick={() => toggle(stage)}
              className="flex items-center gap-2 w-full px-3 py-1.5 font-display text-[11px] font-semibold uppercase tracking-widest text-fg-secondary hover:bg-elevated cursor-pointer select-none"
            >
              <span className="text-[10px] w-3 text-center">
                {isCollapsed ? '\u25B6' : '\u25BC'}
              </span>
              <span className={`w-2 h-2 rounded-full ${STAGE_COLORS[stage]} shrink-0`} />
              <span>{stage}</span>
              <span className="text-fg-muted ml-0.5">{stageTasks.length}</span>
            </button>

            {/* Task rows with more detail */}
            {!isCollapsed &&
              stageTasks.map((task) => {
                const prio = PRIORITY_CONFIG[task.priority] ?? PRIORITY_CONFIG[1];
                const sub = substepDisplay(task.substep);
                const desc = task.description
                  ? task.description.length > 60
                    ? task.description.slice(0, 60) + '\u2026'
                    : task.description
                  : '';
                const timeStr = relativeTime(task.created_at);

                return (
                  <button
                    key={task.id}
                    type="button"
                    onClick={() => onTaskClick(task)}
                    className="flex flex-col gap-0.5 w-full px-3 py-1.5 pl-8 hover:bg-elevated cursor-pointer text-left"
                  >
                    {/* First line: id, title, priority, substep */}
                    <div className="flex items-center gap-3 text-xs">
                      <span className="text-fg-muted shrink-0 w-8 text-right font-mono">#{task.id}</span>
                      <span className="text-fg truncate flex-1 min-w-0">{task.title}</span>
                      <span className={`shrink-0 ${prio.color}`}>{prio.label}</span>
                      {sub && <span className="text-fg-secondary shrink-0">{sub}</span>}
                    </div>
                    {/* Second line: description preview + timestamp */}
                    {(desc || timeStr) && (
                      <div className="flex items-center gap-3 text-[10px] pl-11">
                        {desc && <span className="text-fg-secondary truncate flex-1 min-w-0">{desc}</span>}
                        {timeStr && <span className="text-fg-muted shrink-0">{timeStr}</span>}
                      </div>
                    )}
                  </button>
                );
              })}
          </div>
        );
      })}
    </div>
  );
}
