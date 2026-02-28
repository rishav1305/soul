import { useState } from 'react';
import type { PlannerTask, TaskStage, TaskView, GridSubView, TaskFilters } from '../../lib/types.ts';
import FilterBar from './FilterBar.tsx';
import TaskContent from './TaskContent.tsx';
import TaskDetail from './TaskDetail.tsx';
import NewTaskForm from './NewTaskForm.tsx';

interface TaskPanelProps {
  // Layout store values + setters
  taskView: TaskView;
  gridSubView: GridSubView;
  panelWidth: number | null;
  filters: TaskFilters;
  setTaskView: (v: TaskView) => void;
  setGridSubView: (v: GridSubView) => void;
  setPanelWidth: (w: number | null) => void;
  setFilters: (partial: Partial<TaskFilters>) => void;
  canCollapse: boolean;
  onCollapse: () => void;
  // Data
  tasks: PlannerTask[];
  filteredTasks: PlannerTask[];
  tasksByStage: Record<TaskStage, PlannerTask[]>;
  products: string[];
  loading: boolean;
  // Actions
  createTask: (title: string, description: string, priority: number, product: string) => Promise<PlannerTask>;
  moveTask: (id: number, stage: TaskStage, comment: string) => Promise<PlannerTask>;
  deleteTask: (id: number) => Promise<void>;
}

const VIEW_BUTTONS: { view: TaskView; icon: string; title: string }[] = [
  { view: 'list', icon: '\u2261', title: 'List view' },      // ≡
  { view: 'kanban', icon: '\u229E', title: 'Kanban view' },   // ⊞
  { view: 'grid', icon: '\u229F', title: 'Grid view' },       // ⊟
];

export default function TaskPanel({
  taskView,
  gridSubView,
  panelWidth,
  filters,
  setTaskView,
  setGridSubView,
  setPanelWidth,
  setFilters,
  canCollapse,
  onCollapse,
  filteredTasks,
  tasksByStage,
  products,
  loading,
  createTask,
  moveTask,
  deleteTask,
}: TaskPanelProps) {
  const [selectedTask, setSelectedTask] = useState<PlannerTask | null>(null);
  const [showNewForm, setShowNewForm] = useState(false);

  const handleCreate = async (title: string, description: string, priority: number, product: string) => {
    await createTask(title, description, priority, product);
    setShowNewForm(false);
  };

  const handleMove = async (id: number, stage: TaskStage, comment: string) => {
    await moveTask(id, stage, comment);
    setSelectedTask(null);
  };

  const handleDelete = async (id: number) => {
    await deleteTask(id);
    setSelectedTask(null);
  };

  const handleClearFilters = () => {
    setFilters({ stage: 'all', priority: 'all', product: 'all' });
  };

  return (
    <div className="flex flex-col h-full bg-zinc-950">
      {/* Navbar */}
      <div className="flex items-center gap-2 px-3 py-2 border-b border-zinc-800 shrink-0 h-11">
        <span className="text-sm font-semibold text-zinc-100">Tasks</span>

        {/* View mode buttons */}
        <div className="flex items-center gap-0.5 ml-2">
          {VIEW_BUTTONS.map(({ view, icon, title }) => (
            <button
              key={view}
              type="button"
              onClick={() => setTaskView(view)}
              title={title}
              className={`w-7 h-7 flex items-center justify-center rounded text-sm cursor-pointer transition-colors ${
                taskView === view
                  ? 'bg-zinc-700 text-zinc-100'
                  : 'text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800'
              }`}
            >
              {icon}
            </button>
          ))}
        </div>

        <div className="flex-1" />

        {/* Reset width */}
        {panelWidth !== null && (
          <button
            type="button"
            onClick={() => setPanelWidth(null)}
            className="text-zinc-500 hover:text-zinc-300 text-sm cursor-pointer"
            title="Reset to auto width"
          >
            &#8635;
          </button>
        )}

        {/* Collapse */}
        <button
          type="button"
          onClick={onCollapse}
          disabled={!canCollapse}
          className="text-zinc-500 hover:text-zinc-300 disabled:opacity-30 disabled:cursor-not-allowed text-lg leading-none cursor-pointer"
          title={canCollapse ? 'Collapse tasks' : 'Cannot collapse — chat panel is collapsed'}
        >
          &times;
        </button>

        {/* New Task */}
        <button
          type="button"
          onClick={() => setShowNewForm(true)}
          className="px-2.5 py-1 text-xs font-medium rounded bg-sky-600 hover:bg-sky-500 text-white transition-colors cursor-pointer"
        >
          + New Task
        </button>
      </div>

      {/* Filter Bar */}
      <FilterBar filters={filters} products={products} onChange={setFilters} />

      {/* Body */}
      <div className="flex-1 overflow-hidden">
        {loading ? (
          <div className="flex-1 flex items-center justify-center h-full">
            <span className="text-zinc-500 text-sm">Loading tasks...</span>
          </div>
        ) : (
          <TaskContent
            taskView={taskView}
            filteredTasks={filteredTasks}
            tasksByStage={tasksByStage}
            gridSubView={gridSubView}
            onGridSubViewChange={setGridSubView}
            onTaskClick={setSelectedTask}
            onClearFilters={handleClearFilters}
          />
        )}
      </div>

      {/* Modals */}
      {selectedTask && (
        <TaskDetail
          task={selectedTask}
          onClose={() => setSelectedTask(null)}
          onMove={handleMove}
          onDelete={handleDelete}
        />
      )}

      {showNewForm && (
        <NewTaskForm
          onClose={() => setShowNewForm(false)}
          onCreate={handleCreate}
        />
      )}
    </div>
  );
}
