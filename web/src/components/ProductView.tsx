import type { PlannerTask, TaskStage, TaskView, GridSubView, TaskFilters, TaskActivity, TaskComment } from '../lib/types.ts';
import TaskPanel from './planner/TaskPanel.tsx';
import CompliancePanel from './panels/CompliancePanel.tsx';
import ScoutPanel from './panels/ScoutPanel.tsx';

interface ProductViewProps {
  activeProduct: string | null;
  tasks: PlannerTask[];
  filteredTasks: PlannerTask[];
  tasksByStage: Record<TaskStage, PlannerTask[]>;
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

export default function ProductView({
  activeProduct,
  tasks,
  filteredTasks,
  tasksByStage,
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
  if (activeProduct === 'compliance') {
    return <CompliancePanel />;
  }
  if (activeProduct === 'compliance-go') {
    return <CompliancePanel />;
  }
  if (activeProduct === 'scout') {
    return <ScoutPanel />;
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
  );
}
