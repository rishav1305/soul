import type { PlannerTask, TaskStage, TaskView, GridSubView } from '../../lib/types.ts';
import ListView from './ListView.tsx';
import KanbanBoard from './KanbanBoard.tsx';
import GridView from './grid/GridView.tsx';

interface TaskContentProps {
  taskView: TaskView;
  filteredTasks: PlannerTask[];
  tasksByStage: Record<TaskStage, PlannerTask[]>;
  gridSubView: GridSubView;
  onGridSubViewChange: (v: GridSubView) => void;
  onTaskClick: (task: PlannerTask) => void;
  onClearFilters: () => void;
}

export default function TaskContent({
  taskView,
  filteredTasks,
  tasksByStage,
  gridSubView,
  onGridSubViewChange,
  onTaskClick,
  onClearFilters,
}: TaskContentProps) {
  // Empty state
  if (filteredTasks.length === 0) {
    return (
      <div className="flex-1 flex flex-col items-center justify-center h-full gap-2">
        <span className="text-fg-muted text-sm font-body">No tasks match filters</span>
        <button
          type="button"
          onClick={onClearFilters}
          className="text-soul hover:text-soul/80 text-xs cursor-pointer underline"
        >
          Clear filters
        </button>
      </div>
    );
  }

  switch (taskView) {
    case 'list':
      return <ListView tasks={filteredTasks} onTaskClick={onTaskClick} />;
    case 'kanban':
      return <KanbanBoard tasksByStage={tasksByStage} onTaskClick={onTaskClick} />;
    case 'grid':
      return (
        <GridView
          tasks={filteredTasks}
          subView={gridSubView}
          onSubViewChange={onGridSubViewChange}
          onTaskClick={onTaskClick}
        />
      );
    default:
      return null;
  }
}
