import { useState } from 'react';
import type { PlannerTask, TaskStage, TaskSubstep } from '../../lib/types.ts';

interface ListViewProps {
  tasks: PlannerTask[];
  onTaskClick: (task: PlannerTask) => void;
}

/** Active-first ordering (not pipeline order) */
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

export default function ListView({ tasks, onTaskClick }: ListViewProps) {
  // Group tasks by stage
  const grouped: Record<TaskStage, PlannerTask[]> = {
    active: [], backlog: [], brainstorm: [], blocked: [], validation: [], done: [],
  };
  for (const t of tasks) {
    grouped[t.stage].push(t);
  }

  // Non-empty stages start expanded, empty start collapsed
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
              className="flex items-center gap-2 w-full px-3 py-1.5 text-xs font-medium text-fg-secondary hover:bg-elevated cursor-pointer select-none"
            >
              <span className="text-[10px] w-3 text-center text-fg-muted">
                {isCollapsed ? '\u25B6' : '\u25BC'}
              </span>
              <span className={`w-2 h-2 rounded-full ${STAGE_COLORS[stage]} shrink-0`} />
              <span className="font-display text-[11px] font-semibold uppercase tracking-widest">{stage}</span>
              <span className="text-fg-muted ml-0.5">{stageTasks.length}</span>
            </button>

            {/* Task rows */}
            {!isCollapsed &&
              stageTasks.map((task) => {
                const prio = PRIORITY_CONFIG[task.priority] ?? PRIORITY_CONFIG[1];
                const sub = substepDisplay(task.substep);

                return (
                  <button
                    key={task.id}
                    type="button"
                    onClick={() => onTaskClick(task)}
                    className="flex flex-col gap-0.5 w-full px-3 py-1.5 pl-8 hover:bg-elevated cursor-pointer text-left"
                  >
                    {/* First line: id, title, priority, product, substep */}
                    <div className="flex items-center gap-3 text-xs">
                      <span className="text-fg-muted font-mono shrink-0 w-8 text-right">#{task.id}</span>
                      <span className="text-fg truncate flex-1 min-w-0">{task.title}</span>
                      <span className={`shrink-0 ${prio.color}`}>{prio.label}</span>
                      {task.product && (
                        <span className="bg-overlay text-fg-secondary shrink-0 truncate max-w-20 px-1.5 py-0.5 rounded text-[10px]">{task.product}</span>
                      )}
                      {sub && (
                        <span className="bg-stage-active/15 text-stage-active shrink-0 px-1.5 py-0.5 rounded text-[10px]">{sub}</span>
                      )}
                      {task.blocker && (
                        <span className="bg-stage-blocked/15 text-stage-blocked shrink-0 px-1.5 py-0.5 rounded text-[10px]">Blocked</span>
                      )}
                    </div>
                    {/* Second line: description preview */}
                    {task.description && (
                      <div className="text-[10px] text-fg-secondary truncate pl-11">{task.description}</div>
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
