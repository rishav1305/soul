import { useMemo } from 'react';
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

function emptyByStage(): Record<TaskStage, PlannerTask[]> {
  return { backlog: [], brainstorm: [], active: [], blocked: [], validation: [], done: [] };
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
  // ── Dedicated product dashboards ──────────────────────────────────────────

  if (activeProduct === 'compliance' || activeProduct === 'compliance-go') {
    return (
      <div className="h-full flex flex-col">
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

  // ── Product-scoped task dashboard ─────────────────────────────────────────
  // Any other named product gets its own TaskPanel dashboard scoped to that product.

  if (activeProduct) {
    return (
      <ProductTaskDashboard
        product={activeProduct}
        tasks={tasks}
        taskView={taskView}
        gridSubView={gridSubView}
        panelWidth={panelWidth}
        filters={filters}
        setTaskView={setTaskView}
        setGridSubView={setGridSubView}
        setPanelWidth={setPanelWidth}
        setFilters={setFilters}
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

  // ── Global "All Tasks" dashboard (activeProduct === null) ─────────────────

  return (
    <TaskPanel
      taskView={taskView}
      gridSubView={gridSubView}
      panelWidth={panelWidth}
      filters={filters}
      setTaskView={setTaskView}
      setGridSubView={setGridSubView}
      setPanelWidth={setPanelWidth}
      setFilters={setFilters}
      canCollapse={false}
      onCollapse={() => {}}
      tasks={tasks}
      filteredTasks={filteredTasks}
      tasksByStage={tasksByStage}
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

// ─── Product-scoped Task Dashboard ───────────────────────────────────────────
// Owns its own filtered + grouped task state scoped to one product.

interface ProductTaskDashboardProps {
  product: string;
  tasks: PlannerTask[];
  taskView: TaskView;
  gridSubView: GridSubView;
  panelWidth: number | null;
  filters: TaskFilters;
  setTaskView: (v: TaskView) => void;
  setGridSubView: (v: GridSubView) => void;
  setPanelWidth: (w: number | null) => void;
  setFilters: (partial: Partial<TaskFilters>) => void;
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

function ProductTaskDashboard({
  product,
  tasks,
  taskView,
  gridSubView,
  panelWidth,
  filters,
  setTaskView,
  setGridSubView,
  setPanelWidth,
  setFilters,
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
}: ProductTaskDashboardProps) {
  // Scope all tasks to this product first, then apply stage/priority filters
  const scopedFiltered = useMemo(() => {
    return tasks.filter((t) => {
      if (t.product !== product) return false;
      if (filters.stage !== 'all' && t.stage !== filters.stage) return false;
      if (filters.priority !== 'all' && t.priority !== filters.priority) return false;
      return true;
    });
  }, [tasks, product, filters.stage, filters.priority]);

  const scopedByStage = useMemo(() => {
    const grouped = emptyByStage();
    for (const t of scopedFiltered) grouped[t.stage].push(t);
    return grouped;
  }, [scopedFiltered]);

  // Strip product filter from filters object since it's implicit here
  const scopedFilters: TaskFilters = useMemo(
    () => ({ ...filters, product: 'all' }),
    [filters],
  );

  return (
    <TaskPanel
      taskView={taskView}
      gridSubView={gridSubView}
      panelWidth={panelWidth}
      filters={scopedFilters}
      setTaskView={setTaskView}
      setGridSubView={setGridSubView}
      setPanelWidth={setPanelWidth}
      setFilters={setFilters}
      canCollapse={false}
      onCollapse={() => {}}
      tasks={tasks}
      filteredTasks={scopedFiltered}
      tasksByStage={scopedByStage}
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
      productScope={product}
    />
  );
}
