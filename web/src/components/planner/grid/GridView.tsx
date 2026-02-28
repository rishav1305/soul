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
      <div className="flex items-center gap-1 px-4 py-2 border-b border-border-subtle shrink-0">
        {TABS.map((tab) => (
          <button
            key={tab.value}
            type="button"
            onClick={() => onSubViewChange(tab.value)}
            className={`px-2.5 py-1 font-display text-xs cursor-pointer transition-colors ${
              subView === tab.value
                ? 'text-fg font-semibold border-b-2 border-soul'
                : 'text-fg-muted hover:text-fg-secondary border-b-2 border-transparent'
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
