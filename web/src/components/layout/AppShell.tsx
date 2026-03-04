import { useState, useMemo, useCallback, useEffect, useRef } from 'react';
import { useLayoutStore } from '../../hooks/useLayoutStore.ts';
import { usePlanner } from '../../hooks/usePlanner.ts';
import { useSessions } from '../../hooks/useSessions.ts';
import { useWebSocket } from '../../hooks/useWebSocket.ts';
import { useNotifications } from '../../hooks/useNotifications.ts';
import { useProductContext } from '../../hooks/useProductContext.ts';
import { useChat } from '../../hooks/useChat.ts';
import type { PlannerTask, TaskStage } from '../../lib/types.ts';
import ProductRail from '../ProductRail.tsx';
import SessionsDrawer from '../SessionsDrawer.tsx';
import ProductView from '../ProductView.tsx';
import HorizontalRail from '../HorizontalRail.tsx';
import ToastStack from '../ToastStack.tsx';
import SettingsPanel from '../SettingsPanel.tsx';
import TaskDetail from '../planner/TaskDetail.tsx';

function emptyByStage(): Record<TaskStage, PlannerTask[]> {
  return { backlog: [], brainstorm: [], active: [], blocked: [], validation: [], done: [] };
}

export default function AppShell() {
  const layout = useLayoutStore();
  const planner = usePlanner();
  const { sessions, activeSessionId, createSession, switchSession } = useSessions();
  const { connected } = useWebSocket();
  const { messages } = useChat();

  const [sessionsOpen, setSessionsOpen] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [selectedTask, setSelectedTask] = useState<PlannerTask | null>(null);
  const [contextChipDismissed, setContextChipDismissed] = useState(false);
  const prevProductRef = useRef<string | null>(null);

  // Notifications
  const taskRefs = useMemo(
    () => planner.tasks.map((t) => ({ id: t.id, title: t.title })),
    [planner.tasks],
  );
  const { toasts, dismiss: dismissToast } = useNotifications(taskRefs, layout.toastsEnabled);

  // Derive unique products from tasks
  const products = useMemo(() => {
    const set = new Set<string>();
    for (const t of planner.tasks) {
      if (t.product) set.add(t.product);
    }
    return Array.from(set).sort();
  }, [planner.tasks]);

  // Filtered tasks
  const filteredTasks = useMemo(() => {
    return planner.tasks.filter((t) => {
      if (layout.filters.stage !== 'all' && t.stage !== layout.filters.stage) return false;
      if (layout.filters.priority !== 'all' && t.priority !== layout.filters.priority) return false;
      if (layout.filters.product !== 'all' && t.product !== layout.filters.product) return false;
      return true;
    });
  }, [planner.tasks, layout.filters]);

  const filteredByStage = useMemo(() => {
    const grouped = emptyByStage();
    for (const t of filteredTasks) grouped[t.stage].push(t);
    return grouped;
  }, [filteredTasks]);

  // Active/blocked counts for horizontal rail pill
  const activeTaskCount = useMemo(() => {
    const tasks = layout.activeProduct
      ? planner.tasks.filter((t) => t.product === layout.activeProduct)
      : planner.tasks;
    return tasks.filter((t) => t.stage === 'active').length;
  }, [planner.tasks, layout.activeProduct]);

  const blockedTaskCount = useMemo(() => {
    const tasks = layout.activeProduct
      ? planner.tasks.filter((t) => t.product === layout.activeProduct)
      : planner.tasks;
    return tasks.filter((t) => t.stage === 'blocked').length;
  }, [planner.tasks, layout.activeProduct]);

  // Last message snippet for collapsed rail
  const lastMessageSnippet = useMemo(() => {
    if (messages.length === 0) return '';
    const last = messages[messages.length - 1];
    return last.content.slice(0, 60) + (last.content.length > 60 ? '...' : '');
  }, [messages]);

  // Context
  const { chipLabel, contextString } = useProductContext(planner.tasks, layout.activeProduct);

  // Show context chip when product switches in existing chat session
  const showContextChipInBar = layout.showContextChip && !contextChipDismissed && !!chipLabel && messages.length > 0;

  // Reset chip dismissed state on product change
  useEffect(() => {
    if (prevProductRef.current !== layout.activeProduct) {
      setContextChipDismissed(false);
      prevProductRef.current = layout.activeProduct;
    }
  }, [layout.activeProduct]);

  const handleInjectContext = useCallback(() => {
    // Inject context as a system-style user message — prepend to next send via contextString
    // We expose the chip and the user clicks to inject — handled as a send with context prefix
    setContextChipDismissed(true);
  }, []);

  const contextChips = showContextChipInBar
    ? [{
        label: chipLabel!,
        onInject: handleInjectContext,
        onDismiss: () => setContextChipDismissed(true),
      }]
    : [];

  return (
    <div className="h-screen bg-deep text-fg font-body noise flex overflow-hidden">
      {/* Left Rail — always 56px */}
      <ProductRail
        tasks={planner.tasks}
        activeProduct={layout.activeProduct}
        onProductSelect={layout.setActiveProduct}
        onLogoClick={() => setSessionsOpen(true)}
        onSettingsClick={() => setSettingsOpen(true)}
      />

      {/* Main area: fills remaining space, with bottom/top padding for HorizontalRail */}
      <div
        className="flex-1 min-w-0 overflow-hidden"
        style={{
          paddingBottom: layout.chatPosition === 'bottom'
            ? layout.railExpanded ? layout.railHeight : 48
            : 0,
          paddingTop: layout.chatPosition === 'top'
            ? layout.railExpanded ? layout.railHeight : 48
            : 0,
        }}
      >
        <ProductView
          activeProduct={layout.activeProduct}
          tasks={planner.tasks}
          filteredTasks={filteredTasks}
          tasksByStage={filteredByStage}
          products={products}
          loading={planner.loading}
          taskView={layout.taskView}
          gridSubView={layout.gridSubView}
          panelWidth={layout.panelWidth}
          filters={layout.filters}
          setTaskView={layout.setTaskView}
          setGridSubView={layout.setGridSubView}
          setPanelWidth={layout.setPanelWidth}
          setFilters={layout.setFilters}
          createTask={planner.createTask}
          updateTask={planner.updateTask}
          moveTask={planner.moveTask}
          deleteTask={planner.deleteTask}
          taskActivities={planner.taskActivities}
          taskStreams={planner.taskStreams}
          taskComments={planner.taskComments}
          fetchComments={planner.fetchComments}
          addComment={planner.addComment}
        />
      </div>

      {/* Horizontal Rail */}
      <HorizontalRail
        position={layout.chatPosition}
        expanded={layout.railExpanded}
        railHeight={layout.railHeight}
        chatSplit={layout.chatSplit}
        tasks={planner.tasks}
        activeProduct={layout.activeProduct}
        lastMessageSnippet={lastMessageSnippet}
        activeTaskCount={activeTaskCount}
        blockedTaskCount={blockedTaskCount}
        contextChips={contextChips}
        contextString={layout.autoInjectContext ? contextString : undefined}
        onToggle={() => layout.setRailExpanded(!layout.railExpanded)}
        onHeightChange={layout.setRailHeight}
        onTaskClick={setSelectedTask}
        taskActivities={planner.taskActivities}
        sessions={sessions}
        activeSessionId={activeSessionId}
        onNewSession={createSession}
        onSessionSelect={switchSession}
      />

      {/* Sessions drawer */}
      <SessionsDrawer
        open={sessionsOpen}
        onClose={() => setSessionsOpen(false)}
        sessions={sessions}
        activeSessionId={activeSessionId}
        onSessionSelect={switchSession}
        onNewChat={createSession}
        connected={connected}
      />

      {/* Settings panel */}
      <SettingsPanel
        open={settingsOpen}
        onClose={() => setSettingsOpen(false)}
        chatPosition={layout.chatPosition}
        chatSplit={layout.chatSplit}
        autoInjectContext={layout.autoInjectContext}
        showContextChip={layout.showContextChip}
        toastsEnabled={layout.toastsEnabled}
        inlineBadgesEnabled={layout.inlineBadgesEnabled}
        onChatPosition={layout.setChatPosition}
        onChatSplit={layout.setChatSplit}
        onAutoInjectContext={layout.setAutoInjectContext}
        onShowContextChip={layout.setShowContextChip}
        onToastsEnabled={layout.setToastsEnabled}
        onInlineBadgesEnabled={layout.setInlineBadgesEnabled}
      />

      {/* Toast stack */}
      <ToastStack toasts={toasts} onDismiss={dismissToast} />

      {/* Task detail modal — opened from HorizontalRail task list */}
      {selectedTask && (
        <TaskDetail
          task={planner.tasks.find((t) => t.id === selectedTask.id) ?? selectedTask}
          onClose={() => setSelectedTask(null)}
          onMove={async (id, stage, comment) => {
            await planner.moveTask(id, stage, comment);
            setSelectedTask(null);
          }}
          onUpdate={async (id, updates) => {
            const updated = await planner.updateTask(id, updates);
            setSelectedTask(updated);
            return updated;
          }}
          onDelete={async (id) => {
            await planner.deleteTask(id);
            setSelectedTask(null);
          }}
          activities={planner.taskActivities[selectedTask.id] || []}
          streamContent={planner.taskStreams[selectedTask.id] || ''}
          products={products}
          comments={planner.taskComments[selectedTask.id]}
          onFetchComments={planner.fetchComments}
          onAddComment={planner.addComment}
        />
      )}
    </div>
  );
}
