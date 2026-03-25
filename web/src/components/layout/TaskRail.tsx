import { useState } from 'react';
import type { TaskStage, PlannerTask } from '../../lib/types.ts';
import NewTaskForm from '../planner/NewTaskForm.tsx';

const STAGES: TaskStage[] = ['backlog', 'brainstorm', 'active', 'blocked', 'validation', 'done'];

const STAGE_BG: Record<TaskStage, string> = {
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
  onNewTask: (title: string, description: string, priority: number, product: string) => Promise<void>;
}

export default function TaskRail({ tasksByStage, onExpand, onNewTask }: TaskRailProps) {
  const [showNewForm, setShowNewForm] = useState(false);

  return (
    <>
      <div className="w-10 h-full bg-surface border-l border-border-subtle flex flex-col items-center py-3 gap-2 shrink-0">
        {/* Expand icon */}
        <button
          type="button"
          onClick={onExpand}
          className="w-7 h-7 flex items-center justify-center rounded hover:bg-elevated text-fg-muted hover:text-fg transition-colors cursor-pointer"
          title="Expand tasks"
        >
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.2">
            <rect x="1.5" y="2.5" width="13" height="11" rx="1.5" />
            <line x1="10.5" y1="2.5" x2="10.5" y2="13.5" />
          </svg>
        </button>

        {/* New task button */}
        <button
          type="button"
          onClick={(e) => { e.stopPropagation(); setShowNewForm(true); }}
          className="w-7 h-7 rounded bg-soul/80 hover:bg-soul text-deep flex items-center justify-center transition-colors cursor-pointer font-bold text-sm"
          title="New task"
        >
          +
        </button>

        <div className="h-1" />

        {/* Stage count boxes */}
        {STAGES.map((stage) => {
          const count = tasksByStage[stage].length;
          return (
            <div
              key={stage}
              className={`w-7 h-7 rounded-sm flex items-center justify-center ${STAGE_BG[stage]} ${count === 0 ? 'opacity-30' : ''} transition-opacity`}
              title={`${stage}: ${count}`}
            >
              <span className="text-deep font-mono text-[11px] font-bold leading-none">
                {count}
              </span>
            </div>
          );
        })}
      </div>

      {showNewForm && (
        <NewTaskForm
          onClose={() => setShowNewForm(false)}
          onCreate={async (title, desc, priority, product) => {
            await onNewTask(title, desc, priority, product);
            setShowNewForm(false);
          }}
        />
      )}
    </>
  );
}
