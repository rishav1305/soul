import type { PlannerTask, GridSubView } from '../../../lib/types.ts';
import CompactGrid from './CompactGrid.tsx';
import TableView from './TableView.tsx';
import GroupedList from './GroupedList.tsx';

interface GridViewProps {
  tasks: PlannerTask[];
  subView: GridSubView;
  onSubViewChange: (v: GridSubView) => void;
  onTaskClick: (task: PlannerTask) => void;
}

const TABS: { value: GridSubView; label: string }[] = [
  { value: 'grid', label: 'Grid' },
  { value: 'table', label: 'Table' },
  { value: 'grouped', label: 'Grouped' },
];

export default function GridView({ tasks, subView, onSubViewChange, onTaskClick }: GridViewProps) {
  return (
    <div className="flex flex-col h-full">
      {/* Pill selector tabs */}
      <div className="flex items-center gap-1 px-3 py-1.5 border-b border-zinc-800 shrink-0">
        {TABS.map((tab) => (
          <button
            key={tab.value}
            type="button"
            onClick={() => onSubViewChange(tab.value)}
            className={`px-2.5 py-1 text-xs cursor-pointer transition-colors ${
              subView === tab.value
                ? 'text-zinc-100 border-b-2 border-sky-500'
                : 'text-zinc-500 hover:text-zinc-300 border-b-2 border-transparent'
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Sub-view content */}
      <div className="flex-1 overflow-hidden">
        {subView === 'grid' && <CompactGrid tasks={tasks} onTaskClick={onTaskClick} />}
        {subView === 'table' && <TableView tasks={tasks} onTaskClick={onTaskClick} />}
        {subView === 'grouped' && <GroupedList tasks={tasks} onTaskClick={onTaskClick} />}
      </div>
    </div>
  );
}
