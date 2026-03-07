import { useMemo } from 'react';
import type {
  PlannerTask,
  TaskStage,
  TaskView,
  GridSubView,
  TaskFilters,
  TaskActivity,
  TaskComment,
  ProductInfo,
} from '../../lib/types.ts';
import TaskPanel from '../planner/TaskPanel.tsx';
import ScoutPanel from '../panels/ScoutPanel.tsx';
import CompliancePanel from '../panels/CompliancePanel.tsx';
import PlaceholderPanel from '../panels/PlaceholderPanel.tsx';

interface ProductViewProps {
  activeProduct: string | null;
  productMetadata?: Map<string, ProductInfo>;
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
  productMetadata,
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
  // ── Panel registry — every product gets a dedicated panel ────────────────
  // Products with real dashboards get their own component.
  // Everything else gets PlaceholderPanel (blank) until a dashboard is built.
  const DEDICATED_PANELS: Record<string, React.ComponentType> = {
    // Real dashboards
    compliance:      CompliancePanel,
    'compliance-go': CompliancePanel,
    scout:           ScoutPanel,
    // Placeholders — replace each with a real panel when ready
    soul:            PlaceholderPanel,
    qa:              PlaceholderPanel,
    observe:         PlaceholderPanel,
    viz:             PlaceholderPanel,
    bench:           PlaceholderPanel,
    migrate:         PlaceholderPanel,
    devops:          PlaceholderPanel,
    analytics:       PlaceholderPanel,
    docs:            PlaceholderPanel,
    api:             PlaceholderPanel,
    dba:             PlaceholderPanel,
    costops:         PlaceholderPanel,
    mesh:            PlaceholderPanel,
    dataeng:         PlaceholderPanel,
    stocks:          PlaceholderPanel,
  };

  if (activeProduct) {
    const Panel = DEDICATED_PANELS[activeProduct];
    if (Panel) {
      const meta = productMetadata?.get(activeProduct);
      // PlaceholderPanel renders null — skip the chrome wrapper entirely
      if (Panel === PlaceholderPanel) return null;
      return (
        <div className="h-full flex flex-col">
          <div className="glass flex items-center gap-2 px-4 h-11 shrink-0 border-b border-border-subtle">
            <span className="font-display text-sm font-semibold text-fg">{meta?.label || activeProduct}</span>
            {meta?.label && meta.label !== activeProduct && (
              <span className="text-[10px] px-2 py-0.5 rounded bg-soul/10 text-soul font-medium">
                {meta.label}
              </span>
            )}
          </div>
          <div className="flex-1 overflow-hidden">
            <Panel />
          </div>
        </div>
      );
    }
    // Unknown product not in registry — render blank
    return null;
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
