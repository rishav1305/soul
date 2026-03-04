import type { PlannerTask, TaskStage, TaskActivity } from '../../lib/types.ts';
import TaskCard from './TaskCard.tsx';

const STAGE_COLORS: Record<TaskStage, string> = {
  backlog: 'text-stage-backlog',
  brainstorm: 'text-stage-brainstorm',
  active: 'text-stage-active',
  blocked: 'text-stage-blocked',
  validation: 'text-stage-validation',
  done: 'text-stage-done',
};

const STAGE_DOT_COLORS: Record<TaskStage, string> = {
  backlog: 'bg-stage-backlog',
  brainstorm: 'bg-stage-brainstorm',
  active: 'bg-stage-active',
  blocked: 'bg-stage-blocked',
  validation: 'bg-stage-validation',
  done: 'bg-stage-done',
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
  taskActivities?: Record<number, TaskActivity[]>;
  inlineBadgesEnabled?: boolean;
}

export default function StageColumn({ stage, tasks, onTaskClick, taskActivities, inlineBadgesEnabled = true }: StageColumnProps) {
  return (
    <div className="flex flex-col min-w-[220px] w-full bg-surface/30">
      <div className="flex items-center gap-2 px-3 py-2 shrink-0">
        <span className={`w-2 h-2 rounded-full ${STAGE_DOT_COLORS[stage]}`} />
        <h3 className={`font-display text-[11px] font-semibold uppercase tracking-widest ${STAGE_COLORS[stage]}`}>
          {STAGE_LABELS[stage]}
        </h3>
        <span className="ml-auto bg-overlay text-fg-muted text-[10px] font-mono px-1.5 py-0.5 rounded-full">
          {tasks.length}
        </span>
      </div>
      <div className="flex-1 overflow-y-auto px-2 pb-2 space-y-2">
        {tasks.map((task) => {
          const activities = taskActivities?.[task.id] ?? [];
          const lastStageActivity = [...activities].reverse().find((a) => a.type === 'stage');
          return (
            <TaskCard
              key={task.id}
              task={task}
              onClick={onTaskClick}
              recentActivity={lastStageActivity}
              inlineBadgesEnabled={inlineBadgesEnabled}
            />
          );
        })}
        {tasks.length === 0 && (
          <div className="text-xs text-fg-muted text-center py-8">
            No tasks
          </div>
        )}
      </div>
    </div>
  );
}
