import type { PlannerTask, TaskStage, TaskView, GridSubView, PlannerActivity } from '../../lib/types.ts';
import ListView from './ListView.tsx';
import KanbanBoard from './KanbanBoard.tsx';
import CompactGrid from './grid/CompactGrid.tsx';
import TableView from './grid/TableView.tsx';

interface TaskContentProps {
  taskView: TaskView;
  filteredTasks: PlannerTask[];
  tasksByStage: Record<TaskStage, PlannerTask[]>;
  gridSubView: GridSubView;
  onGridSubViewChange: (v: GridSubView) => void;
  onTaskClick: (task: PlannerTask) => void;
  onClearFilters: () => void;
  taskActivities?: Record<number, PlannerActivity[]>;
  products?: string[];
}

export default function TaskContent({
  taskView,
  filteredTasks,
  tasksByStage,
  onTaskClick,
  onClearFilters,
  taskActivities,
  products,
}: TaskContentProps) {
  if (filteredTasks.length === 0) {
    return (
      <div className="flex-1 flex flex-col items-center justify-center h-full gap-2">
        <span className="text-fg-muted text-sm font-body">No tasks match filters</span>
        <button
          type="button"
          onClick={onClearFilters}
          data-testid="clear-filters"
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
      return <KanbanBoard tasksByStage={tasksByStage} onTaskClick={onTaskClick} taskActivities={taskActivities} products={products} />;
    case 'grid':
      return <CompactGrid tasks={filteredTasks} onTaskClick={onTaskClick} />;
    case 'table':
      return <TableView tasks={filteredTasks} onTaskClick={onTaskClick} />;
    default:
      return null;
  }
}
