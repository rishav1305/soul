import type { PlannerTask, TaskStage } from '../../lib/types.ts';
import StageColumn from './StageColumn.tsx';

const STAGES: TaskStage[] = ['backlog', 'brainstorm', 'active', 'blocked', 'validation', 'done'];

interface KanbanBoardProps {
  tasksByStage: Record<TaskStage, PlannerTask[]>;
  onTaskClick: (task: PlannerTask) => void;
}

export default function KanbanBoard({ tasksByStage, onTaskClick }: KanbanBoardProps) {
  return (
    <div className="flex h-full overflow-x-auto overflow-y-hidden">
      {STAGES.map((stage) => (
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
