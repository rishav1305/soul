import { useState, useMemo, useCallback, useEffect } from 'react';
import { useLayoutStore } from '../../hooks/useLayoutStore.ts';
import { usePlanner } from '../../hooks/usePlanner.ts';
import { useNotifications } from '../../hooks/useNotifications.ts';
import { useProductContext } from '../../hooks/useProductContext.ts';
import { ChatSessionsProvider, useChatSessions } from '../../hooks/useChatSessions.tsx';
import { WebSocketContext, useWebSocketProvider } from '../../hooks/useWebSocketContext.ts';
import { authFetch } from '../../lib/api.ts';
import type { PlannerTask, TaskStage, TaskFilters, ProductInfo } from '../../lib/types.ts';
import ProductRail, { RAIL_WIDTH, PANEL_WIDTH } from './ProductRail.tsx';
import ProductView from './ProductView.tsx';
import HorizontalRail from './HorizontalRail.tsx';
import RightPanel from './RightPanel.tsx';
import SessionsDrawer from './SessionsDrawer.tsx';
import ToastStack from './ToastStack.tsx';
import TaskDetail from '../planner/TaskDetail.tsx';

function emptyByStage(): Record<TaskStage, PlannerTask[]> {
  return { backlog: [], brainstorm: [], active: [], blocked: [], validation: [], done: [] };
}

function AppShellInner() {
  const layout = useLayoutStore();
  const planner = usePlanner();
  const {
    sessions, activeSessionId, setActiveSessionId, createSession,
    messages, runningSessions, unreadSessions, connected,
  } = useChatSessions();
  const { toasts: notifications, dismiss } = useNotifications(planner.tasks, layout.toastsEnabled);

  const [selectedTask, setSelectedTask] = useState<PlannerTask | null>(null);

  // Fetch registered products from API on mount
  const [apiProducts, setApiProducts] = useState<ProductInfo[]>([]);
  useEffect(() => {
    authFetch('/api/products')
      .then((r) => r.json())
      .then((data) => { if (Array.isArray(data)) setApiProducts(data); })
      .catch(() => {});
  }, []);

  // Merge API products with task-discovered products
  // Only include task-discovered products that are also registered in the API
  // 'soul' is always first, then the rest sorted alphabetically
  const products = useMemo(() => {
    const set = new Set<string>();
    set.add('soul'); // Soul is always present
    for (const p of apiProducts) set.add(p.name);
    // Only add task-discovered products if they exist in the API registry
    const apiNames = new Set(apiProducts.map((p) => p.name));
    apiNames.add('soul');
    for (const t of planner.tasks) {
      if (t.product && apiNames.has(t.product)) set.add(t.product);
    }
    const arr = Array.from(set);
    const rest = arr.filter((p) => p.toLowerCase() !== 'soul').sort();
    return ['soul', ...rest];
  }, [apiProducts, planner.tasks]);

  // Forced single-select: if no product is active, default to 'soul'
  useEffect(() => {
    if (!layout.activeProduct && products.length > 0) {
      layout.setActiveProduct('soul');
    }
  }, [layout.activeProduct, products]);

  // Build product metadata map for downstream components
  const productMetadata = useMemo(() => {
    const map = new Map<string, ProductInfo>();
    for (const p of apiProducts) map.set(p.name, p);
    return map;
  }, [apiProducts]);

  const { buildContextString } = useProductContext(planner.tasks, layout.activeProduct, products);

  // Derive layout mode from panel positions
  const isMixedPosition = layout.chatPosition !== layout.tasksPosition;
  // Only force independent when both are horizontal but at different positions
  const isMixedHorizontal = isMixedPosition
    && layout.chatPosition !== 'right' && layout.tasksPosition !== 'right';
  const effectiveDrawerLayout = isMixedHorizontal ? 'independent' as const : layout.drawerLayout;

  // Which panels go where (horizontal rails)
  const topPanel = isMixedPosition
    ? (layout.chatPosition === 'top' ? 'chat' as const : layout.tasksPosition === 'top' ? 'tasks' as const : 'both' as const)
    : 'both' as const;
  const bottomPanel = isMixedPosition
    ? (layout.chatPosition === 'bottom' ? 'chat' as const : layout.tasksPosition === 'bottom' ? 'tasks' as const : 'both' as const)
    : 'both' as const;
  const hasTopRail = layout.chatPosition === 'top' || layout.tasksPosition === 'top';
  const hasBottomRail = layout.chatPosition === 'bottom' || layout.tasksPosition === 'bottom';

  // Right panel detection
  const hasRightPanel = layout.chatPosition === 'right' || layout.tasksPosition === 'right';
  const rightPanels: 'both' | 'chat' | 'tasks' = (layout.chatPosition === 'right' && layout.tasksPosition === 'right')
    ? 'both'
    : layout.chatPosition === 'right' ? 'chat' : 'tasks';

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
    setActiveSessionId(id);
  }, [setActiveSessionId]);

  // Shared props for HorizontalRail instances
  const railBaseProps = {
    tab: layout.railTab,
    chatSplitPct: layout.chatSplitPct,
    onSetTab: layout.setRailTab,
    onChatSplitChange: layout.setChatSplitPct,
    activeSessionId,
    sessions,
    onSessionCreated: handleSessionCreated,
    onSessionSelect: setActiveSessionId,
    onNewSession: createSession,
    runningSessions,
    unreadSessions,
    lastChatSnippet,
    tasks: planner.tasks,
    activeProduct: layout.activeProduct,
    taskActivities: planner.taskActivities,
    taskStreams: planner.taskStreams,
    taskComments: planner.taskComments,
    updateTask: planner.updateTask,
    moveTask: planner.moveTask,
    deleteTask: planner.deleteTask,
    fetchComments: planner.fetchComments,
    addComment: planner.addComment,
    products,
    createTask: planner.createTask,
    taskView: layout.taskView,
    gridSubView: layout.gridSubView,
    filters: layout.filters,
    setTaskView: layout.setTaskView,
    setGridSubView: layout.setGridSubView,
    setFilters: (partial: Partial<TaskFilters>) => layout.setFilters(partial),
    syncProductFilter: layout.syncProductFilter,
    onSyncProductFilterToggle: () => layout.setSyncProductFilter(!layout.syncProductFilter),
    buildContextString,
    autoInjectContext: layout.autoInjectContext,
    showContextChip: layout.showContextChip,
    inlineBadgesEnabled: layout.inlineBadgesEnabled,
  };

  // Per-rail expand/height props for mixed mode
  const railPropsForPanel = (panel: 'chat' | 'tasks') => {
    if (panel === 'chat') {
      return {
        expanded: layout.chatRailExpanded,
        heightVh: layout.chatRailHeightVh,
        onToggleExpand: () => layout.setChatRailExpanded(!layout.chatRailExpanded),
        onHeightChange: layout.setChatRailHeightVh,
      };
    }
    return {
      expanded: layout.tasksRailExpanded,
      heightVh: layout.tasksRailHeightVh,
      onToggleExpand: () => layout.setTasksRailExpanded(!layout.tasksRailExpanded),
      onHeightChange: layout.setTasksRailHeightVh,
    };
  };

  const railSpacer = <div className="shrink-0" style={{ width: layout.panelExpanded ? PANEL_WIDTH : RAIL_WIDTH }} />;

  const renderRail = (pos: 'top' | 'bottom', panels: 'both' | 'chat' | 'tasks') => (
    <div className="flex min-w-0">
      {railSpacer}
      <div className="flex-1 min-w-0">
        <HorizontalRail
          {...railBaseProps}
          position={pos}
          drawerLayout={effectiveDrawerLayout}
          visiblePanels={panels}
          {...(isMixedHorizontal && panels !== 'both'
            ? railPropsForPanel(panels as 'chat' | 'tasks')
            : {
                expanded: layout.railExpanded,
                heightVh: layout.railHeightVh,
                onToggleExpand: () => layout.setRailExpanded(!layout.railExpanded),
                onHeightChange: layout.setRailHeightVh,
              }
          )}
        />
      </div>
    </div>
  );

  return (
    <div data-testid="app-shell" className="h-screen bg-deep text-fg font-body noise overflow-hidden flex flex-col">
      {/* ── Left Product Rail (fixed position) ── */}
      <ProductRail
          products={products}
          activeProduct={layout.activeProduct}
          tasks={planner.tasks}
          productMetadata={productMetadata}
          onProductSelect={(p) => { if (p) layout.setActiveProduct(p); }}
          expanded={layout.panelExpanded}
          onToggleExpanded={() => layout.setPanelExpanded(!layout.panelExpanded)}
          settingsOpen={layout.settingsOpen}
          onSettingsToggle={() => layout.setSettingsOpen(!layout.settingsOpen)}
          chatPosition={layout.chatPosition}
          setChatPosition={layout.setChatPosition}
          tasksPosition={layout.tasksPosition}
          setTasksPosition={layout.setTasksPosition}
          drawerLayout={layout.drawerLayout}
          setDrawerLayout={layout.setDrawerLayout}
          autoInjectContext={layout.autoInjectContext}
          setAutoInjectContext={layout.setAutoInjectContext}
          showContextChip={layout.showContextChip}
          setShowContextChip={layout.setShowContextChip}
          toastsEnabled={layout.toastsEnabled}
          setToastsEnabled={layout.setToastsEnabled}
          inlineBadgesEnabled={layout.inlineBadgesEnabled}
          setInlineBadgesEnabled={layout.setInlineBadgesEnabled}
      />

      {/* ── Top rail (if any panel lives here) ── */}
      {hasTopRail && renderRail('top', topPanel)}

      {/* ── Main area ── */}
      <div className="flex flex-1 min-h-0 overflow-hidden relative" style={{ marginLeft: layout.panelExpanded ? PANEL_WIDTH : RAIL_WIDTH }}>
        <div className="flex-1 min-w-0 overflow-hidden">
          <ProductView
            activeProduct={layout.activeProduct}
            productMetadata={productMetadata}
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

        {layout.sessionsOpen && (
          <SessionsDrawer
            onClose={() => layout.setSessionsOpen(false)}
            sessions={sessions}
            activeSessionId={activeSessionId}
            onSessionSelect={setActiveSessionId}
            onNewChat={createSession}
            connected={connected}
          />
        )}

        {hasRightPanel && (
          <RightPanel
            visiblePanels={rightPanels}
            drawerLayout={effectiveDrawerLayout}
            chatExpanded={layout.rightChatExpanded}
            onToggleChatExpanded={() => layout.setRightChatExpanded(!layout.rightChatExpanded)}
            tasksExpanded={layout.rightTasksExpanded}
            onToggleTasksExpanded={() => layout.setRightTasksExpanded(!layout.rightTasksExpanded)}
            width={
              layout.rightChatExpanded && layout.rightTasksExpanded ? layout.rightPanelWidth
              : layout.rightChatExpanded ? layout.rightChatWidth
              : layout.rightTasksExpanded ? layout.rightTasksWidth
              : layout.rightPanelWidth
            }
            onWidthChange={
              layout.rightChatExpanded && layout.rightTasksExpanded ? layout.setRightPanelWidth
              : layout.rightChatExpanded ? layout.setRightChatWidth
              : layout.rightTasksExpanded ? layout.setRightTasksWidth
              : layout.setRightPanelWidth
            }
            chatSplitPct={layout.chatSplitPct}
            onChatSplitChange={layout.setChatSplitPct}
            activeSessionId={activeSessionId}
            sessions={sessions}
            onSessionCreated={handleSessionCreated}
            onSessionSelect={setActiveSessionId}
            onNewSession={createSession}
            activeProduct={layout.activeProduct}
            buildContextString={buildContextString}
            autoInjectContext={layout.autoInjectContext}
            showContextChip={layout.showContextChip}
            connected={connected}
            messageCount={messages.length}
            lastChatSnippet={lastChatSnippet}
            tasks={planner.tasks}
            taskView={layout.taskView}
            gridSubView={layout.gridSubView}
            filters={layout.filters}
            setTaskView={layout.setTaskView}
            setGridSubView={layout.setGridSubView}
            setFilters={(partial: Partial<TaskFilters>) => layout.setFilters(partial)}
            taskActivities={planner.taskActivities}
            taskStreams={planner.taskStreams}
            taskComments={planner.taskComments}
            updateTask={planner.updateTask}
            moveTask={planner.moveTask}
            deleteTask={planner.deleteTask}
            fetchComments={planner.fetchComments}
            addComment={planner.addComment}
            products={products}
            productMetadata={productMetadata}
            createTask={planner.createTask}
            syncProductFilter={layout.syncProductFilter}
            onSyncProductFilterToggle={() => layout.setSyncProductFilter(!layout.syncProductFilter)}
            inlineBadgesEnabled={layout.inlineBadgesEnabled}
          />
        )}
      </div>

      {/* ── Bottom rail (if any panel lives here) ── */}
      {hasBottomRail && renderRail('bottom', bottomPanel)}

      {/* ── Toast notifications ── */}
      {layout.toastsEnabled && <ToastStack notifications={notifications} onDismiss={dismiss} />}

      {/* Task detail modal */}
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
          productMetadata={productMetadata}
          comments={planner.taskComments[selectedTask.id]}
          onFetchComments={planner.fetchComments}
          onAddComment={planner.addComment}
        />
      )}
    </div>
  );
}

export default function AppShell() {
  const ws = useWebSocketProvider();
  return (
    <WebSocketContext.Provider value={ws}>
      <ChatSessionsProvider>
        <AppShellInner />
      </ChatSessionsProvider>
    </WebSocketContext.Provider>
  );
}
