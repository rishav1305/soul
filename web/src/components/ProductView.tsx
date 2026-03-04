import { useMemo } from 'react';
import type { PlannerTask, TaskStage, TaskView, GridSubView, TaskFilters, TaskActivity, TaskComment } from '../lib/types.ts';
import TaskPanel from './planner/TaskPanel.tsx';
import CompliancePanel from './panels/CompliancePanel.tsx';
import ScoutPanel from './panels/ScoutPanel.tsx';

interface ProductViewProps {
  activeProduct: string | null;
  tasks: PlannerTask[];
  filteredTasks: PlannerTask[];
  products: string[];
  loading: boolean;
  // Layout store pass-through
  taskView: TaskView;
  gridSubView: GridSubView;
  panelWidth: number | null;
  filters: TaskFilters;
  setTaskView: (v: TaskView) => void;
  setGridSubView: (v: GridSubView) => void;
  setPanelWidth: (w: number | null) => void;
  setFilters: (partial: Partial<TaskFilters>) => void;
  // Actions
  createTask: (title: string, description: string, priority: number, product: string) => Promise<PlannerTask>;
  updateTask: (id: number, updates: Partial<PlannerTask>) => Promise<PlannerTask>;
  moveTask: (id: number, stage: TaskStage, comment: string) => Promise<PlannerTask>;
  deleteTask: (id: number) => Promise<void>;
  taskActivities: Record<number, TaskActivity[]>;
  taskStreams: Record<number, string>;
  taskComments: Record<number, TaskComment[]>;
  fetchComments: (id: number) => Promise<any>;
  addComment: (id: number, body: string) => Promise<TaskComment>;
}

function ProductHeader({ product, tasks }: { product: string; tasks: PlannerTask[] }) {
  const productTasks = useMemo(() => tasks.filter((t) => t.product === product), [tasks, product]);
  const active = productTasks.filter((t) => t.stage === 'active').length;
  const blocked = productTasks.filter((t) => t.stage === 'blocked').length;
  const total = productTasks.length;

  return (
    <div className="glass flex items-center gap-3 px-4 h-11 shrink-0 border-b border-border-subtle">
      <span className="font-display text-sm font-semibold text-fg capitalize">{product}</span>
      <div className="flex items-center gap-2 text-xs">
        {active > 0 && (
          <span className="px-1.5 py-0.5 rounded bg-stage-active/15 text-stage-active">
            {active} active
          </span>
        )}
        {blocked > 0 && (
          <span className="px-1.5 py-0.5 rounded bg-stage-blocked/15 text-stage-blocked">
            {blocked} blocked
          </span>
        )}
        <span className="text-fg-muted">{total} total</span>
      </div>
    </div>
  );
}

export default function ProductView({
  activeProduct,
  tasks,
  filteredTasks,
  products,
  loading,
  taskView,
  gridSubView,
  panelWidth,
  filters,
  setTaskView,
  setGridSubView,
  setPanelWidth,
  setFilters,
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
  // Route based on activeProduct
  if (activeProduct === 'compliance' || activeProduct === 'compliance-go') {
    return (
      <div className="flex flex-col h-full">
        <ProductHeader product={activeProduct} tasks={tasks} />
        <div className="flex-1 overflow-hidden">
          <CompliancePanel />
        </div>
      </div>
    );
  }
  if (activeProduct === 'scout') {
    return (
      <div className="flex flex-col h-full">
        <ProductHeader product="scout" tasks={tasks} />
        <div className="flex-1 overflow-hidden">
          <ScoutPanel />
        </div>
      </div>
    );
  }

  // Null or unknown product → TaskPanel (filtered to product if set)
  const viewTasks = activeProduct
    ? filteredTasks.filter((t) => t.product === activeProduct)
    : filteredTasks;

  const viewByStage: Record<TaskStage, PlannerTask[]> = {
    backlog: [],
    brainstorm: [],
    active: [],
    blocked: [],
    validation: [],
    done: [],
  };
  for (const t of viewTasks) {
    viewByStage[t.stage].push(t);
  }

  return (
    <div className="flex flex-col h-full">
      {/* Product header for custom products */}
      {activeProduct && (
        <ProductHeader product={activeProduct} tasks={tasks} />
      )}
      <div className="flex-1 overflow-hidden">
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
          filteredTasks={viewTasks}
          tasksByStage={viewByStage}
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
      </div>
    </div>
  );
}
