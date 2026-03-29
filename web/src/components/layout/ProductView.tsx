import { Component, type ComponentType, type ReactNode, type ErrorInfo } from 'react';
import type {
  PlannerTask,
  TaskStage,
  TaskView,
  GridSubView,
  TaskFilters,
  PlannerActivity,
  TaskComment,
  ProductInfo,
} from '../../lib/types.ts';
import { reportError } from '../../lib/telemetry.ts';
import TaskPanel from '../planner/TaskPanel.tsx';
import PlaceholderPanel from '../panels/PlaceholderPanel.tsx';

/** Lightweight error boundary for product panels — avoids taking down the entire AppShell. */
class PanelErrorBoundary extends Component<
  { name: string; children: ReactNode },
  { hasError: boolean; error: Error | null }
> {
  constructor(props: { name: string; children: ReactNode }) {
    super(props);
    this.state = { hasError: false, error: null };
  }
  static getDerivedStateFromError(error: Error) {
    return { hasError: true, error };
  }
  componentDidCatch(error: Error, info: ErrorInfo) {
    reportError(`PanelError:${this.props.name}`, error);
  }
  render() {
    if (this.state.hasError) {
      return (
        <div data-testid={`panel-error-${this.props.name}`} className="flex items-center justify-center h-full">
          <div className="text-center space-y-3 p-8">
            <div className="text-2xl">⚠️</div>
            <h2 className="text-sm font-semibold text-fg">
              {this.props.name} panel crashed
            </h2>
            <p className="text-xs text-fg-muted max-w-xs">
              {this.state.error?.message || 'An unexpected error occurred'}
            </p>
            <button
              data-testid={`panel-retry-${this.props.name}`}
              onClick={() => this.setState({ hasError: false, error: null })}
              className="px-3 py-1.5 text-xs bg-soul text-deep rounded-lg hover:bg-soul/85 transition-colors cursor-pointer"
            >
              Retry
            </button>
          </div>
        </div>
      );
    }
    return this.props.children;
  }
}

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
  taskActivities: Record<number, PlannerActivity[]>;
  taskStreams: Record<number, string>;
  taskComments: Record<number, TaskComment[]>;
  fetchComments: (id: number) => Promise<TaskComment[]>;
  addComment: (id: number, body: string) => Promise<TaskComment>;
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
  const DEDICATED_PANELS: Record<string, ComponentType> = {
    // Real dashboards
    compliance:      PlaceholderPanel,  // stub — falls through to TaskPanel until dashboard built
    "compliance-go": PlaceholderPanel,  // stub
    scout:           PlaceholderPanel,  // stub — falls through to TaskPanel until dashboard built
    tutor:           PlaceholderPanel,  // stub — falls through to TaskPanel until dashboard built
    projects:        PlaceholderPanel,  // stub — falls through to TaskPanel until dashboard built
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

  // ── Dedicated product panel (if registered and not a placeholder) ──
  if (activeProduct) {
    const Panel = DEDICATED_PANELS[activeProduct];
    if (Panel && Panel !== PlaceholderPanel) {
      const meta = productMetadata?.get(activeProduct);
      return (
        <div data-testid={`product-view-${activeProduct}`} className="h-full flex flex-col">
          <div className="glass flex items-center gap-2 px-4 h-11 shrink-0 border-b border-border-subtle">
            <span className="font-display text-sm font-semibold text-fg">{meta?.label || activeProduct}</span>
            {meta?.label && meta.label !== activeProduct && (
              <span className="text-[10px] px-2 py-0.5 rounded bg-soul/10 text-soul font-medium">
                {meta.label}
              </span>
            )}
          </div>
          <div className="flex-1 overflow-hidden">
            <PanelErrorBoundary name={activeProduct}>
              <Panel />
            </PanelErrorBoundary>
          </div>
        </div>
      );
    }
    // Placeholder or unknown products fall through to the Tasks dashboard below
  }

  // ── Tasks dashboard (default for "soul", placeholder products, or null) ──

  return (
    <PanelErrorBoundary name={activeProduct || 'tasks'}>
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
    </PanelErrorBoundary>
  );
}
