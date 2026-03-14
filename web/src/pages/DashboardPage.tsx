import { Link } from 'react-router';
import { useTasks } from '../hooks/useTasks';
import type { TaskStage } from '../lib/types';

const STAGE_COLORS: Record<TaskStage, string> = {
  backlog: 'bg-zinc-600',
  active: 'bg-blue-500',
  validation: 'bg-yellow-500',
  done: 'bg-green-500',
  blocked: 'bg-red-500',
};

const STAGE_LABELS: Record<TaskStage, string> = {
  backlog: 'Backlog',
  active: 'Active',
  validation: 'Validation',
  done: 'Done',
  blocked: 'Blocked',
};

export function DashboardPage() {
  const { tasks, loading, error } = useTasks();

  const counts = (Object.keys(STAGE_LABELS) as TaskStage[]).map(stage => ({
    stage,
    label: STAGE_LABELS[stage],
    color: STAGE_COLORS[stage],
    count: tasks.filter(t => t.stage === stage).length,
  }));

  return (
    <div data-testid="dashboard-page" className="h-full overflow-y-auto p-6">
      <div className="max-w-4xl mx-auto space-y-8">
        <div>
          <h2 className="text-xl font-semibold text-fg mb-1">Dashboard</h2>
          <p className="text-sm text-fg-muted">System overview</p>
        </div>

        {error && (
          <div data-testid="dashboard-error" className="p-3 rounded-lg bg-red-900/30 text-red-400 text-sm">
            {error}
          </div>
        )}

        {/* Task counts */}
        <div className="grid grid-cols-2 sm:grid-cols-5 gap-3">
          {counts.map(({ stage, label, color, count }) => (
            <Link
              key={stage}
              to={`/tasks?stage=${stage}`}
              data-testid={`count-${stage}`}
              className="flex flex-col items-center p-4 rounded-lg bg-surface hover:bg-elevated transition-colors"
            >
              <span className={`inline-block w-2 h-2 rounded-full ${color} mb-2`} />
              <span className="text-2xl font-bold text-fg">{loading ? '-' : count}</span>
              <span className="text-xs text-fg-muted mt-1">{label}</span>
            </Link>
          ))}
        </div>

        {/* Recent tasks */}
        <div>
          <h3 className="text-sm font-medium text-fg-muted mb-3">Recent Tasks</h3>
          {loading ? (
            <div className="text-fg-muted text-sm">Loading...</div>
          ) : tasks.length === 0 ? (
            <div className="text-fg-muted text-sm">No tasks yet. <Link to="/tasks" className="text-soul hover:underline">Create one</Link>.</div>
          ) : (
            <div className="space-y-2">
              {tasks.slice(0, 10).map(task => (
                <Link
                  key={task.id}
                  to={`/tasks/${task.id}`}
                  data-testid={`recent-task-${task.id}`}
                  className="flex items-center justify-between p-3 rounded-lg bg-surface hover:bg-elevated transition-colors"
                >
                  <div className="flex items-center gap-3 min-w-0">
                    <span className={`shrink-0 w-2 h-2 rounded-full ${STAGE_COLORS[task.stage as TaskStage] || 'bg-zinc-600'}`} />
                    <span className="text-sm text-fg truncate">{task.title}</span>
                  </div>
                  <span className="text-xs text-fg-muted shrink-0 ml-3">{task.stage}</span>
                </Link>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
