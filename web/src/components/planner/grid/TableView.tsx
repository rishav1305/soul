import { useState, useMemo } from 'react';
import type { PlannerTask, TaskStage, TaskSubstep } from '../../../lib/types.ts';

interface TableViewProps {
  tasks: PlannerTask[];
  onTaskClick: (task: PlannerTask) => void;
}

type SortKey = 'id' | 'title' | 'stage' | 'priority' | 'product' | 'substep' | 'created_at';
type SortDir = 'asc' | 'desc';

const STAGE_COLORS: Record<TaskStage, string> = {
  active: 'text-stage-active',
  backlog: 'text-stage-backlog',
  brainstorm: 'text-stage-brainstorm',
  blocked: 'text-stage-blocked',
  validation: 'text-stage-validation',
  done: 'text-stage-done',
};

const PRIORITY_CONFIG: Record<number, { label: string; color: string }> = {
  0: { label: 'Low', color: 'text-priority-low' },
  1: { label: 'Norm', color: 'text-priority-normal' },
  2: { label: 'High', color: 'text-priority-high' },
  3: { label: 'Crit', color: 'text-priority-critical' },
};

const SUBSTEP_LABELS: Record<TaskSubstep, string> = {
  tdd: 'TDD',
  implementing: 'Impl',
  reviewing: 'Review',
  qa_test: 'QA',
  e2e_test: 'E2E',
  security_review: 'SecRev',
};

const SUBSTEP_ORDER: TaskSubstep[] = ['tdd', 'implementing', 'reviewing', 'qa_test', 'e2e_test', 'security_review'];

const COLUMNS: { key: SortKey; label: string; className?: string }[] = [
  { key: 'id', label: 'ID', className: 'w-14' },
  { key: 'title', label: 'Title' },
  { key: 'stage', label: 'Stage', className: 'w-24' },
  { key: 'priority', label: 'Priority', className: 'w-20' },
  { key: 'product', label: 'Product', className: 'w-28' },
  { key: 'substep', label: 'Substep', className: 'w-24' },
  { key: 'created_at', label: 'Created', className: 'w-24' },
];

function substepIndex(substep: TaskSubstep | ''): number {
  if (!substep) return -1;
  return SUBSTEP_ORDER.indexOf(substep);
}

function relativeTime(dateStr: string): string {
  if (!dateStr) return '\u2014';
  const now = Date.now();
  const then = new Date(dateStr).getTime();
  if (isNaN(then)) return '\u2014';
  const diff = now - then;
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;
  return `${Math.floor(days / 30)}mo ago`;
}

export default function TableView({ tasks, onTaskClick }: TableViewProps) {
  const [sortKey, setSortKey] = useState<SortKey>('priority');
  const [sortDir, setSortDir] = useState<SortDir>('desc');

  const sorted = useMemo(() => {
    return [...tasks].sort((a, b) => {
      let cmp = 0;
      switch (sortKey) {
        case 'id':
          cmp = a.id - b.id;
          break;
        case 'title':
          cmp = a.title.localeCompare(b.title);
          break;
        case 'stage':
          cmp = a.stage.localeCompare(b.stage);
          break;
        case 'priority':
          cmp = a.priority - b.priority;
          break;
        case 'product':
          cmp = (a.product || '').localeCompare(b.product || '');
          break;
        case 'substep':
          cmp = substepIndex(a.substep) - substepIndex(b.substep);
          break;
        case 'created_at':
          cmp = new Date(a.created_at).getTime() - new Date(b.created_at).getTime();
          break;
      }
      return sortDir === 'asc' ? cmp : -cmp;
    });
  }, [tasks, sortKey, sortDir]);

  const handleSort = (key: SortKey) => {
    if (sortKey === key) {
      setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
    } else {
      setSortKey(key);
      setSortDir(key === 'priority' ? 'desc' : 'asc');
    }
  };

  return (
    <div className="h-full overflow-auto">
      <table className="w-full text-xs border-collapse">
        <thead className="sticky top-0 bg-surface z-10">
          <tr className="border-b border-border-subtle">
            {COLUMNS.map((col) => (
              <th
                key={col.key}
                className={`text-left px-3 py-2 text-fg-muted font-display text-[11px] uppercase tracking-wider font-semibold cursor-pointer hover:text-fg-secondary select-none ${col.className ?? ''}`}
                onClick={() => handleSort(col.key)}
              >
                <span>{col.label}</span>
                {sortKey === col.key && (
                  <span className="ml-1 text-soul">{sortDir === 'asc' ? '\u2191' : '\u2193'}</span>
                )}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {sorted.map((task) => {
            const prio = PRIORITY_CONFIG[task.priority] ?? { label: 'Norm', color: 'text-priority-normal' };
            const subIdx = substepIndex(task.substep);
            const subLabel =
              task.substep && subIdx >= 0
                ? `${SUBSTEP_LABELS[task.substep]} ${subIdx + 1}/${SUBSTEP_ORDER.length}`
                : '\u2014';

            return (
              <tr
                key={task.id}
                onClick={() => onTaskClick(task)}
                className="border-b border-border-subtle hover:bg-elevated/60 cursor-pointer transition-colors"
              >
                <td className="px-3 py-1.5 text-fg-muted font-mono">#{task.id}</td>
                <td className="px-3 py-1.5 text-fg truncate max-w-0">
                  <div className="truncate">{task.title}</div>
                  {task.description && (
                    <div className="text-[10px] text-fg-muted truncate mt-0.5">{task.description}</div>
                  )}
                </td>
                <td className={`px-3 py-1.5 uppercase ${STAGE_COLORS[task.stage]}`}>{task.stage}</td>
                <td className={`px-3 py-1.5 ${prio.color}`}>{prio.label}</td>
                <td className="px-3 py-1.5 text-fg-muted truncate max-w-0">{task.product || '\u2014'}</td>
                <td className="px-3 py-1.5 text-fg-muted">{subLabel}</td>
                <td className="px-3 py-1.5 text-fg-muted whitespace-nowrap">{relativeTime(task.created_at)}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
