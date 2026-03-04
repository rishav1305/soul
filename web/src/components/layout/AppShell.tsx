import { useMemo, useCallback } from 'react';
import { useLayoutStore } from '../../hooks/useLayoutStore.ts';
import { usePlanner } from '../../hooks/usePlanner.ts';
import { useSessions } from '../../hooks/useSessions.ts';
import { useWebSocket } from '../../hooks/useWebSocket.ts';
import { useNotifications } from '../../hooks/useNotifications.ts';
import { useChat } from '../../hooks/useChat.ts';
import { useProductContext } from '../../hooks/useProductContext.ts';
import type { PlannerTask, TaskStage } from '../../lib/types.ts';
import ProductRail from './ProductRail.tsx';
import ProductView from './ProductView.tsx';
import HorizontalRail from './HorizontalRail.tsx';
import SessionsDrawer from './SessionsDrawer.tsx';
import SettingsPanel from './SettingsPanel.tsx';
import ToastStack from './ToastStack.tsx';

function emptyByStage(): Record<TaskStage, PlannerTask[]> {
  return { backlog: [], brainstorm: [], active: [], blocked: [], validation: [], done: [] };
}

export default function AppShell() {
  const layout = useLayoutStore();
  const planner = usePlanner();
  const { sessions, activeSessionId, createSession, switchSession } = useSessions();
  const { connected } = useWebSocket();
  const { notifications, dismiss } = useNotifications(planner.tasks);
  const { messages } = useChat();

  // Derive unique products dynamically from tasks (must be before useProductContext)
  const products = useMemo(() => {
    const set = new Set<string>(['Soul', 'compliance', 'compliance-go', 'scout']);
    for (const t of planner.tasks) {
      if (t.product) set.add(t.product);
    }
    return Array.from(set).sort();
  }, [planner.tasks]);

  const { buildContextString } = useProductContext(planner.tasks, layout.activeProduct, products);

  // Filtered tasks for the main product view
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

  // Last chat message snippet for rail bar
  const lastChatSnippet = useMemo(() => {
    const last = [...messages].reverse().find((m) => m.role === 'assistant');
    if (!last?.content) return undefined;
    return last.content.slice(0, 80) + (last.content.length > 80 ? '…' : '');
  }, [messages]);

  const handleSessionCreated = useCallback((id: number) => {
    switchSession(id);
  }, [switchSession]);

  return (
    <div
      className={`h-screen bg-deep text-fg font-body noise overflow-hidden flex ${
        layout.railPosition === 'bottom' ? 'flex-col' : 'flex-col-reverse'
      }`}
    >
      {/* ── Main area: Left rail + Product view ── */}
      <div className="flex flex-1 min-h-0 overflow-hidden relative">
        {/* Left Product Rail — always fixed 56px */}
        <ProductRail
          products={products}
          activeProduct={layout.activeProduct}
          tasks={planner.tasks}
          onProductSelect={layout.setActiveProduct}
          onSessionsToggle={() => layout.setSessionsOpen(!layout.sessionsOpen)}
          onSettingsToggle={() => layout.setSettingsOpen(!layout.settingsOpen)}
          sessionsOpen={layout.sessionsOpen}
          settingsOpen={layout.settingsOpen}
        />

        {/* Product content — fills remaining space */}
        <div className="flex-1 min-w-0 overflow-hidden">
          <ProductView
            activeProduct={layout.activeProduct}
            taskView={layout.taskView}
            gridSubView={layout.gridSubView}
            panelWidth={layout.panelWidth}
            filters={layout.filters}
            setTaskView={layout.setTaskView}
            setGridSubView={layout.setGridSubView}
            setPanelWidth={layout.setPanelWidth}
            setFilters={layout.setFilters}
            tasks={planner.tasks}
            filteredTasks={filteredTasks}
            tasksByStage={filteredByStage}
            products={products}
            loading={planner.loading}
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

        {/* Overlay drawers — rendered inside the main area */}
        {layout.sessionsOpen && (
          <SessionsDrawer
            onClose={() => layout.setSessionsOpen(false)}
            sessions={sessions}
            activeSessionId={activeSessionId}
            onSessionSelect={switchSession}
            onNewChat={createSession}
            connected={connected}
          />
        )}

        {layout.settingsOpen && (
          <SettingsPanel
            onClose={() => layout.setSettingsOpen(false)}
            railPosition={layout.railPosition}
            setRailPosition={layout.setRailPosition}
            chatSplitPct={layout.chatSplitPct}
            setChatSplitPct={layout.setChatSplitPct}
            autoInjectContext={layout.autoInjectContext}
            setAutoInjectContext={layout.setAutoInjectContext}
            showContextChip={layout.showContextChip}
            setShowContextChip={layout.setShowContextChip}
            toastsEnabled={layout.toastsEnabled}
            setToastsEnabled={layout.setToastsEnabled}
            inlineBadgesEnabled={layout.inlineBadgesEnabled}
            setInlineBadgesEnabled={layout.setInlineBadgesEnabled}
          />
        )}
      </div>

      {/* ── Horizontal Rail: Chat + Tasks (bottom or top) ── */}
      <HorizontalRail
        expanded={layout.railExpanded}
        heightVh={layout.railHeightVh}
        tab={layout.railTab}
        chatSplitPct={layout.chatSplitPct}
        position={layout.railPosition}
        onToggleExpand={() => layout.setRailExpanded(!layout.railExpanded)}
        onSetTab={layout.setRailTab}
        onHeightChange={layout.setRailHeightVh}
        activeSessionId={activeSessionId}
        onSessionCreated={handleSessionCreated}
        lastChatSnippet={lastChatSnippet}
        tasks={planner.tasks}
        activeProduct={layout.activeProduct}
        taskActivities={planner.taskActivities}
        taskStreams={planner.taskStreams}
        taskComments={planner.taskComments}
        updateTask={planner.updateTask}
        moveTask={planner.moveTask}
        deleteTask={planner.deleteTask}
        fetchComments={planner.fetchComments}
        addComment={planner.addComment}
        products={products}
        createTask={planner.createTask}
        buildContextString={buildContextString}
        autoInjectContext={layout.autoInjectContext}
        showContextChip={layout.showContextChip}
        inlineBadgesEnabled={layout.inlineBadgesEnabled}
      />

      {/* ── Toast notifications ── */}
      {layout.toastsEnabled && <ToastStack notifications={notifications} onDismiss={dismiss} />}
    </div>
  );
}
