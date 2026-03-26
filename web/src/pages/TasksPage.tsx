import { useState, useEffect } from 'react';
import { useTaskSync } from '../hooks/useTaskSync';
import { usePerformance } from '../hooks/usePerformance';
import { reportError, reportUsage } from '../lib/telemetry';
import { TaskCard } from '../components/TaskCard';
import type { TaskStage } from '../lib/types';

const COLUMNS: { stage: TaskStage; label: string; color: string }[] = [
  { stage: 'backlog', label: 'Backlog', color: 'border-zinc-600' },
  { stage: 'active', label: 'Active', color: 'border-blue-500' },
  { stage: 'validation', label: 'Validation', color: 'border-yellow-500' },
  { stage: 'done', label: 'Done', color: 'border-green-500' },
  { stage: 'blocked', label: 'Blocked', color: 'border-red-500' },
];

export function TasksPage() {
  usePerformance('TasksPage');
  useEffect(() => { reportUsage('page.view', { page: 'tasks' }); }, []);
  const { tasks, loading, error, createTask, startTask, stopTask } = useTaskSync({ mode: 'kanban' });
  const [showCreate, setShowCreate] = useState(false);
  const [newTitle, setNewTitle] = useState('');
  const [newDesc, setNewDesc] = useState('');
  const [creating, setCreating] = useState(false);

  const handleCreate = async () => {
    if (!newTitle.trim()) return;
    setCreating(true);
    try {
      await createTask({ title: newTitle.trim(), description: newDesc.trim() });
      setNewTitle('');
      setNewDesc('');
      setShowCreate(false);
    } catch {
      // Error surfaced via useTaskSync.error state
    } finally {
      setCreating(false);
    }
  };

  return (
    <div data-testid="tasks-page" className="h-full flex flex-col">
      {/* Toolbar */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-elevated">
        <h2 className="text-lg font-semibold text-fg">Tasks</h2>
        <button
          data-testid="new-task-btn"
          onClick={() => setShowCreate(!showCreate)}
          className="px-3 py-1.5 text-sm rounded-lg bg-soul/20 text-soul hover:bg-soul/30 transition-colors"
        >
          + New Task
        </button>
      </div>

      {/* Create form */}
      {showCreate && (
        <div data-testid="create-task-form" className="px-4 py-3 border-b border-elevated bg-surface">
          <input
            data-testid="new-task-title"
            value={newTitle}
            onChange={e => setNewTitle(e.target.value)}
            placeholder="Task title"
            aria-label="Task title"
            className="w-full bg-elevated rounded px-3 py-2 text-sm text-fg placeholder:text-fg-muted outline-none focus:ring-1 focus:ring-soul/50"
            onKeyDown={e => e.key === 'Enter' && handleCreate()}
            autoFocus
          />
          <textarea
            data-testid="new-task-desc"
            value={newDesc}
            onChange={e => setNewDesc(e.target.value)}
            placeholder="Description (optional)"
            aria-label="Task description"
            className="w-full bg-elevated rounded px-3 py-2 text-sm text-fg placeholder:text-fg-muted outline-none focus:ring-1 focus:ring-soul/50 mt-2 resize-none"
            rows={2}
          />
          <div className="flex justify-end gap-2 mt-2">
            <button onClick={() => setShowCreate(false)} className="px-3 py-1 text-xs text-fg-muted hover:text-fg transition-colors">
              Cancel
            </button>
            <button
              data-testid="create-task-submit"
              onClick={handleCreate}
              disabled={creating || !newTitle.trim()}
              className="px-3 py-1 text-xs rounded bg-soul text-deep font-medium hover:bg-soul/90 disabled:opacity-50 transition-colors"
            >
              {creating ? 'Creating...' : 'Create'}
            </button>
          </div>
        </div>
      )}

      {error && (
        <div className="px-4 py-2 text-sm text-red-400" role="alert">{error}</div>
      )}

      {/* Kanban columns */}
      <div className="flex-1 overflow-x-auto p-4">
        <div className="flex gap-4 min-w-max h-full">
          {COLUMNS.map(({ stage, label, color }) => {
            const columnTasks = tasks.filter(t => t.stage === stage);
            return (
              <div key={stage} data-testid={`column-${stage}`} className={`w-64 flex flex-col border-t-2 ${color} rounded-lg bg-deep/50`}>
                <div className="flex items-center justify-between px-3 py-2">
                  <span className="text-xs font-medium text-fg-muted uppercase tracking-wider">{label}</span>
                  <span className="text-xs text-fg-muted">{loading ? '-' : columnTasks.length}</span>
                </div>
                <div className="flex-1 overflow-y-auto px-2 pb-2 space-y-2">
                  {columnTasks.map(task => (
                    <TaskCard key={task.id} task={task} onStart={(id) => startTask(id).catch(err => reportError('TasksPage.start', err))} onStop={(id) => stopTask(id).catch(err => reportError('TasksPage.stop', err))} />
                  ))}
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
