import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router';
import { api } from '../lib/api';
import type { Task, TaskActivity, TaskStage } from '../lib/types';
import { ActivityTimeline } from '../components/ActivityTimeline';

const STAGE_COLORS: Record<TaskStage, string> = {
  backlog: 'bg-zinc-600',
  active: 'bg-blue-500',
  validation: 'bg-yellow-500',
  done: 'bg-green-500',
  blocked: 'bg-red-500',
};

export function TaskDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [task, setTask] = useState<Task | null>(null);
  const [activities, setActivities] = useState<TaskActivity[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    setLoading(true);
    Promise.all([
      api.get<Task>(`/api/tasks/${id}`),
      api.get<TaskActivity[]>(`/api/tasks/${id}/activity`),
    ])
      .then(([t, acts]) => {
        setTask(t);
        setActivities(acts);
        setError(null);
      })
      .catch(err => setError(err.message))
      .finally(() => setLoading(false));
  }, [id]);

  const handleStart = async () => {
    if (!id) return;
    try {
      await api.post(`/api/tasks/${id}/start`);
      const t = await api.get<Task>(`/api/tasks/${id}`);
      setTask(t);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Start failed');
    }
  };

  const handleStop = async () => {
    if (!id) return;
    try {
      await api.post(`/api/tasks/${id}/stop`);
      const t = await api.get<Task>(`/api/tasks/${id}`);
      setTask(t);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Stop failed');
    }
  };

  if (loading) {
    return <div className="p-6 text-fg-muted">Loading...</div>;
  }

  if (error || !task) {
    return (
      <div className="p-6">
        <Link to="/tasks" className="text-sm text-soul hover:underline">&larr; Back to tasks</Link>
        <div className="mt-4 text-red-400">{error || 'Task not found'}</div>
      </div>
    );
  }

  return (
    <div data-testid="task-detail-page" className="h-full overflow-y-auto p-6">
      <div className="max-w-3xl mx-auto">
        <Link to="/tasks" className="text-sm text-soul hover:underline">&larr; Back to tasks</Link>

        <div className="mt-4 space-y-6">
          {/* Header */}
          <div className="flex items-start justify-between gap-4">
            <div>
              <h2 className="text-xl font-semibold text-fg">{task.title}</h2>
              {task.description && (
                <p className="text-sm text-fg-muted mt-1">{task.description}</p>
              )}
            </div>
            <div className="flex items-center gap-2">
              <span className={`px-2 py-1 text-xs rounded-full text-white ${STAGE_COLORS[task.stage as TaskStage] || 'bg-zinc-600'}`}>
                {task.stage}
              </span>
              {task.workflow && (
                <span className="px-2 py-1 text-xs rounded-full bg-elevated text-fg-muted">{task.workflow}</span>
              )}
            </div>
          </div>

          {/* Actions */}
          <div className="flex gap-2">
            {(task.stage === 'backlog' || task.stage === 'blocked') && (
              <button
                data-testid="detail-start"
                onClick={handleStart}
                className="px-4 py-2 text-sm rounded-lg bg-green-700 text-white hover:bg-green-600 transition-colors"
              >
                Start Execution
              </button>
            )}
            {task.stage === 'active' && (
              <button
                data-testid="detail-stop"
                onClick={handleStop}
                className="px-4 py-2 text-sm rounded-lg bg-red-700 text-white hover:bg-red-600 transition-colors"
              >
                Stop Execution
              </button>
            )}
          </div>

          {/* Activity */}
          <div>
            <h3 className="text-sm font-medium text-fg-muted mb-3">Activity Log</h3>
            <ActivityTimeline activities={activities} />
          </div>
        </div>
      </div>
    </div>
  );
}
