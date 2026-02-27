import { useState } from 'react';
import { usePlanner } from '../../hooks/usePlanner.ts';
import type { PlannerTask, TaskStage } from '../../lib/types.ts';
import StageColumn from './StageColumn.tsx';
import TaskDetail from './TaskDetail.tsx';
import NewTaskForm from './NewTaskForm.tsx';

const STAGES: TaskStage[] = ['backlog', 'brainstorm', 'active', 'blocked', 'validation', 'done'];

export default function KanbanBoard() {
  const { tasksByStage, loading, createTask, moveTask, deleteTask } = usePlanner();
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

  return (
    <div className="flex flex-col h-full bg-zinc-950">
      <div className="flex items-center justify-between px-4 py-3 border-b border-zinc-800 shrink-0">
        <h2 className="text-lg font-bold text-zinc-100">Task Manager</h2>
        <button
          type="button"
          onClick={() => setShowNewForm(true)}
          className="px-3 py-1.5 text-sm font-medium rounded-md bg-sky-600 hover:bg-sky-500 text-white transition-colors cursor-pointer"
        >
          + New Task
        </button>
      </div>

      {loading ? (
        <div className="flex-1 flex items-center justify-center">
          <span className="text-zinc-500 text-sm">Loading tasks...</span>
        </div>
      ) : (
        <div className="flex-1 flex overflow-x-auto overflow-y-hidden">
          {STAGES.map((stage) => (
            <StageColumn
              key={stage}
              stage={stage}
              tasks={tasksByStage[stage]}
              onTaskClick={setSelectedTask}
            />
          ))}
        </div>
      )}

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
