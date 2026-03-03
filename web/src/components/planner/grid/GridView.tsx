import type { PlannerTask } from '../../../lib/types.ts';
import CompactGrid from './CompactGrid.tsx';

interface GridViewProps {
  tasks: PlannerTask[];
  onTaskClick: (task: PlannerTask) => void;
}

export default function GridView({ tasks, onTaskClick }: GridViewProps) {
  return (
    <div className="flex flex-col h-full">
      <CompactGrid tasks={tasks} onTaskClick={onTaskClick} />
    </div>
  );
}
