import type {
  PlannerTask,
  TaskStage,
  TaskView,
  GridSubView,
  TaskFilters,
  TaskActivity,
  TaskComment,
} from '../../lib/types.ts';
import TaskPanel from '../planner/TaskPanel.tsx';
import ScoutPanel from '../panels/ScoutPanel.tsx';
import CompliancePanel from '../panels/CompliancePanel.tsx';

interface ProductViewProps {
  activeProduct: string | null;
  // TaskPanel props
  taskView: TaskView;
  gridSubView: GridSubView;
  panelWidth: number | null;
  filters: TaskFilters;
  setTaskView: (v: TaskView) => void;
  setGridSubView: (v: GridSubView) => void;
  setPanelWidth: (w: number | null) => void;
  setFilters: (partial: Partial<TaskFilters>) => void;
  tasks: PlannerTask[];
  filteredTasks: PlannerTask[];
  tasksByStage: Record<TaskStage, PlannerTask[]>;
  products: string[];
  loading: boolean;
  createTask: (title: string, description: string, priority: number, product: string) => Promise<PlannerTask>;
  updateTask: (id: number, updates: Partial<PlannerTask>) => Promise<PlannerTask>;
  moveTask: (id: number, stage: TaskStage, comment: string) => Promise<PlannerTask>;
  deleteTask: (id: number) => Promise<void>;
  taskActivities: Record<number, TaskActivity[]>;
  taskStreams: Record<number, string>;
  taskComments: Record<number, TaskComment[]>;
  fetchComments: (id: number) => Promise<TaskComment[]>;
  addComment: (id: number, body: string) => Promise<TaskComment>;
}

export default function ProductView({
  activeProduct,
  taskView,
  gridSubView,
  panelWidth,
  filters,
  setTaskView,
  setGridSubView,
  setPanelWidth,
  setFilters,
  tasks,
  filteredTasks,
  tasksByStage,
  products,
  loading,
  createTask,
  updateTask,
  moveTask,
  deleteTask,
  taskActivities,
  taskStreams,
  taskComments,
  fetchComments,
  addComment,
}: ProductViewProps) {
  // Route to dedicated product panels
  if (activeProduct === 'compliance' || activeProduct === 'compliance-go') {
    return (
      <div className="h-full flex flex-col">
        {/* Product header */}
        <div className="glass flex items-center gap-2 px-4 h-11 shrink-0 border-b border-border-subtle">
          <span className="font-display text-sm font-semibold text-fg">{activeProduct}</span>
          <span className="text-[10px] px-2 py-0.5 rounded bg-stage-validation/15 text-stage-validation font-medium">
            Compliance
          </span>
        </div>
        <div className="flex-1 overflow-hidden">
          <CompliancePanel />
        </div>
      </div>
    );
  }

  if (activeProduct === 'scout') {
    return (
      <div className="h-full flex flex-col">
        <div className="glass flex items-center gap-2 px-4 h-11 shrink-0 border-b border-border-subtle">
          <span className="font-display text-sm font-semibold text-fg">Scout</span>
          <span className="text-[10px] px-2 py-0.5 rounded bg-stage-brainstorm/15 text-stage-brainstorm font-medium">
            Career Intelligence
          </span>
        </div>
        <div className="flex-1 overflow-hidden">
          <ScoutPanel />
        </div>
      </div>
    );
  }

  // Default: TaskPanel, optionally pre-filtered to the active product
  const effectiveFilters = activeProduct
    ? { ...filters, product: activeProduct }
    : filters;

  const effectiveFiltered = activeProduct
    ? tasks.filter((t) => {
        if (t.product !== activeProduct) return false;
        if (filters.stage !== 'all' && t.stage !== filters.stage) return false;
        if (filters.priority !== 'all' && t.priority !== filters.priority) return false;
        return true;
      })
    : filteredTasks;

  const effectiveByStage = (() => {
    const grouped: Record<TaskStage, PlannerTask[]> = {
      backlog: [], brainstorm: [], active: [], blocked: [], validation: [], done: [],
    };
    for (const t of effectiveFiltered) grouped[t.stage].push(t);
    return grouped;
  })();

  return (
    <TaskPanel
      taskView={taskView}
      gridSubView={gridSubView}
      panelWidth={panelWidth}
      filters={effectiveFilters}
      setTaskView={setTaskView}
      setGridSubView={setGridSubView}
      setPanelWidth={setPanelWidth}
      setFilters={setFilters}
      canCollapse={false}
      onCollapse={() => {}}
      tasks={tasks}
      filteredTasks={effectiveFiltered}
      tasksByStage={effectiveByStage}
      products={products}
      loading={loading}
      createTask={createTask}
      updateTask={updateTask}
      moveTask={moveTask}
      deleteTask={deleteTask}
      taskActivities={taskActivities}
      taskStreams={taskStreams}
      taskComments={taskComments}
      fetchComments={fetchComments}
      addComment={addComment}
    />
  );
}
