import { useRef, useState, useCallback, useEffect } from 'react';
import type {
  PlannerTask,
  TaskStage,
  TaskView,
  GridSubView,
  TaskFilters,
  PlannerActivity,
  TaskComment,
  DrawerLayout,
  ChatSession,
  ProductInfo,
} from '../../lib/types.ts';
import ChatView from '../chat/ChatView.tsx';
import SessionDrawer from '../chat/SessionDrawer.tsx';
import TaskContent from '../planner/TaskContent.tsx';
import TaskDetail from '../planner/TaskDetail.tsx';
import NewTaskForm from '../planner/NewTaskForm.tsx';

const RAIL_WIDTH = 48;

// ── Internal split divider ──
function SplitDivider({ direction, onSplitChange, containerRef }: {
  direction: 'vertical' | 'horizontal';
  onSplitChange: (pct: number) => void;
  containerRef: React.RefObject<HTMLDivElement | null>;
}) {
  const isVert = direction === 'vertical';
  const onMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    const container = containerRef.current;
    if (!container) return;
    const rect = container.getBoundingClientRect();
    const onMove = (ev: MouseEvent) => {
      const pct = isVert
        ? ((ev.clientX - rect.left) / rect.width) * 100
        : ((ev.clientY - rect.top) / rect.height) * 100;
      onSplitChange(Math.min(80, Math.max(20, pct)));
    };
    const onUp = () => { document.removeEventListener('mousemove', onMove); document.removeEventListener('mouseup', onUp); };
    document.addEventListener('mousemove', onMove);
    document.addEventListener('mouseup', onUp);
  }, [isVert, onSplitChange, containerRef]);

  return (
    <div
      onMouseDown={onMouseDown}
      className={`${isVert ? 'w-px self-stretch cursor-col-resize' : 'h-px w-full cursor-row-resize'} shrink-0 group relative z-10 bg-border-subtle`}
      title="Drag to resize"
    >
      <div className={`absolute ${isVert ? 'inset-y-0 -inset-x-1.5' : 'inset-x-0 -inset-y-1.5'} group-hover:bg-soul/30 transition-colors rounded-sm`} />
    </div>
  );
}

// ── Stage colors ──
const STAGE_DOT: Record<TaskStage, string> = {
  backlog: 'bg-stage-backlog',
  brainstorm: 'bg-stage-brainstorm',
  active: 'bg-stage-active',
  blocked: 'bg-stage-blocked',
  validation: 'bg-stage-validation',
  done: 'bg-stage-done',
};

interface RightPanelProps {
  visiblePanels: 'both' | 'chat' | 'tasks';
  drawerLayout: DrawerLayout;
  chatExpanded: boolean;
  onToggleChatExpanded: () => void;
  tasksExpanded: boolean;
  onToggleTasksExpanded: () => void;
  width: number;
  onWidthChange: (w: number) => void;
  chatSplitPct: number;
  onChatSplitChange: (pct: number) => void;
  // Chat
  activeSessionId: number | null;
  sessions: ChatSession[];
  onSessionCreated?: (id: number) => void;
  onSessionSelect: (id: number) => void;
  onNewSession: () => void;
  activeProduct: string | null;
  buildContextString?: () => string;
  autoInjectContext?: boolean;
  showContextChip?: boolean;
  connected: boolean;
  messageCount: number;
  lastChatSnippet?: string;
  // Tasks
  tasks: PlannerTask[];
  taskView: TaskView;
  gridSubView: GridSubView;
  filters: TaskFilters;
  setTaskView: (v: TaskView) => void;
  setGridSubView: (v: GridSubView) => void;
  setFilters: (partial: Partial<TaskFilters>) => void;
  taskActivities: Record<number, PlannerActivity[]>;
  taskStreams: Record<number, string>;
  taskComments: Record<number, TaskComment[]>;
  updateTask: (id: number, updates: Partial<PlannerTask>) => Promise<PlannerTask>;
  moveTask: (id: number, stage: TaskStage, comment: string) => Promise<PlannerTask>;
  deleteTask: (id: number) => Promise<void>;
  fetchComments: (id: number) => Promise<TaskComment[]>;
  addComment: (id: number, body: string) => Promise<TaskComment>;
  products: string[];
  productMetadata?: Map<string, ProductInfo>;
  createTask: (title: string, description: string, priority: number, product: string) => Promise<PlannerTask>;
  syncProductFilter: boolean;
  onSyncProductFilterToggle: () => void;
  inlineBadgesEnabled?: boolean;
}

export default function RightPanel({
  visiblePanels,
  drawerLayout,
  chatExpanded,
  onToggleChatExpanded,
  tasksExpanded,
  onToggleTasksExpanded,
  width,
  onWidthChange,
  chatSplitPct,
  onChatSplitChange,
  activeSessionId,
  sessions,
  onSessionCreated,
  onSessionSelect,
  onNewSession,
  activeProduct,
  buildContextString,
  autoInjectContext,
  showContextChip,
  connected,
  messageCount,
  lastChatSnippet,
  tasks,
  taskView,
  gridSubView,
  filters,
  setTaskView,
  setGridSubView,
  setFilters,
  taskActivities,
  taskStreams,
  taskComments,
  updateTask,
  moveTask,
  deleteTask,
  fetchComments,
  addComment,
  products,
  productMetadata,
  createTask,
  syncProductFilter,
  onSyncProductFilterToggle,
  inlineBadgesEnabled: _inlineBadgesEnabled,
}: RightPanelProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const [selectedTask, setSelectedTask] = useState<PlannerTask | null>(null);
  const [showNewForm, setShowNewForm] = useState(false);
  const [historyOpen, setHistoryOpen] = useState(false);
  const [reauthStatus, setReauthStatus] = useState<'idle' | 'ok' | 'error'>('idle');

  // Unread tracking — snapshot message count when collapsing chat
  const [collapsedMsgCount, setCollapsedMsgCount] = useState(messageCount);
  useEffect(() => {
    if (!chatExpanded) setCollapsedMsgCount(messageCount);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [chatExpanded]);
  const unreadCount = chatExpanded ? 0 : Math.max(0, messageCount - collapsedMsgCount);

  const showChat = visiblePanels === 'both' || visiblePanels === 'chat';
  const showTasks = visiblePanels === 'both' || visiblePanels === 'tasks';
  const showBoth = visiblePanels === 'both';

  // Filter tasks
  const visibleTasks = syncProductFilter && activeProduct
    ? tasks.filter((t) => t.product === activeProduct)
    : tasks;
  const filteredTasks = visibleTasks.filter((t) => {
    if (filters.stage !== 'all' && t.stage !== filters.stage) return false;
    if (filters.priority !== 'all' && t.priority !== filters.priority) return false;
    if (filters.product !== 'all' && t.product !== filters.product) return false;
    return true;
  });
  const tasksByStage = (() => {
    const grouped: Record<TaskStage, PlannerTask[]> = {
      backlog: [], brainstorm: [], active: [], blocked: [], validation: [], done: [],
    };
    for (const t of filteredTasks) grouped[t.stage].push(t);
    return grouped;
  })();

  // ── Reauth handler (shared between rail and drawer) ──
  const handleReauth = useCallback(async () => {
    try {
      const res = await fetch('/api/reauth', { method: 'POST' });
      setReauthStatus(res.ok ? 'ok' : 'error');
    } catch {
      setReauthStatus('error');
    }
    setTimeout(() => setReauthStatus('idle'), 2000);
  }, []);

  // ── Stage counts for tasks rail ──
  const stageCounts = (() => {
    const counts: Partial<Record<TaskStage, number>> = {};
    for (const t of tasks) {
      counts[t.stage] = (counts[t.stage] || 0) + 1;
    }
    return counts;
  })();
  const hasActiveAgent = tasks.some((t) => t.stage === 'active' && t.agent_id);

  // Left-edge resize handle
  const onResizeStart = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    const startX = e.clientX;
    const startWidth = width;
    const onMove = (ev: MouseEvent) => {
      const delta = startX - ev.clientX;
      onWidthChange(startWidth + delta);
    };
    const onUp = () => {
      document.removeEventListener('mousemove', onMove);
      document.removeEventListener('mouseup', onUp);
    };
    document.addEventListener('mousemove', onMove);
    document.addEventListener('mouseup', onUp);
  }, [width, onWidthChange]);

  // ── Derived per-panel state ──
  const chatIsDrawer = showChat && chatExpanded;
  const chatIsRail = showChat && !chatExpanded;
  const tasksIsDrawer = showTasks && tasksExpanded;
  const tasksIsRail = showTasks && !tasksExpanded;
  const anyDrawer = chatIsDrawer || tasksIsDrawer;
  const bothDrawers = chatIsDrawer && tasksIsDrawer;
  const isSplit = drawerLayout === 'split';

  // Container width: full width if any drawer open, else sum of rails
  const railCount = (chatIsRail ? 1 : 0) + (tasksIsRail ? 1 : 0);
  const containerWidth = anyDrawer ? width : railCount * RAIL_WIDTH;

  // ── Chat rail (collapsed) ──
  const chatRail = (
    <div
      className="flex flex-col items-center shrink-0 h-full py-2 gap-1 cursor-pointer hover:bg-elevated/50 transition-colors border-l border-border-subtle"
      style={{ width: RAIL_WIDTH }}
      onClick={onToggleChatExpanded}
      title={lastChatSnippet || 'Open chat'}
    >
      <div className="relative mb-1">
        <span className="text-soul text-base leading-none">&#9670;</span>
        <span className={`absolute -bottom-0.5 -right-0.5 w-2 h-2 rounded-full border border-surface ${connected ? 'bg-green-400' : 'bg-red-400'}`} />
      </div>
      {unreadCount > 0 && (
        <span className="bg-soul text-deep text-[9px] font-bold font-display rounded-full min-w-4 h-4 flex items-center justify-center px-1 leading-none">
          {unreadCount > 9 ? '9+' : unreadCount}
        </span>
      )}
      <div className="flex-1 flex flex-col items-center gap-1.5 mt-2">
        <button type="button" className="w-7 h-7 flex items-center justify-center rounded text-fg-secondary hover:text-soul hover:bg-soul/10 transition-colors cursor-pointer" title="Quick: Status" onClick={(e) => { e.stopPropagation(); onToggleChatExpanded(); }}>
          <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"><path d="M8 1v4M8 11v4M1 8h4M11 8h4" /></svg>
        </button>
        <button type="button" className="w-7 h-7 flex items-center justify-center rounded text-fg-secondary hover:text-soul hover:bg-soul/10 transition-colors cursor-pointer" title="Quick: Search" onClick={(e) => { e.stopPropagation(); onToggleChatExpanded(); }}>
          <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"><circle cx="7" cy="7" r="4.5" /><path d="M10.5 10.5L14 14" /></svg>
        </button>
      </div>
      <div className="flex flex-col items-center gap-1 mt-auto">
        <button type="button" onClick={(e) => { e.stopPropagation(); handleReauth(); }} className={`w-7 h-7 flex items-center justify-center rounded transition-colors cursor-pointer ${reauthStatus === 'ok' ? 'text-green-400' : reauthStatus === 'error' ? 'text-red-400' : 'text-fg-secondary hover:text-fg hover:bg-elevated'}`} title="Refresh AI credentials">
          <svg width="12" height="12" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"><path d="M1 7a6 6 0 0111.196-3M13 7A6 6 0 011.804 10" /><path d="M1 1v3h3M13 13v-3h-3" /></svg>
        </button>
        <button type="button" onClick={(e) => { e.stopPropagation(); onToggleChatExpanded(); setHistoryOpen(true); }} className="w-7 h-7 flex items-center justify-center rounded text-fg-secondary hover:text-fg hover:bg-elevated transition-colors cursor-pointer" title="Chat history">
          <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"><circle cx="8" cy="8" r="6.5" /><path d="M8 4.5V8l2.5 2" /></svg>
        </button>
      </div>
    </div>
  );

  // ── Tasks rail (collapsed) ──
  const tasksRail = (
    <div
      className="flex flex-col items-center shrink-0 h-full py-2 gap-1 cursor-pointer hover:bg-elevated/50 transition-colors border-l border-border-subtle"
      style={{ width: RAIL_WIDTH }}
      onClick={onToggleTasksExpanded}
      title="Open tasks"
    >
      <div className="relative mb-1">
        <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" className="text-soul"><path d="M3 4l2 2 4-4" /><path d="M3 10l2 2 4-4" /></svg>
        {hasActiveAgent && <span className="absolute -top-0.5 -right-1.5 w-2 h-2 rounded-full bg-stage-active animate-pulse" />}
      </div>
      <div className="flex flex-col items-center gap-1 mt-2">
        {(['active', 'blocked', 'backlog', 'validation', 'done'] as TaskStage[]).map((stage) => {
          const count = stageCounts[stage] || 0;
          if (count === 0) return null;
          return (
            <div key={stage} className="flex items-center gap-1" title={`${stage}: ${count}`}>
              <span className={`w-2 h-2 rounded-full ${STAGE_DOT[stage]}`} />
              <span className="text-[10px] font-display text-fg-secondary leading-none">{count}</span>
            </div>
          );
        })}
      </div>
      <div className="mt-auto text-[10px] font-display text-fg-muted">{tasks.length}</div>
    </div>
  );

  // ── Collapse arrow icon ──
  const collapseIcon = (
    <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"><path d="M10 3l5 5-5 5" /><path d="M15 8H5" /></svg>
  );

  // ── Chat drawer (expanded) ──
  const chatDrawer = (
    <div className="panel-container flex flex-col min-h-0 flex-1 overflow-hidden">
      <div className="flex items-center px-3 gap-1.5 h-10 border-b border-border-subtle shrink-0">
        <span className="flex items-center gap-1.5 px-1 h-full text-xs font-display font-semibold text-soul">
          <span className="text-base leading-none">&#9670;</span> Chat
        </span>
        <span className={`w-2 h-2 rounded-full ${connected ? 'bg-green-400' : 'bg-red-400'}`} title={connected ? 'Connected' : 'Disconnected'} />
        <div className="flex-1" />
        <button type="button" onClick={handleReauth} className={`h-6 flex items-center gap-1 px-1.5 rounded transition-colors cursor-pointer ${reauthStatus === 'ok' ? 'text-green-400' : reauthStatus === 'error' ? 'text-red-400' : 'text-fg-secondary hover:text-fg hover:bg-elevated'}`} title="Refresh AI credentials">
          <svg width="12" height="12" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"><path d="M1 7a6 6 0 0111.196-3M13 7A6 6 0 011.804 10" /><path d="M1 1v3h3M13 13v-3h-3" /></svg>
          <span className="rp-label-action text-[10px] font-mono">Auth</span>
        </button>
        <button type="button" onClick={() => setHistoryOpen((o) => !o)} className={`h-6 flex items-center gap-1 px-1.5 rounded transition-colors cursor-pointer ${historyOpen ? 'bg-soul/15 text-soul' : 'hover:bg-elevated text-fg-secondary hover:text-fg'}`} title="Chat history">
          <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"><circle cx="8" cy="8" r="6.5" /><path d="M8 4.5V8l2.5 2" /></svg>
          <span className="rp-label-action text-[10px] font-mono">History</span>
        </button>
        <button type="button" onClick={() => { onNewSession(); setTimeout(() => { const ta = document.querySelector<HTMLTextAreaElement>('textarea[placeholder*="Message"]'); ta?.focus(); }, 100); }} className="h-6 flex items-center gap-1 px-1.5 rounded bg-soul/10 hover:bg-soul/20 text-soul transition-colors cursor-pointer" title="New chat (Ctrl+Shift+N)">
          <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round"><path d="M8 3v10M3 8h10" /></svg>
          <span className="rp-label-primary text-[10px] font-mono">New</span>
        </button>
        <button type="button" onClick={onToggleChatExpanded} className="h-6 flex items-center gap-1 px-1.5 rounded text-fg-secondary hover:text-fg hover:bg-elevated transition-colors cursor-pointer" title="Collapse chat">
          {collapseIcon}
          <span className="rp-label-action text-[10px] font-mono">Close</span>
        </button>
      </div>
      <div className="flex-1 min-h-0 overflow-hidden relative">
        <ChatView activeSessionId={activeSessionId} onSessionCreated={onSessionCreated} activeProduct={activeProduct} buildContextString={buildContextString} autoInjectContext={autoInjectContext} showContextChip={showContextChip} />
        {historyOpen && (
          <SessionDrawer sessions={sessions} activeSessionId={activeSessionId} onSelect={(id) => { onSessionSelect(id); setHistoryOpen(false); }} onClose={() => setHistoryOpen(false)} />
        )}
      </div>
    </div>
  );

  // ── Tasks drawer (expanded) ──
  const tasksDrawer = (
    <div className="panel-container flex flex-col min-h-0 flex-1 overflow-hidden">
      <div className="border-b border-border-subtle shrink-0">
        {/* Row 1: Title + Views + Actions */}
        <div className="flex items-center px-2 gap-1.5 h-9">
          <span className="flex items-center gap-1 px-1 text-xs font-display font-semibold text-soul shrink-0">
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"><path d="M3 4l2 2 4-4" /><path d="M3 10l2 2 4-4" /></svg>
            Tasks
          </span>
          <div className="w-px h-4 bg-border-subtle shrink-0" />
          {([
            { view: 'list' as TaskView, label: 'List', icon: <svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"><path d="M3 4h10M3 8h10M3 12h10" /></svg> },
            { view: 'kanban' as TaskView, label: 'Board', icon: <svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"><path d="M3 3v10M8 3v7M13 3v10" /></svg> },
            { view: 'grid' as TaskView, label: 'Grid', icon: <svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"><rect x="2" y="2" width="5" height="5" rx="1" /><rect x="9" y="2" width="5" height="5" rx="1" /><rect x="2" y="9" width="5" height="5" rx="1" /><rect x="9" y="9" width="5" height="5" rx="1" /></svg> },
            { view: 'table' as TaskView, label: 'Table', icon: <svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"><rect x="2" y="2" width="12" height="12" rx="1" /><path d="M2 5.5h12" /><path d="M6 5.5v8.5" /></svg> },
          ] as { view: TaskView; label: string; icon: React.ReactNode }[]).map(({ view, label, icon }) => (
            <button key={view} type="button" onClick={() => setTaskView(view)} title={view} className={`h-6 flex items-center gap-1 px-1.5 rounded cursor-pointer transition-colors shrink-0 ${taskView === view ? 'bg-overlay text-fg' : 'text-fg-secondary hover:text-fg hover:bg-elevated'}`}>
              {icon}
              <span className="rp-label-view text-[10px] font-mono">{label}</span>
            </button>
          ))}
          <div className="flex-1" />
          <button type="button" onClick={() => setShowNewForm(true)} className="h-6 flex items-center gap-1 px-1.5 bg-soul/10 hover:bg-soul/20 text-soul rounded font-display font-semibold transition-colors cursor-pointer shrink-0" title="New task">
            <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round"><path d="M8 3v10M3 8h10" /></svg>
            <span className="rp-label-primary text-[10px] font-mono">New</span>
          </button>
          <button type="button" onClick={onToggleTasksExpanded} className="h-6 flex items-center gap-1 px-1.5 rounded text-fg-secondary hover:text-fg hover:bg-elevated transition-colors cursor-pointer shrink-0" title="Collapse tasks">
            {collapseIcon}
            <span className="rp-label-action text-[10px] font-mono">Close</span>
          </button>
        </div>
        {/* Row 2: Filters + Sync */}
        <div className="flex items-center px-2 gap-2 h-9 border-t border-border-subtle/50">
          <div className="flex items-center gap-1 shrink-0">
            <span className="rp-label-filter text-[9px] font-mono text-fg-muted">Stage:</span>
            <select className="soul-select text-[10px]" value={filters.stage} onChange={(e) => setFilters({ stage: e.target.value as TaskStage | 'all' })} title="Filter by stage">
              <option value="all">All stages</option><option value="backlog">Backlog</option><option value="brainstorm">Brainstorm</option><option value="active">Active</option><option value="blocked">Blocked</option><option value="validation">Review</option><option value="done">Done</option>
            </select>
          </div>
          <div className="flex items-center gap-1 shrink-0">
            <span className="rp-label-filter text-[9px] font-mono text-fg-muted">Priority:</span>
            <select className="soul-select text-[10px]" value={filters.priority === 'all' ? 'all' : String(filters.priority)} onChange={(e) => { const v = e.target.value; setFilters({ priority: v === 'all' ? 'all' : Number(v) }); }} title="Filter by priority">
              <option value="all">All priority</option><option value="3">Critical</option><option value="2">High</option><option value="1">Normal</option><option value="0">Low</option>
            </select>
          </div>
          <div className="flex items-center gap-1 shrink-0">
            <span className="rp-label-filter text-[9px] font-mono text-fg-muted">Project:</span>
            <select className="soul-select text-[10px]" value={filters.product ?? 'all'} onChange={(e) => setFilters({ product: e.target.value === 'all' ? 'all' : e.target.value })} title="Filter by project">
              <option value="all">All projects</option>
              {products.map((p) => <option key={p} value={p}>{productMetadata?.get(p)?.label ?? (p.charAt(0).toUpperCase() + p.slice(1))}</option>)}
            </select>
          </div>
          <div className="w-px h-4 bg-border-subtle shrink-0" />
          <button
            type="button"
            onClick={onSyncProductFilterToggle}
            title={syncProductFilter ? 'Sync ON — showing active product tasks' : 'Sync OFF — showing all tasks'}
            className={`h-5 flex items-center gap-1 px-1.5 rounded cursor-pointer transition-colors shrink-0 ${syncProductFilter ? 'bg-soul/15 text-soul' : 'text-fg-secondary hover:text-fg hover:bg-elevated'}`}
          >
            <svg width="11" height="11" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
              {syncProductFilter ? (
                <><path d="M8 2v12" /><path d="M3 5l3-3 3 3" /><path d="M7 11l3 3 3-3" /></>
              ) : (
                <><path d="M4 2v12" /><path d="M1 5l3-3 3 3" /><path d="M12 14V2" /><path d="M9 11l3 3 3-3" /></>
              )}
            </svg>
            <span className="rp-label-action text-[10px] font-mono">Sync</span>
          </button>
        </div>
      </div>
      <div className="flex-1 min-h-0 overflow-hidden">
        <TaskContent taskView={taskView} filteredTasks={filteredTasks} tasksByStage={tasksByStage} gridSubView={gridSubView} onGridSubViewChange={setGridSubView} onTaskClick={setSelectedTask} onClearFilters={() => setFilters({ stage: 'all', priority: 'all' })} taskActivities={taskActivities} />
      </div>
    </div>
  );

  // Suppress unused variable warnings for showBoth (used for layout logic)
  void showBoth;

  // ── Render: mix of rails and drawers ──
  return (
    <>
      <div
        ref={containerRef}
        className="flex h-full bg-surface shrink-0"
        style={{ width: containerWidth }}
      >
        {/* Resize handle only when any drawer is open */}
        {anyDrawer && (
          <div onMouseDown={onResizeStart} className="w-1 h-full cursor-col-resize shrink-0 group relative z-10 border-l border-border-subtle" title="Drag to resize">
            <div className="absolute inset-y-0 -inset-x-1 group-hover:bg-soul/30 transition-colors" />
          </div>
        )}

        {/* Chat: rail or drawer */}
        {chatIsRail && chatRail}
        {chatIsDrawer && !tasksIsDrawer && (
          <div className="flex flex-1 min-h-0 min-w-0">{chatDrawer}</div>
        )}

        {/* Both drawers: split/independent with divider */}
        {bothDrawers && (
          <div className={`flex ${isSplit ? 'flex-col' : ''} flex-1 min-w-0 min-h-0`}>
            <div className="flex min-h-0 min-w-0" style={isSplit ? { height: `${chatSplitPct}%`, flexShrink: 0 } : { width: `${chatSplitPct}%`, flexShrink: 0 }}>
              {chatDrawer}
            </div>
            <SplitDivider direction={isSplit ? 'horizontal' : 'vertical'} onSplitChange={onChatSplitChange} containerRef={containerRef} />
            <div className="flex flex-1 min-h-0 min-w-0 bg-deep">{tasksDrawer}</div>
          </div>
        )}

        {/* Tasks: rail or drawer */}
        {tasksIsDrawer && !chatIsDrawer && (
          <div className="flex flex-1 min-h-0 min-w-0">{tasksDrawer}</div>
        )}
        {tasksIsRail && tasksRail}
      </div>

      {selectedTask && (
        <TaskDetail
          task={tasks.find((t) => t.id === selectedTask.id) ?? selectedTask}
          onClose={() => setSelectedTask(null)}
          onMove={async (id, stage, comment) => { await moveTask(id, stage, comment); setSelectedTask(null); }}
          onUpdate={async (id, updates) => { const u = await updateTask(id, updates); setSelectedTask(u); return u; }}
          onDelete={async (id) => { await deleteTask(id); setSelectedTask(null); }}
          activities={taskActivities[selectedTask.id] || []}
          streamContent={taskStreams[selectedTask.id] || ''}
          products={products}
          productMetadata={productMetadata}
          comments={taskComments[selectedTask.id]}
          onFetchComments={fetchComments}
          onAddComment={addComment}
        />
      )}

      {showNewForm && (
        <NewTaskForm
          onClose={() => setShowNewForm(false)}
          onCreate={async (title, description, priority, product) => { await createTask(title, description, priority, product); setShowNewForm(false); }}
        />
      )}
    </>
  );
}
