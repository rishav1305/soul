import { useMemo } from 'react';
import type { PlannerTask, TaskStage } from '../../lib/types.ts';
import StageColumn from './StageColumn.tsx';

const STAGES: TaskStage[] = ['backlog', 'brainstorm', 'active', 'blocked', 'validation', 'done'];

interface KanbanBoardProps {
  tasksByStage: Record<TaskStage, PlannerTask[]>;
  onTaskClick: (task: PlannerTask) => void;
}

export default function KanbanBoard({ tasksByStage, onTaskClick }: KanbanBoardProps) {
  // Only render stages that have tasks; if all empty, fall back to all stages
  const visibleStages = useMemo(() => {
    const populated = STAGES.filter((s) => tasksByStage[s].length > 0);
    return populated.length > 0 ? populated : STAGES;
  }, [tasksByStage]);

  return (
    <div className="flex h-full overflow-x-auto overflow-y-hidden">
      {visibleStages.map((stage) => (
        <StageColumn
          key={stage}
          stage={stage}
          tasks={tasksByStage[stage]}
          onTaskClick={onTaskClick}
        />
      ))}
    </div>
  );
}
