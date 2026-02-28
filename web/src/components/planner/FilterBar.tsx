import type { TaskFilters, TaskStage } from '../../lib/types.ts';

interface FilterBarProps {
  filters: TaskFilters;
  products: string[];
  onChange: (f: Partial<TaskFilters>) => void;
}

const STAGE_OPTIONS: { value: TaskStage | 'all'; label: string }[] = [
  { value: 'all', label: 'All Stages' },
  { value: 'backlog', label: 'Backlog' },
  { value: 'brainstorm', label: 'Brainstorm' },
  { value: 'active', label: 'Active' },
  { value: 'blocked', label: 'Blocked' },
  { value: 'validation', label: 'Validation' },
  { value: 'done', label: 'Done' },
];

const PRIORITY_OPTIONS: { value: number | 'all'; label: string }[] = [
  { value: 'all', label: 'All Priorities' },
  { value: 3, label: 'Critical (3)' },
  { value: 2, label: 'High (2)' },
  { value: 1, label: 'Normal (1)' },
  { value: 0, label: 'Low (0)' },
];

const selectClass =
  'bg-zinc-900 border border-zinc-700 rounded px-2 py-1 text-xs text-zinc-300 outline-none focus:border-zinc-500 cursor-pointer appearance-none';

export default function FilterBar({ filters, products, onChange }: FilterBarProps) {
  return (
    <div className="flex items-center gap-3 px-3 py-1.5 border-b border-zinc-800 shrink-0 text-xs">
      {/* Stage filter */}
      <select
        className={selectClass}
        value={filters.stage}
        onChange={(e) => onChange({ stage: e.target.value as TaskStage | 'all' })}
      >
        {STAGE_OPTIONS.map((opt) => (
          <option key={opt.value} value={opt.value}>
            {opt.label}
          </option>
        ))}
      </select>

      {/* Priority filter */}
      <select
        className={selectClass}
        value={filters.priority === 'all' ? 'all' : String(filters.priority)}
        onChange={(e) => {
          const val = e.target.value;
          onChange({ priority: val === 'all' ? 'all' : Number(val) });
        }}
      >
        {PRIORITY_OPTIONS.map((opt) => (
          <option key={String(opt.value)} value={String(opt.value)}>
            {opt.label}
          </option>
        ))}
      </select>

      {/* Product filter */}
      <select
        className={selectClass}
        value={filters.product}
        onChange={(e) => onChange({ product: e.target.value as string | 'all' })}
      >
        <option value="all">All Products</option>
        {products.map((p) => (
          <option key={p} value={p}>
            {p}
          </option>
        ))}
      </select>
    </div>
  );
}
