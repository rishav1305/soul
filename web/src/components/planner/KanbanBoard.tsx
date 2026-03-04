import { useMemo } from 'react';
import type { PlannerTask, TaskStage, TaskActivity } from '../../lib/types.ts';
import StageColumn from './StageColumn.tsx';

const STAGES: TaskStage[] = ['backlog', 'brainstorm', 'active', 'blocked', 'validation', 'done'];

interface KanbanBoardProps {
  tasksByStage: Record<TaskStage, PlannerTask[]>;
  onTaskClick: (task: PlannerTask) => void;
  taskActivities?: Record<number, TaskActivity[]>;
}

export default function KanbanBoard({ tasksByStage, onTaskClick, taskActivities }: KanbanBoardProps) {
  const visibleStages = useMemo(() => {
    const populated = STAGES.filter((s) => tasksByStage[s].length > 0);
    return populated.length > 0 ? populated : STAGES;
  }, [tasksByStage]);

  return (
    <div className="flex gap-px h-full overflow-x-auto overflow-y-hidden">
      {visibleStages.map((stage) => (
        <StageColumn
          key={stage}
          stage={stage}
          tasks={tasksByStage[stage]}
          onTaskClick={onTaskClick}
          taskActivities={taskActivities}
        />
      ))}
    </div>
  );
}
