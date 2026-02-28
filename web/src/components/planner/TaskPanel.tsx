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
    <div className="flex flex-col h-full bg-surface">
      {/* Navbar */}
      <div className="glass flex items-center gap-2 px-4 shrink-0 h-11">
        <span className="font-display text-sm font-semibold text-fg">Tasks</span>

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
                  ? 'bg-overlay text-fg'
                  : 'text-fg-muted hover:text-fg-secondary hover:bg-elevated'
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
            className="w-7 h-7 flex items-center justify-center rounded hover:bg-elevated text-fg-muted hover:text-fg transition-colors cursor-pointer"
            title="Reset to auto width"
          >
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
              <path d="M2 8a6 6 0 0 1 10.3-4.2" />
              <path d="M14 2v4h-4" />
              <path d="M14 8a6 6 0 0 1-10.3 4.2" />
              <path d="M2 14v-4h4" />
            </svg>
          </button>
        )}

        {/* Collapse */}
        <button
          type="button"
          onClick={onCollapse}
          disabled={!canCollapse}
          className="w-7 h-7 flex items-center justify-center rounded hover:bg-elevated text-fg-muted hover:text-fg disabled:opacity-20 disabled:cursor-not-allowed transition-colors cursor-pointer"
          title={canCollapse ? 'Collapse tasks' : 'Cannot collapse — chat panel is collapsed'}
        >
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
            <path d="M6 3l5 5-5 5" />
          </svg>
        </button>

        {/* New Task */}
        <button
          type="button"
          onClick={() => setShowNewForm(true)}
          className="bg-soul hover:bg-soul/80 text-deep font-display font-semibold text-xs rounded-md px-3 h-7 whitespace-nowrap shrink-0 flex items-center transition-colors cursor-pointer"
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
