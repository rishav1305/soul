import { useState, useMemo, useCallback } from 'react';
import { useLayoutStore, autoWidth } from '../../hooks/useLayoutStore.ts';
import { usePlanner } from '../../hooks/usePlanner.ts';
import type { PlannerTask, TaskStage } from '../../lib/types.ts';
import ChatRail from './ChatRail.tsx';
import TaskRail from './TaskRail.tsx';
import ResizeDivider from './ResizeDivider.tsx';
import ChatPanel from '../chat/ChatPanel.tsx';
import TaskPanel from '../planner/TaskPanel.tsx';

function emptyByStage(): Record<TaskStage, PlannerTask[]> {
  return { backlog: [], brainstorm: [], active: [], blocked: [], validation: [], done: [] };
}

export default function AppShell() {
  const layout = useLayoutStore();
  const planner = usePlanner();

  const [unreadCount, setUnreadCount] = useState(0);

  // Derive filtered tasks
  const filteredTasks = useMemo(() => {
    return planner.tasks.filter((t) => {
      if (layout.filters.stage !== 'all' && t.stage !== layout.filters.stage) return false;
      if (layout.filters.priority !== 'all' && t.priority !== layout.filters.priority) return false;
      if (layout.filters.product !== 'all' && t.product !== layout.filters.product) return false;
      return true;
    });
  }, [planner.tasks, layout.filters]);

  // Derive unique products
  const products = useMemo(() => {
    const set = new Set<string>();
    for (const t of planner.tasks) {
      if (t.product) set.add(t.product);
    }
    return Array.from(set).sort();
  }, [planner.tasks]);

  // tasksByStage for filtered tasks
  const filteredByStage = useMemo(() => {
    const grouped = emptyByStage();
    for (const t of filteredTasks) {
      grouped[t.stage].push(t);
    }
    return grouped;
  }, [filteredTasks]);

  // tasksByStage for ALL tasks (used by TaskRail)
  const allByStage = useMemo(() => {
    const grouped = emptyByStage();
    for (const t of planner.tasks) {
      grouped[t.stage].push(t);
    }
    return grouped;
  }, [planner.tasks]);

  // Panel width computations
  const taskPercent = layout.panelWidth ?? autoWidth(filteredTasks.length);
  const chatPercent = 100 - taskPercent;

  const bothOpen = layout.chatState === 'open' && layout.taskState === 'open';

  const handleResize = useCallback(
    (chatPct: number) => {
      layout.setPanelWidth(Math.round(100 - chatPct));
    },
    [layout.setPanelWidth],
  );

  const handleUnreadChange = useCallback((count: number) => {
    setUnreadCount(count);
  }, []);

  // When chat collapses, track incoming messages as unread
  const handleChatCollapse = useCallback(() => {
    layout.setChatState('rail');
  }, [layout.setChatState]);

  const handleTaskCollapse = useCallback(() => {
    layout.setTaskState('rail');
  }, [layout.setTaskState]);

  const handleChatExpand = useCallback(() => {
    layout.setChatState('open');
    setUnreadCount(0);
  }, [layout.setChatState]);

  const handleTaskExpand = useCallback(() => {
    layout.setTaskState('open');
  }, [layout.setTaskState]);

  return (
    <div className="h-screen bg-deep text-fg font-body noise flex overflow-hidden">
      {/* Chat: rail or panel */}
      {layout.chatState === 'rail' ? (
        <ChatRail unreadCount={unreadCount} onExpand={handleChatExpand} />
      ) : (
        <div
          className="h-full overflow-hidden transition-[width] duration-200 ease-in-out"
          style={{
            width: bothOpen ? `${chatPercent}%` : 'calc(100% - 40px)',
          }}
        >
          <ChatPanel
            onCollapse={handleChatCollapse}
            canCollapse={layout.canCollapse('chat')}
            onUnreadChange={handleUnreadChange}
          />
        </div>
      )}

      {/* Resize divider — only when both panels open */}
      {bothOpen && <ResizeDivider onResize={handleResize} />}

      {/* Task: rail or panel */}
      {layout.taskState === 'rail' ? (
        <TaskRail
          tasksByStage={allByStage}
          onExpand={handleTaskExpand}
          onNewTask={async (title, desc, priority, product) => {
            await planner.createTask(title, desc, priority, product);
          }}
        />
      ) : (
        <div
          className="h-full overflow-hidden transition-[width] duration-200 ease-in-out"
          style={{
            width: bothOpen ? `${taskPercent}%` : 'calc(100% - 40px)',
          }}
        >
          <TaskPanel
            taskView={layout.taskView}
            gridSubView={layout.gridSubView}
            panelWidth={layout.panelWidth}
            filters={layout.filters}
            setTaskView={layout.setTaskView}
            setGridSubView={layout.setGridSubView}
            setPanelWidth={layout.setPanelWidth}
            setFilters={layout.setFilters}
            canCollapse={layout.canCollapse('task')}
            onCollapse={handleTaskCollapse}
            tasks={planner.tasks}
            filteredTasks={filteredTasks}
            tasksByStage={filteredByStage}
            products={products}
            loading={planner.loading}
            createTask={planner.createTask}
            moveTask={planner.moveTask}
            deleteTask={planner.deleteTask}
          />
        </div>
      )}
    </div>
  );
}
