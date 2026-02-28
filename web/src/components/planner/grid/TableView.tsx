import { useState, useMemo } from 'react';
import type { PlannerTask, TaskStage, TaskSubstep } from '../../../lib/types.ts';

interface TableViewProps {
  tasks: PlannerTask[];
  onTaskClick: (task: PlannerTask) => void;
}

type SortKey = 'id' | 'title' | 'stage' | 'priority' | 'product' | 'substep';
type SortDir = 'asc' | 'desc';

const STAGE_COLORS: Record<TaskStage, string> = {
  active: 'text-green-400',
  backlog: 'text-zinc-400',
  brainstorm: 'text-violet-400',
  blocked: 'text-red-400',
  validation: 'text-amber-400',
  done: 'text-sky-400',
};

const PRIORITY_CONFIG: Record<number, { label: string; color: string }> = {
  0: { label: 'Low', color: 'text-zinc-500' },
  1: { label: 'Norm', color: 'text-zinc-400' },
  2: { label: 'High', color: 'text-amber-400' },
  3: { label: 'Crit', color: 'text-red-400' },
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
  { key: 'id', label: 'ID', className: 'w-16' },
  { key: 'title', label: 'Title' },
  { key: 'stage', label: 'Stage', className: 'w-24' },
  { key: 'priority', label: 'Priority', className: 'w-20' },
  { key: 'product', label: 'Product', className: 'w-28' },
  { key: 'substep', label: 'Substep', className: 'w-24' },
];

function substepIndex(substep: TaskSubstep | ''): number {
  if (!substep) return -1;
  return SUBSTEP_ORDER.indexOf(substep);
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
        <thead className="sticky top-0 bg-zinc-900 z-10">
          <tr className="border-b border-zinc-800">
            {COLUMNS.map((col) => (
              <th
                key={col.key}
                className={`text-left px-3 py-2 text-zinc-400 font-medium cursor-pointer hover:text-zinc-200 select-none ${col.className ?? ''}`}
                onClick={() => handleSort(col.key)}
              >
                <span>{col.label}</span>
                {sortKey === col.key && (
                  <span className="ml-1 text-sky-400">{sortDir === 'asc' ? '\u2191' : '\u2193'}</span>
                )}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {sorted.map((task) => {
            const prio = PRIORITY_CONFIG[task.priority] ?? PRIORITY_CONFIG[1];
            const subIdx = substepIndex(task.substep);
            const subLabel =
              task.substep && subIdx >= 0
                ? `${SUBSTEP_LABELS[task.substep]} ${subIdx + 1}/${SUBSTEP_ORDER.length}`
                : '\u2014';

            return (
              <tr
                key={task.id}
                onClick={() => onTaskClick(task)}
                className="border-b border-zinc-800/50 hover:bg-zinc-900/70 cursor-pointer"
              >
                <td className="px-3 py-1.5 text-zinc-500">#{task.id}</td>
                <td className="px-3 py-1.5 text-zinc-200 truncate max-w-0">{task.title}</td>
                <td className={`px-3 py-1.5 uppercase ${STAGE_COLORS[task.stage]}`}>{task.stage}</td>
                <td className={`px-3 py-1.5 ${prio.color}`}>{prio.label}</td>
                <td className="px-3 py-1.5 text-zinc-500 truncate max-w-0">{task.product || '\u2014'}</td>
                <td className="px-3 py-1.5 text-sky-400">{subLabel}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
