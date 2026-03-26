import { useMemo, useState, useCallback } from 'react';
import { authFetch } from '../../lib/api.ts';
import type { PlannerTask, TaskStage, PlannerActivity } from '../../lib/types.ts';
import StageColumn from './StageColumn.tsx';

const STAGES: TaskStage[] = ['backlog', 'brainstorm', 'active', 'blocked', 'validation', 'done'];

const STAGE_LABELS: Record<TaskStage, string> = {
  backlog: 'Backlog',
  brainstorm: 'Brainstorm',
  active: 'Active',
  blocked: 'Blocked',
  validation: 'Validation',
  done: 'Done',
};

const PRIORITY_OPTIONS = [
  { value: 0, label: 'Low' },
  { value: 1, label: 'Normal' },
  { value: 2, label: 'High' },
  { value: 3, label: 'Critical' },
];

interface KanbanBoardProps {
  tasksByStage: Record<TaskStage, PlannerTask[]>;
  onTaskClick: (task: PlannerTask) => void;
  taskActivities?: Record<number, PlannerActivity[]>;
  products?: string[];
}

export default function KanbanBoard({ tasksByStage, onTaskClick, taskActivities, products }: KanbanBoardProps) {
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
  const [batchMode, setBatchMode] = useState(false);
  const [batchLoading, setBatchLoading] = useState(false);

  const visibleStages = useMemo(() => {
    const populated = STAGES.filter((s) => tasksByStage[s].length > 0);
    return populated.length > 0 ? populated : STAGES;
  }, [tasksByStage]);

  const allTasks = useMemo(() => STAGES.flatMap((s) => tasksByStage[s]), [tasksByStage]);

  const toggleSelect = useCallback((id: number) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

  const selectAll = useCallback(() => {
    setSelectedIds(new Set(allTasks.map((t) => t.id)));
  }, [allTasks]);

  const exitBatch = useCallback(() => {
    setBatchMode(false);
    setSelectedIds(new Set());
  }, []);

  const handleBatchMove = useCallback(async (stage: TaskStage) => {
    if (selectedIds.size === 0) return;
    setBatchLoading(true);
    try {
      await Promise.all(
        Array.from(selectedIds).map((id) =>
          authFetch(`/api/tasks/${id}/move`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ stage, comment: 'Batch move' }),
          })
        )
      );
      exitBatch();
    } catch (err) {
      console.error('Batch move failed:', err);
    } finally {
      setBatchLoading(false);
    }
  }, [selectedIds, exitBatch]);

  const handleBatchPriority = useCallback(async (priority: number) => {
    if (selectedIds.size === 0) return;
    setBatchLoading(true);
    try {
      await Promise.all(
        Array.from(selectedIds).map((id) =>
          authFetch(`/api/tasks/${id}`, {
            method: 'PATCH',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ priority }),
          })
        )
      );
      exitBatch();
    } catch (err) {
      console.error('Batch priority failed:', err);
    } finally {
      setBatchLoading(false);
    }
  }, [selectedIds, exitBatch]);

  const handleBatchProduct = useCallback(async (product: string) => {
    if (selectedIds.size === 0) return;
    setBatchLoading(true);
    try {
      await Promise.all(
        Array.from(selectedIds).map((id) =>
          authFetch(`/api/tasks/${id}`, {
            method: 'PATCH',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ product }),
          })
        )
      );
      exitBatch();
    } catch (err) {
      console.error('Batch assign product failed:', err);
    } finally {
      setBatchLoading(false);
    }
  }, [selectedIds, exitBatch]);

  const handleBatchDelete = useCallback(async () => {
    if (selectedIds.size === 0) return;
    setBatchLoading(true);
    try {
      await Promise.all(
        Array.from(selectedIds).map((id) =>
          authFetch(`/api/tasks/${id}`, { method: 'DELETE' })
        )
      );
      exitBatch();
    } catch (err) {
      console.error('Batch delete failed:', err);
    } finally {
      setBatchLoading(false);
    }
  }, [selectedIds, exitBatch]);

  return (
    <div className="flex flex-col h-full overflow-hidden">
      {/* Batch action bar */}
      {batchMode ? (
        <div className="flex items-center gap-2 px-3 py-1.5 bg-soul/10 border-b border-soul/20 shrink-0">
          <span className="text-xs font-display font-semibold text-soul">
            {selectedIds.size} selected
          </span>
          <button
            type="button"
            onClick={selectAll}
            data-testid="kanban-select-all"
            className="text-[10px] text-soul hover:text-soul/80 cursor-pointer underline"
          >
            Select all ({allTasks.length})
          </button>
          <div className="w-px h-4 bg-border-subtle" />

          {/* Move to stage */}
          <select
            disabled={selectedIds.size === 0 || batchLoading}
            onChange={(e) => { if (e.target.value) handleBatchMove(e.target.value as TaskStage); e.target.value = ''; }}
            className="soul-select text-[10px] h-6 px-1 rounded bg-elevated border border-border-default cursor-pointer disabled:opacity-50"
            defaultValue=""
          >
            <option value="" disabled>Move to...</option>
            {STAGES.map((s) => (
              <option key={s} value={s}>{STAGE_LABELS[s]}</option>
            ))}
          </select>

          {/* Set priority */}
          <select
            disabled={selectedIds.size === 0 || batchLoading}
            onChange={(e) => { if (e.target.value) handleBatchPriority(Number(e.target.value)); e.target.value = ''; }}
            className="soul-select text-[10px] h-6 px-1 rounded bg-elevated border border-border-default cursor-pointer disabled:opacity-50"
            defaultValue=""
          >
            <option value="" disabled>Priority...</option>
            {PRIORITY_OPTIONS.map((p) => (
              <option key={p.value} value={p.value}>{p.label}</option>
            ))}
          </select>

          {/* Assign product */}
          {products && products.length > 0 && (
            <select
              disabled={selectedIds.size === 0 || batchLoading}
              onChange={(e) => { if (e.target.value) handleBatchProduct(e.target.value); e.target.value = ''; }}
              className="soul-select text-[10px] h-6 px-1 rounded bg-elevated border border-border-default cursor-pointer disabled:opacity-50"
              defaultValue=""
            >
              <option value="" disabled>Product...</option>
              {products.map((p) => (
                <option key={p} value={p}>{p}</option>
              ))}
            </select>
          )}

          {/* Delete */}
          <button
            type="button"
            disabled={selectedIds.size === 0 || batchLoading}
            onClick={handleBatchDelete}
            className="text-[10px] px-2 py-0.5 rounded bg-stage-blocked/15 text-stage-blocked hover:bg-stage-blocked/25 transition-colors cursor-pointer disabled:opacity-50"
          >
            Delete
          </button>

          <div className="flex-1" />
          <button
            type="button"
            onClick={exitBatch}
            data-testid="kanban-exit-batch"
            className="text-[10px] text-fg-secondary hover:text-fg cursor-pointer"
          >
            Cancel
          </button>
        </div>
      ) : (
        <div className="flex items-center justify-end px-3 py-1 shrink-0">
          <button
            type="button"
            onClick={() => setBatchMode(true)}
            data-testid="kanban-batch-mode"
            className="text-[10px] text-fg-muted hover:text-fg cursor-pointer"
            title="Select multiple tasks for bulk actions"
          >
            Select
          </button>
        </div>
      )}

      {/* Kanban columns */}
      <div className="flex gap-px flex-1 overflow-x-auto overflow-y-hidden">
        {visibleStages.map((stage) => (
          <StageColumn
            key={stage}
            stage={stage}
            tasks={tasksByStage[stage]}
            onTaskClick={batchMode ? undefined : onTaskClick}
            onTaskSelect={batchMode ? toggleSelect : undefined}
            selectedIds={batchMode ? selectedIds : undefined}
            taskActivities={taskActivities}
          />
        ))}
      </div>
    </div>
  );
}
