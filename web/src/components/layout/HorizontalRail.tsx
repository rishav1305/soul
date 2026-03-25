import { useRef, useCallback, useEffect, useState } from 'react';
import type {
  PlannerTask,
  TaskStage,
  TaskView,
  GridSubView,
  TaskFilters,
  PlannerActivity,
  TaskComment,
  HorizontalRailTab,
  DrawerLayout,
  ChatSession,
  SendOptions,
} from '../../lib/types.ts';
import { useChat } from '../../hooks/useChat.ts';
import ChatView from '../chat/ChatView.tsx';
import SessionDrawer from '../chat/SessionDrawer.tsx';
import TaskContent from '../planner/TaskContent.tsx';
import TaskDetail from '../planner/TaskDetail.tsx';
import NewTaskForm from '../planner/NewTaskForm.tsx';

interface ModelInfo {
  id: string;
  name: string;
  description: string;
}

const CHAT_TYPES = [
  { value: 'Chat', label: 'Chat' },
  { value: 'Code', label: 'Code' },
  { value: 'Architect', label: 'Arch' },
  { value: 'Debug', label: 'Debug' },
  { value: 'Review', label: 'Review' },
] as const;

// ── Stage colors for pills ──
const STAGE_COLORS: Record<TaskStage, string> = {
  backlog: 'bg-stage-backlog/15 text-stage-backlog',
  brainstorm: 'bg-stage-brainstorm/15 text-stage-brainstorm',
  active: 'bg-stage-active/15 text-stage-active',
  blocked: 'bg-stage-blocked/15 text-stage-blocked',
  validation: 'bg-stage-validation/15 text-stage-validation',
  done: 'bg-stage-done/15 text-stage-done',
};

// ── Split divider (vertical or horizontal) ──
interface SplitDividerProps {
  onSplitChange: (pct: number) => void;
  containerRef: React.RefObject<HTMLDivElement | null>;
  direction?: 'vertical' | 'horizontal';
}

function SplitDivider({ onSplitChange, containerRef, direction = 'vertical' }: SplitDividerProps) {
  const onMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    const container = containerRef.current;
    if (!container) return;

    const onMove = (me: MouseEvent) => {
      if (!container) return;
      const r = container.getBoundingClientRect();
      const pct = direction === 'vertical'
        ? ((me.clientX - r.left) / r.width) * 100
        : ((me.clientY - r.top) / r.height) * 100;
      onSplitChange(Math.round(Math.min(80, Math.max(30, pct))));
    };

    const onUp = () => {
      document.removeEventListener('mousemove', onMove);
      document.removeEventListener('mouseup', onUp);
    };

    document.addEventListener('mousemove', onMove);
    document.addEventListener('mouseup', onUp);
  }, [onSplitChange, containerRef, direction]);

  const isVert = direction === 'vertical';

  return (
    <div
      onMouseDown={onMouseDown}
      className={`${isVert ? 'w-1 self-stretch cursor-col-resize' : 'h-1 w-full cursor-row-resize'} shrink-0 group relative z-10`}
      title="Drag to resize panels"
    >
      <div className={`absolute ${isVert ? 'inset-y-0 -inset-x-1' : 'inset-x-0 -inset-y-1'} group-hover:bg-soul/30 transition-colors rounded-sm`} />
    </div>
  );
}

interface HorizontalRailProps {
  // Layout state
  expanded: boolean;
  heightVh: number;
  tab: HorizontalRailTab;
  chatSplitPct: number;
  drawerLayout: DrawerLayout;
  position: 'bottom' | 'top';
  onToggleExpand: () => void;
  onSetTab: (tab: HorizontalRailTab) => void;
  onHeightChange: (vh: number) => void;
  onChatSplitChange: (pct: number) => void;
  // Chat
  activeSessionId: number | null;
  sessions: ChatSession[];
  onSessionCreated?: (id: number) => void;
  onSessionSelect: (id: number) => void;
  onNewSession: () => void;
  runningSessions?: ChatSession[];
  unreadSessions?: ChatSession[];
  lastChatSnippet?: string;
  // Tasks
  tasks: PlannerTask[];
  activeProduct: string | null;
  taskActivities: Record<number, PlannerActivity[]>;
  taskStreams: Record<number, string>;
  taskComments: Record<number, TaskComment[]>;
  updateTask: (id: number, updates: Partial<PlannerTask>) => Promise<PlannerTask>;
  moveTask: (id: number, stage: TaskStage, comment: string) => Promise<PlannerTask>;
  deleteTask: (id: number) => Promise<void>;
  fetchComments: (id: number) => Promise<TaskComment[]>;
  addComment: (id: number, body: string) => Promise<TaskComment>;
  products: string[];
  createTask: (title: string, description: string, priority: number, product: string) => Promise<PlannerTask>;
  // Task view/filter state
  taskView: TaskView;
  gridSubView: GridSubView;
  filters: TaskFilters;
  setTaskView: (v: TaskView) => void;
  setGridSubView: (v: GridSubView) => void;
  setFilters: (partial: Partial<TaskFilters>) => void;
  // Sync filter
  syncProductFilter: boolean;
  onSyncProductFilterToggle: () => void;
  // Context injection
  buildContextString?: () => string;
  autoInjectContext?: boolean;
  showContextChip?: boolean;
  inlineBadgesEnabled?: boolean;
  visiblePanels?: 'both' | 'chat' | 'tasks';
}

export default function HorizontalRail({
  expanded,
  heightVh,
  tab,
  chatSplitPct,
  drawerLayout,
  position,
  onToggleExpand,
  onSetTab,
  onHeightChange,
  onChatSplitChange,
  activeSessionId,
  sessions,
  onSessionCreated,
  onSessionSelect,
  onNewSession,
  runningSessions = [],
  unreadSessions = [],
  lastChatSnippet,
  tasks,
  activeProduct,
  taskActivities,
  taskStreams,
  taskComments,
  updateTask,
  moveTask,
  deleteTask,
  fetchComments,
  addComment,
  products,
  createTask,
  taskView,
  gridSubView,
  filters,
  setTaskView,
  setGridSubView,
  setFilters,
  syncProductFilter,
  onSyncProductFilterToggle,
  buildContextString,
  autoInjectContext,
  showContextChip,
  inlineBadgesEnabled: _inlineBadgesEnabled,
  visiblePanels = 'both',
}: HorizontalRailProps) {
  const dragRef = useRef<{ startY: number; startVh: number } | null>(null);
  const railContainerRef = useRef<HTMLDivElement | null>(null);
  const [selectedTask, setSelectedTask] = useState<PlannerTask | null>(null);
  const [showNewForm, setShowNewForm] = useState(false);
  const [historyOpen, setHistoryOpen] = useState(false);
  const [reauthStatus, setReauthStatus] = useState<'idle' | 'ok' | 'error'>('idle');

  // Independent mode: track which panels are expanded
  const [chatOpen, setChatOpen] = useState(false);
  const [tasksOpen, setTasksOpen] = useState(false);

  const showChat = visiblePanels === 'both' || visiblePanels === 'chat';
  const showTasks = visiblePanels === 'both' || visiblePanels === 'tasks';
  const singlePanel = visiblePanels !== 'both';

  // Sync with parent expanded state (e.g. when parent collapses all)
  useEffect(() => {
    if (!expanded) {
      setChatOpen(false);
      setTasksOpen(false);
    }
  }, [expanded]);

  // Single-panel mode: sync local open state with parent expanded
  useEffect(() => {
    if (visiblePanels === 'chat') {
      setChatOpen(expanded);
      setTasksOpen(false);
    } else if (visiblePanels === 'tasks') {
      setTasksOpen(expanded);
      setChatOpen(false);
    }
  }, [expanded, visiblePanels]);

  // Inline rail chat
  const { sendMessage, isStreaming } = useChat();
  const railInputRef = useRef<HTMLInputElement>(null);
  const [railInput, setRailInput] = useState('');
  const [railModel, setRailModel] = useState('');
  const [railModels, setRailModels] = useState<ModelInfo[]>([]);
  const [railChatType, setRailChatType] = useState('Chat');
  const [isListening, setIsListening] = useState(false);
  const recognitionRef = useRef<any>(null);
  const speechSupported = typeof window !== 'undefined' && ('webkitSpeechRecognition' in window || 'SpeechRecognition' in window);

  // Fetch models on mount
  useEffect(() => {
    fetch('/api/models')
      .then((r) => r.json())
      .then((data: ModelInfo[]) => {
        setRailModels(data);
        if (data.length > 0) setRailModel(data[0]!.id);
      })
      .catch(() => {});
  }, []);

  const startListening = useCallback(() => {
    if (!speechSupported) return;
    const SR = (window as any).webkitSpeechRecognition || (window as any).SpeechRecognition;
    const recognition = new SR();
    recognition.continuous = false;
    recognition.interimResults = false;
    recognition.lang = 'en-US';
    recognition.onresult = (event: any) => {
      const transcript = event.results[0]?.[0]?.transcript ?? '';
      if (transcript) setRailInput((prev) => prev + transcript);
    };
    recognition.onend = () => setIsListening(false);
    recognition.onerror = () => setIsListening(false);
    recognitionRef.current = recognition;
    recognition.start();
    setIsListening(true);
  }, [speechSupported]);

  const stopListening = useCallback(() => {
    if (recognitionRef.current) { recognitionRef.current.stop(); recognitionRef.current = null; }
    setIsListening(false);
  }, []);

  // Filter tasks — respect sync filter toggle
  const visibleTasks = syncProductFilter && activeProduct
    ? tasks.filter((t) => t.product === activeProduct)
    : tasks;

  // Apply stage/priority filters
  const filteredTasks = visibleTasks.filter((t) => {
    if (filters.stage !== 'all' && t.stage !== filters.stage) return false;
    if (filters.priority !== 'all' && t.priority !== filters.priority) return false;
    return true;
  });

  // Group filtered tasks by stage (for kanban)
  const tasksByStage = (() => {
    const grouped: Record<TaskStage, PlannerTask[]> = {
      backlog: [], brainstorm: [], active: [], blocked: [], validation: [], done: [],
    };
    for (const t of filteredTasks) grouped[t.stage].push(t);
    return grouped;
  })();

  // Stage counts for rail bar (unfiltered visible tasks)
  const stageCounts: { stage: TaskStage; count: number }[] = [];
  const stageOrder: TaskStage[] = ['active', 'blocked', 'brainstorm', 'validation', 'backlog', 'done'];
  for (const stage of stageOrder) {
    const count = visibleTasks.filter((t) => t.stage === stage).length;
    if (count > 0) stageCounts.push({ stage, count });
  }

  // ── Drag-to-resize handle ──
  const onDragStart = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    dragRef.current = { startY: e.clientY, startVh: heightVh };

    const onMove = (me: MouseEvent) => {
      if (!dragRef.current) return;
      const delta = dragRef.current.startY - me.clientY;
      const deltaVh = (delta / window.innerHeight) * 100;
      const newVh = dragRef.current.startVh + (position === 'bottom' ? deltaVh : -deltaVh);
      onHeightChange(Math.min(60, Math.max(20, Math.round(newVh))));
    };

    const onUp = () => {
      dragRef.current = null;
      document.removeEventListener('mousemove', onMove);
      document.removeEventListener('mouseup', onUp);
    };

    document.addEventListener('mousemove', onMove);
    document.addEventListener('mouseup', onUp);
  }, [heightVh, onHeightChange, position]);

  // Keep selectedTask in sync with live task data
  useEffect(() => {
    if (!selectedTask) return;
    const updated = tasks.find((t) => t.id === selectedTask.id);
    if (updated && updated !== selectedTask) setSelectedTask(updated);
  }, [tasks, selectedTask]);

  const isIndependent = drawerLayout === 'independent';

  // ── Chat rail content (reused in both modes) ──
  const chatRailContent = (
    <div className="flex items-center px-2 gap-1.5 min-w-0 flex-1">
        {/* Model selector */}
        {railModels.length > 0 && (
          <select
            value={railModel}
            onChange={(e) => setRailModel(e.target.value)}
            className="soul-select text-[10px] h-7 px-1 rounded shrink-0 bg-elevated border border-border-default cursor-pointer max-w-[90px]"
            title="Model"
          >
            {railModels.map((m) => (
              <option key={m.id} value={m.id}>◆ {m.name}</option>
            ))}
          </select>
        )}

        {/* Chat type selector */}
        <select
          value={railChatType}
          onChange={(e) => setRailChatType(e.target.value)}
          className="soul-select text-[10px] h-7 px-1 rounded shrink-0 bg-elevated border border-border-default cursor-pointer"
          title="Chat mode"
        >
          {CHAT_TYPES.map((t) => (
            <option key={t.value} value={t.value}>{t.label}</option>
          ))}
        </select>

        {/* Input */}
        <form
          className="flex-1 flex items-center min-w-0 bg-elevated rounded-lg px-2 h-7 border border-border-default"
          onSubmit={(e) => {
            e.preventDefault();
            if (railInput.trim() && !isStreaming) {
              const opts: SendOptions = {};
              if (railModel) opts.model = railModel;
              if (railChatType !== 'Chat') opts.chatType = railChatType.toLowerCase();
              sendMessage(railInput.trim(), Object.keys(opts).length > 0 ? opts : undefined);
              setRailInput('');
            }
          }}
        >
          <input
            ref={railInputRef}
            type="text"
            value={railInput}
            onChange={(e) => setRailInput(e.target.value)}
            placeholder={lastChatSnippet ?? 'Message Soul...'}
            disabled={isStreaming}
            className="flex-1 bg-transparent text-xs text-fg placeholder:text-fg-muted outline-none min-w-0 disabled:opacity-50"
          />
          {/* Send or Voice button */}
          {railInput.trim() ? (
            <button
              type="submit"
              disabled={isStreaming}
              className="w-6 h-6 flex items-center justify-center rounded-full bg-soul text-deep hover:bg-soul/85 transition-colors cursor-pointer shrink-0 disabled:opacity-50"
              title="Send"
            >
              <svg width="11" height="11" viewBox="0 0 16 16" fill="currentColor">
                <path d="M8 3l-1 1 3.3 3.3H3v1.4h7.3L7 12l1 1 5-5-5-5z" />
              </svg>
            </button>
          ) : speechSupported ? (
            <button
              type="button"
              onClick={isListening ? stopListening : startListening}
              className={`w-6 h-6 flex items-center justify-center rounded-full transition-colors cursor-pointer shrink-0 ${
                isListening
                  ? 'bg-stage-blocked text-white animate-pulse'
                  : 'text-fg-secondary hover:text-fg hover:bg-elevated'
              }`}
              title={isListening ? 'Stop listening' : 'Voice input'}
            >
              <svg width="11" height="11" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                <rect x="5" y="1" width="6" height="8" rx="3" />
                <path d="M3 7v1a5 5 0 0 0 10 0V7" />
                <path d="M8 13v2" />
              </svg>
            </button>
          ) : null}
        </form>
    </div>
  );

  // ── Task rail content ──
  const taskRailContent = (
    <div className="flex items-center px-2 gap-1.5 min-w-0 overflow-x-auto flex-1 justify-end">
      {stageCounts.map(({ stage, count }) => (
        <span
          key={stage}
          className={`text-[10px] px-1.5 py-0.5 rounded-full font-semibold whitespace-nowrap shrink-0 ${STAGE_COLORS[stage]}`}
        >
          {count} {stage}
        </span>
      ))}
      {stageCounts.length === 0 && (
        <span className="text-[10px] text-fg-muted shrink-0">No tasks</span>
      )}
      <div className="w-px h-4 bg-border-subtle shrink-0" />
      <button
        type="button"
        onClick={() => setShowNewForm(true)}
        className="w-6 h-6 flex items-center justify-center rounded bg-soul/10 text-soul hover:bg-soul/20 transition-colors cursor-pointer shrink-0"
        title="New task"
      >
        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
          <path d="M6 2v8M2 6h8" />
        </svg>
      </button>
      <button
        type="button"
        onClick={onSyncProductFilterToggle}
        title={syncProductFilter ? 'Sync ON — showing active product tasks' : 'Sync OFF — showing all tasks'}
        className={`w-6 h-6 flex items-center justify-center rounded cursor-pointer transition-colors shrink-0 ${
          syncProductFilter ? 'bg-soul/15 text-soul' : 'text-fg-secondary hover:text-fg hover:bg-elevated'
        }`}
      >
        <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
          {syncProductFilter ? (
            <><path d="M8 2v12" /><path d="M3 5l3-3 3 3" /><path d="M7 11l3 3 3-3" /></>
          ) : (
            <><path d="M4 2v12" /><path d="M1 5l3-3 3 3" /><path d="M12 14V2" /><path d="M9 11l3 3 3-3" /></>
          )}
        </svg>
      </button>
    </div>
  );

  // ── Expand/collapse button ──
  const expandButton = (
    <button
      type="button"
      onClick={onToggleExpand}
      className="w-8 h-8 flex items-center justify-center rounded-full bg-soul text-deep hover:bg-soul/85 transition-colors cursor-pointer shrink-0 self-center mx-1 shadow-sm"
      title={expanded ? 'Collapse drawer' : 'Expand drawer'}
    >
      <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"
        style={{ transform: expanded ? 'rotate(180deg)' : 'rotate(0deg)' }}
      >
        <path d="M3 9l4-4 4 4" />
      </svg>
    </button>
  );

  // ── Rail bar (collapsed) ──
  const railBorder = {
    borderTopWidth: position === 'bottom' ? 1 : 0,
    borderBottomWidth: position === 'top' ? 1 : 0,
    borderColor: 'var(--color-border-subtle)',
    borderStyle: 'solid' as const,
  };

  const toggleChat = useCallback(() => {
    if (singlePanel) { onToggleExpand(); return; }
    const next = !chatOpen;
    setChatOpen(next);
    if (next && !expanded) onSetTab('chat');
    if (!next && !tasksOpen) onToggleExpand();
  }, [singlePanel, chatOpen, tasksOpen, expanded, onSetTab, onToggleExpand]);

  const toggleTasks = useCallback(() => {
    if (singlePanel) { onToggleExpand(); return; }
    const next = !tasksOpen;
    setTasksOpen(next);
    if (next && !expanded) onSetTab('tasks');
    if (!next && !chatOpen) onToggleExpand();
  }, [singlePanel, tasksOpen, chatOpen, expanded, onSetTab, onToggleExpand]);

  const upChevron = <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M3 9l4-4 4 4" /></svg>;
  const downChevron = <svg width="12" height="12" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M3 5l4 4 4-4" /></svg>;

  const railBar = isIndependent ? (
    <div data-testid="horizontal-rail" className="flex flex-col bg-surface select-none shrink-0" style={railBorder}>
      {showChat && (
        <div className={`flex items-stretch h-12 ${showTasks ? 'border-b border-border-subtle' : ''}`}>
          {chatRailContent}
          <button type="button" onClick={toggleChat} className="w-8 h-8 flex items-center justify-center rounded-full bg-soul text-deep hover:bg-soul/85 transition-colors cursor-pointer shrink-0 self-center mx-1 shadow-sm" title="Expand chat">
            {upChevron}
          </button>
        </div>
      )}
      {showTasks && (
        <div className="flex items-stretch h-12">
          {taskRailContent}
          <button type="button" onClick={toggleTasks} className="w-8 h-8 flex items-center justify-center rounded-full bg-soul text-deep hover:bg-soul/85 transition-colors cursor-pointer shrink-0 self-center mx-1 shadow-sm" title="Expand tasks">
            {upChevron}
          </button>
        </div>
      )}
    </div>
  ) : (
    <div ref={railContainerRef} data-testid="horizontal-rail" className="flex items-stretch h-12 bg-surface select-none shrink-0" style={railBorder}>
      {chatRailContent}
      {expandButton}
      {taskRailContent}
    </div>
  );

  // ── Expanded panel ──
  const expandedPanel = (
    <div
      ref={railContainerRef}
      data-testid="horizontal-rail"
      className="flex flex-col bg-surface shrink-0"
      style={{
        height: `${heightVh}vh`,
        borderTopWidth: position === 'bottom' ? 1 : 0,
        borderBottomWidth: position === 'top' ? 1 : 0,
        borderColor: 'var(--color-border-subtle)',
        borderStyle: 'solid',
      }}
    >
      {/* Drag handle — on top edge for bottom rail */}
      {position === 'bottom' && (
        <div
          onMouseDown={onDragStart}
          className="h-1 w-full cursor-row-resize shrink-0 hover:bg-soul/30 transition-colors"
          title="Drag to resize"
        />
      )}

      {/* Tab bar — side-by-side (split) or stacked (independent) */}
      <div className={`flex ${isIndependent ? 'flex-col' : ''} items-stretch border-b border-border-subtle shrink-0 ${isIndependent ? '' : 'h-10'}`}>
        {/* Chat tab header */}
        <div
          className={`flex items-center px-4 gap-0 min-w-0 ${isIndependent ? 'h-10 border-b border-border-subtle' : ''}`}
          style={isIndependent ? undefined : { width: `${chatSplitPct}%` }}
        >
          <button
            type="button"
            onClick={() => onSetTab('chat')}
            className={`flex items-center gap-1.5 px-3 h-full text-xs font-display font-semibold border-b-2 transition-colors cursor-pointer ${
              tab === 'chat'
                ? 'text-soul border-soul'
                : 'text-fg-secondary border-transparent hover:text-fg'
            }`}
          >
            <span className="text-base leading-none">&#9670;</span>
            Chat
          </button>
          <div className="flex-1" />
          {/* Refresh OAuth button */}
          <button
            type="button"
            onClick={async () => {
              try {
                const res = await fetch('/api/reauth', { method: 'POST' });
                setReauthStatus(res.ok ? 'ok' : 'error');
              } catch {
                setReauthStatus('error');
              }
              setTimeout(() => setReauthStatus('idle'), 2000);
            }}
            className={`w-7 h-7 flex items-center justify-center rounded transition-colors cursor-pointer ${
              reauthStatus === 'ok' ? 'text-green-400' : reauthStatus === 'error' ? 'text-red-400' : 'text-fg-secondary hover:text-fg hover:bg-elevated'
            }`}
            title="Refresh AI credentials"
          >
            <svg width="13" height="13" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
              <path d="M1 7a6 6 0 0111.196-3M13 7A6 6 0 011.804 10" />
              <path d="M1 1v3h3M13 13v-3h-3" />
            </svg>
          </button>
          {/* Running sessions badge */}
          {runningSessions.length > 0 && (
            <button
              type="button"
              onClick={() => setHistoryOpen((o) => !o)}
              className="h-6 px-1.5 flex items-center gap-1 rounded bg-stage-active/15 text-stage-active text-[10px] font-display font-semibold transition-colors cursor-pointer hover:bg-stage-active/25"
              title={`${runningSessions.length} running session${runningSessions.length > 1 ? 's' : ''}`}
            >
              <span className="relative flex h-1.5 w-1.5">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-stage-active opacity-75" />
                <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-stage-active" />
              </span>
              {runningSessions.length}
            </button>
          )}
          {/* Unread sessions badge */}
          {unreadSessions.length > 0 && (
            <button
              type="button"
              onClick={() => setHistoryOpen((o) => !o)}
              className="h-6 px-1.5 flex items-center gap-1 rounded bg-stage-done/15 text-stage-done text-[10px] font-display font-semibold transition-colors cursor-pointer hover:bg-stage-done/25"
              title={`${unreadSessions.length} unread session${unreadSessions.length > 1 ? 's' : ''}`}
            >
              <span className="w-1.5 h-1.5 rounded-full bg-stage-done" />
              {unreadSessions.length}
            </button>
          )}
          {/* History button */}
          <button
            type="button"
            onClick={() => setHistoryOpen((o) => !o)}
            className={`w-7 h-7 flex items-center justify-center rounded transition-colors cursor-pointer ${
              historyOpen
                ? 'bg-soul/15 text-soul'
                : 'hover:bg-elevated text-fg-secondary hover:text-fg'
            }`}
            title="Chat history"
          >
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="8" cy="8" r="6.5" />
              <path d="M8 4.5V8l2.5 2" />
            </svg>
          </button>
          {/* New chat button */}
          <button
            type="button"
            onClick={() => { onNewSession(); setTimeout(() => { const ta = document.querySelector<HTMLTextAreaElement>('textarea[placeholder*="Message"]'); ta?.focus(); }, 100); }}
            className="w-7 h-7 flex items-center justify-center rounded bg-soul/10 hover:bg-soul/20 text-soul transition-colors cursor-pointer"
            title="New chat (Ctrl+Shift+N)"
          >
            <svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round"><path d="M8 3v10M3 8h10" /></svg>
          </button>
          {/* Collapse button — gold circle */}
          <button
            type="button"
            onClick={() => { setHistoryOpen(false); onToggleExpand(); }}
            className="w-7 h-7 flex items-center justify-center rounded-full bg-soul text-deep hover:bg-soul/85 transition-colors cursor-pointer"
            title="Collapse"
          >
            <svg width="12" height="12" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M3 5l4 4 4-4" />
            </svg>
          </button>
        </div>

        {/* Movable divider (only in split mode header) */}
        {!isIndependent && <SplitDivider onSplitChange={onChatSplitChange} containerRef={railContainerRef} />}

        {/* Tasks tab header */}
        <div
          className={`flex items-center px-2 gap-0 min-w-0 ${isIndependent ? 'h-10' : ''}`}
          style={isIndependent ? undefined : { width: `${100 - chatSplitPct}%` }}
        >
          {/* Tasks tab label */}
          <button
            type="button"
            onClick={() => onSetTab('tasks')}
            className={`flex items-center gap-1.5 px-2 h-full text-xs font-display font-semibold border-b-2 transition-colors cursor-pointer shrink-0 ${
              tab === 'tasks'
                ? 'text-soul border-soul'
                : 'text-fg-secondary border-transparent hover:text-fg'
            }`}
          >
            <svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
              <path d="M3 4l2 2 4-4" />
              <path d="M3 10l2 2 4-4" />
            </svg>
            Tasks
          </button>

          {/* Divider */}
          <div className="w-px h-4 bg-border-subtle mx-1 shrink-0" />

          {/* View mode buttons */}
          {(
            [
              {
                view: 'list' as TaskView,
                title: 'List view',
                icon: (
                  <svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
                    <path d="M3 4h10M3 8h10M3 12h10" />
                  </svg>
                ),
              },
              {
                view: 'kanban' as TaskView,
                title: 'Kanban view',
                icon: (
                  <svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
                    <path d="M3 3v10M8 3v7M13 3v10" />
                  </svg>
                ),
              },
              {
                view: 'grid' as TaskView,
                title: 'Grid view',
                icon: (
                  <svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
                    <rect x="2" y="2" width="5" height="5" rx="1" />
                    <rect x="9" y="2" width="5" height="5" rx="1" />
                    <rect x="2" y="9" width="5" height="5" rx="1" />
                    <rect x="9" y="9" width="5" height="5" rx="1" />
                  </svg>
                ),
              },
              {
                view: 'table' as TaskView,
                title: 'Table view',
                icon: (
                  <svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                    <rect x="2" y="2" width="12" height="12" rx="1" />
                    <path d="M2 5.5h12" />
                    <path d="M6 5.5v8.5" />
                  </svg>
                ),
              },
            ] as { view: TaskView; title: string; icon: React.ReactNode }[]
          ).map(({ view, title, icon }) => (
            <button
              key={view}
              type="button"
              onClick={() => setTaskView(view)}
              title={title}
              className={`w-6 h-6 flex items-center justify-center rounded cursor-pointer transition-colors shrink-0 ${
                taskView === view
                  ? 'bg-overlay text-fg'
                  : 'text-fg-secondary hover:text-fg hover:bg-elevated'
              }`}
            >
              {icon}
            </button>
          ))}

          {/* Divider */}
          <div className="w-px h-4 bg-border-subtle mx-1 shrink-0" />

          {/* Inline Stage filter */}
          <select
            className="soul-select text-[11px] h-6 px-1 rounded shrink-0 bg-elevated border border-border-default cursor-pointer"
            value={filters.stage}
            onChange={(e) => setFilters({ stage: e.target.value as TaskStage | 'all' })}
            title="Filter by stage"
          >
            <option value="all">All Stages</option>
            <option value="backlog">Backlog</option>
            <option value="brainstorm">Brainstorm</option>
            <option value="active">Active</option>
            <option value="blocked">Blocked</option>
            <option value="validation">Validation</option>
            <option value="done">Done</option>
          </select>

          {/* Inline Priority filter */}
          <select
            className="soul-select text-[11px] h-6 px-1 rounded shrink-0 bg-elevated border border-border-default cursor-pointer ml-1"
            value={filters.priority === 'all' ? 'all' : String(filters.priority)}
            onChange={(e) => {
              const val = e.target.value;
              setFilters({ priority: val === 'all' ? 'all' : Number(val) });
            }}
            title="Filter by priority"
          >
            <option value="all">All Priority</option>
            <option value="3">Critical</option>
            <option value="2">High</option>
            <option value="1">Normal</option>
            <option value="0">Low</option>
          </select>

          {/* Divider */}
          <div className="w-px h-4 bg-border-subtle mx-1 shrink-0" />

          {/* Sync Filter toggle */}
          <button
            type="button"
            onClick={onSyncProductFilterToggle}
            title={syncProductFilter ? 'Sync filter ON — showing tasks for active product' : 'Sync filter OFF — showing all products'}
            className={`w-6 h-6 flex items-center justify-center rounded cursor-pointer transition-colors shrink-0 ${
              syncProductFilter
                ? 'bg-soul/15 text-soul'
                : 'text-fg-secondary hover:text-fg hover:bg-elevated'
            }`}
          >
            <svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
              {syncProductFilter ? (
                <>
                  <path d="M8 2v12" />
                  <path d="M3 5l3-3 3 3" />
                  <path d="M7 11l3 3 3-3" />
                </>
              ) : (
                <>
                  <path d="M4 2v12" />
                  <path d="M1 5l3-3 3 3" />
                  <path d="M12 14V2" />
                  <path d="M9 11l3 3 3-3" />
                </>
              )}
            </svg>
          </button>

          <div className="flex-1" />

          {/* New Task button */}
          <button
            type="button"
            onClick={() => setShowNewForm(true)}
            className="text-xs bg-soul/10 hover:bg-soul/20 text-soul px-2 py-0.5 rounded font-display font-semibold transition-colors cursor-pointer shrink-0 whitespace-nowrap"
          >
            + New
          </button>
        </div>
      </div>

      {/* Content split: Chat | Tasks */}
      <div className={`flex ${isIndependent ? 'flex-col' : ''} flex-1 min-h-0 overflow-hidden`}>
        {/* Chat pane */}
        <div
          className="flex flex-col min-h-0 overflow-hidden relative"
          style={isIndependent ? { height: `${chatSplitPct}%` } : { width: `${chatSplitPct}%` }}
        >
          <ChatView
            activeSessionId={activeSessionId}
            onSessionCreated={onSessionCreated}
            activeProduct={activeProduct}
            buildContextString={buildContextString}
            autoInjectContext={autoInjectContext}
            showContextChip={showContextChip}
          />
          {/* History drawer — slides in over the chat pane */}
          {historyOpen && (
            <SessionDrawer
              sessions={sessions}
              activeSessionId={activeSessionId}
              onSelect={(id) => { onSessionSelect(id); setHistoryOpen(false); }}
              onClose={() => setHistoryOpen(false)}
            />
          )}
        </div>

        {/* Movable split divider in content area */}
        <SplitDivider
          onSplitChange={onChatSplitChange}
          containerRef={railContainerRef}
          direction={isIndependent ? 'horizontal' : 'vertical'}
        />

        {/* Tasks pane */}
        <div
          className="flex flex-col min-h-0 overflow-hidden"
          style={isIndependent ? { height: `${100 - chatSplitPct}%` } : { width: `${100 - chatSplitPct}%` }}
        >
          <TaskContent
            taskView={taskView}
            filteredTasks={filteredTasks}
            tasksByStage={tasksByStage}
            gridSubView={gridSubView}
            onGridSubViewChange={setGridSubView}
            onTaskClick={setSelectedTask}
            onClearFilters={() => setFilters({ stage: 'all', priority: 'all' })}
            taskActivities={taskActivities}
          />
        </div>
      </div>

      {/* Drag handle — on bottom edge for top rail */}
      {position === 'top' && (
        <div
          onMouseDown={onDragStart}
          className="h-1 w-full cursor-row-resize shrink-0 hover:bg-soul/30 transition-colors"
          title="Drag to resize"
        />
      )}
    </div>
  );

  // ── Independent expanded: only expand the active tab's panel ──
  const chatCollapsedBar = (
    <div className="flex items-stretch h-12 bg-surface border-b border-border-subtle shrink-0">
      {chatRailContent}
      <button type="button" onClick={toggleChat} className="w-8 h-8 flex items-center justify-center rounded-full bg-soul text-deep hover:bg-soul/85 transition-colors cursor-pointer shrink-0 self-center mx-1 shadow-sm" title="Expand chat">
        {upChevron}
      </button>
    </div>
  );

  const tasksCollapsedBar = (
    <div className="flex items-stretch h-12 bg-surface shrink-0">
      {taskRailContent}
      <button type="button" onClick={toggleTasks} className="w-8 h-8 flex items-center justify-center rounded-full bg-soul text-deep hover:bg-soul/85 transition-colors cursor-pointer shrink-0 self-center mx-1 shadow-sm" title="Expand tasks">
        {upChevron}
      </button>
    </div>
  );

  const independentExpandedPanel = (
    <div
      ref={railContainerRef}
      data-testid="horizontal-rail"
      className="flex flex-col bg-surface shrink-0"
      style={{
        borderTopWidth: position === 'bottom' ? 1 : 0,
        borderBottomWidth: position === 'top' ? 1 : 0,
        borderColor: 'var(--color-border-subtle)',
        borderStyle: 'solid',
      }}
    >
      {/* Chat section */}
      {showChat && (chatOpen ? (
        <div className="flex flex-col shrink-0" style={{ height: `${heightVh}vh` }}>
          {position === 'bottom' && (
            <div onMouseDown={onDragStart} className="h-1 w-full cursor-row-resize shrink-0 hover:bg-soul/30 transition-colors" title="Drag to resize" />
          )}
          <div className="flex items-center px-4 gap-1.5 min-w-0 h-10 border-b border-border-subtle shrink-0">
            <span className="flex items-center gap-1.5 px-2 h-full text-xs font-display font-semibold text-soul">
              <span className="text-base leading-none">&#9670;</span> Chat
            </span>
            <div className="flex-1" />
            <button
              type="button"
              onClick={async () => {
                try { const res = await fetch('/api/reauth', { method: 'POST' }); setReauthStatus(res.ok ? 'ok' : 'error'); } catch { setReauthStatus('error'); }
                setTimeout(() => setReauthStatus('idle'), 2000);
              }}
              className={`flex items-center gap-1 px-1.5 h-7 rounded text-[10px] font-display transition-colors cursor-pointer ${reauthStatus === 'ok' ? 'text-green-400' : reauthStatus === 'error' ? 'text-red-400' : 'text-fg-secondary hover:text-fg hover:bg-elevated'}`}
              title="Refresh AI credentials"
            >
              <svg width="12" height="12" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                <path d="M1 7a6 6 0 0111.196-3M13 7A6 6 0 011.804 10" /><path d="M1 1v3h3M13 13v-3h-3" />
              </svg>
              Refresh
            </button>
            {runningSessions.length > 0 && (
              <button type="button" onClick={() => setHistoryOpen((o) => !o)} className="h-6 px-1.5 flex items-center gap-1 rounded bg-stage-active/15 text-stage-active text-[10px] font-display font-semibold cursor-pointer hover:bg-stage-active/25" title={`${runningSessions.length} running`}>
                <span className="relative flex h-1.5 w-1.5"><span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-stage-active opacity-75" /><span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-stage-active" /></span>
                {runningSessions.length}
              </button>
            )}
            {unreadSessions.length > 0 && (
              <button type="button" onClick={() => setHistoryOpen((o) => !o)} className="h-6 px-1.5 flex items-center gap-1 rounded bg-stage-done/15 text-stage-done text-[10px] font-display font-semibold cursor-pointer hover:bg-stage-done/25" title={`${unreadSessions.length} unread`}>
                <span className="w-1.5 h-1.5 rounded-full bg-stage-done" />
                {unreadSessions.length}
              </button>
            )}
            <button type="button" onClick={() => setHistoryOpen((o) => !o)} className={`flex items-center gap-1 px-1.5 h-7 rounded text-[10px] font-display transition-colors cursor-pointer ${historyOpen ? 'bg-soul/15 text-soul' : 'hover:bg-elevated text-fg-secondary hover:text-fg'}`} title="Chat history">
              <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"><circle cx="8" cy="8" r="6.5" /><path d="M8 4.5V8l2.5 2" /></svg>
              History
            </button>
            <button type="button" onClick={() => { setHistoryOpen(false); toggleChat(); }} className="flex items-center gap-1 px-1.5 h-7 rounded-full bg-soul text-deep hover:bg-soul/85 text-[10px] font-display transition-colors cursor-pointer" title="Collapse chat">
              {downChevron} Collapse
            </button>
          </div>
          <div className="flex-1 min-h-0 overflow-hidden relative">
            <ChatView activeSessionId={activeSessionId} onSessionCreated={onSessionCreated} activeProduct={activeProduct} buildContextString={buildContextString} autoInjectContext={autoInjectContext} showContextChip={showContextChip} />
            {historyOpen && (
              <SessionDrawer sessions={sessions} activeSessionId={activeSessionId} onSelect={(id) => { onSessionSelect(id); setHistoryOpen(false); }} onClose={() => setHistoryOpen(false)} />
            )}
          </div>
        </div>
      ) : chatCollapsedBar)}

      {/* Tasks section */}
      {showTasks && (tasksOpen ? (
        <div className="flex flex-col shrink-0" style={{ height: `${heightVh}vh` }}>
          {position === 'bottom' && (
            <div onMouseDown={onDragStart} className="h-1 w-full cursor-row-resize shrink-0 hover:bg-soul/30 transition-colors" title="Drag to resize" />
          )}
          <div className="flex items-center px-2 gap-1 min-w-0 h-10 border-b border-border-subtle shrink-0">
            <span className="flex items-center gap-1.5 px-2 h-full text-xs font-display font-semibold text-soul shrink-0">
              <svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"><path d="M3 4l2 2 4-4" /><path d="M3 10l2 2 4-4" /></svg>
              Tasks
            </span>
            <div className="w-px h-4 bg-border-subtle mx-0.5 shrink-0" />
            {([
              { view: 'list' as TaskView, label: 'List', icon: <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"><path d="M3 4h10M3 8h10M3 12h10" /></svg> },
              { view: 'kanban' as TaskView, label: 'Kanban', icon: <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"><path d="M3 3v10M8 3v7M13 3v10" /></svg> },
              { view: 'grid' as TaskView, label: 'Grid', icon: <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"><rect x="2" y="2" width="5" height="5" rx="1" /><rect x="9" y="2" width="5" height="5" rx="1" /><rect x="2" y="9" width="5" height="5" rx="1" /><rect x="9" y="9" width="5" height="5" rx="1" /></svg> },
              { view: 'table' as TaskView, label: 'Table', icon: <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"><rect x="2" y="2" width="12" height="12" rx="1" /><path d="M2 5.5h12" /><path d="M6 5.5v8.5" /></svg> },
            ] as { view: TaskView; label: string; icon: React.ReactNode }[]).map(({ view, label, icon }) => (
              <button key={view} type="button" onClick={() => setTaskView(view)} title={`${label} view`} className={`flex items-center gap-1 px-1.5 h-6 rounded cursor-pointer text-[10px] font-display transition-colors shrink-0 ${taskView === view ? 'bg-overlay text-fg' : 'text-fg-secondary hover:text-fg hover:bg-elevated'}`}>
                {icon} {label}
              </button>
            ))}
            <div className="w-px h-4 bg-border-subtle mx-0.5 shrink-0" />
            <select className="soul-select text-[11px] h-6 px-1 rounded shrink-0 bg-elevated border border-border-default cursor-pointer" value={filters.stage} onChange={(e) => setFilters({ stage: e.target.value as TaskStage | 'all' })} title="Filter by stage">
              <option value="all">All Stages</option>
              <option value="backlog">Backlog</option>
              <option value="brainstorm">Brainstorm</option>
              <option value="active">Active</option>
              <option value="blocked">Blocked</option>
              <option value="validation">Validation</option>
              <option value="done">Done</option>
            </select>
            <select className="soul-select text-[11px] h-6 px-1 rounded shrink-0 bg-elevated border border-border-default cursor-pointer ml-1" value={filters.priority === 'all' ? 'all' : String(filters.priority)} onChange={(e) => { const val = e.target.value; setFilters({ priority: val === 'all' ? 'all' : Number(val) }); }} title="Filter by priority">
              <option value="all">All Priority</option>
              <option value="3">Critical</option>
              <option value="2">High</option>
              <option value="1">Normal</option>
              <option value="0">Low</option>
            </select>
            <div className="w-px h-4 bg-border-subtle mx-0.5 shrink-0" />
            <button type="button" onClick={onSyncProductFilterToggle} title={syncProductFilter ? 'Sync filter ON' : 'Sync filter OFF'} className={`flex items-center gap-1 px-1.5 h-6 rounded cursor-pointer text-[10px] font-display transition-colors shrink-0 ${syncProductFilter ? 'bg-soul/15 text-soul' : 'text-fg-secondary hover:text-fg hover:bg-elevated'}`}>
              <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                {syncProductFilter ? (<><path d="M8 2v12" /><path d="M3 5l3-3 3 3" /><path d="M7 11l3 3 3-3" /></>) : (<><path d="M4 2v12" /><path d="M1 5l3-3 3 3" /><path d="M12 14V2" /><path d="M9 11l3 3 3-3" /></>)}
              </svg>
              Sync
            </button>
            <div className="flex-1" />
            <button type="button" onClick={() => setShowNewForm(true)} className="text-xs bg-soul/10 hover:bg-soul/20 text-soul px-2 py-0.5 rounded font-display font-semibold transition-colors cursor-pointer shrink-0 whitespace-nowrap">+ New</button>
            <div className="w-2 shrink-0" />
            <button type="button" onClick={toggleTasks} className="flex items-center gap-1 px-1.5 h-7 rounded-full bg-soul text-deep hover:bg-soul/85 text-[10px] font-display transition-colors cursor-pointer" title="Collapse tasks">
              {downChevron} Collapse
            </button>
          </div>
          <div className="flex-1 min-h-0 overflow-hidden">
            <TaskContent taskView={taskView} filteredTasks={filteredTasks} tasksByStage={tasksByStage} gridSubView={gridSubView} onGridSubViewChange={setGridSubView} onTaskClick={setSelectedTask} onClearFilters={() => setFilters({ stage: 'all', priority: 'all' })} taskActivities={taskActivities} />
          </div>
          {position === 'top' && (
            <div onMouseDown={onDragStart} className="h-1 w-full cursor-row-resize shrink-0 hover:bg-soul/30 transition-colors" title="Drag to resize" />
          )}
        </div>
      ) : tasksCollapsedBar)}
    </div>
  );

  return (
    <>
      {!expanded ? railBar : isIndependent ? independentExpandedPanel : expandedPanel}

      {/* Task detail modal */}
      {selectedTask && (
        <TaskDetail
          task={tasks.find((t) => t.id === selectedTask.id) ?? selectedTask}
          onClose={() => setSelectedTask(null)}
          onMove={async (id, stage, comment) => {
            await moveTask(id, stage, comment);
            setSelectedTask(null);
          }}
          onUpdate={async (id, updates) => {
            const updated = await updateTask(id, updates);
            setSelectedTask(updated);
            return updated;
          }}
          onDelete={async (id) => {
            await deleteTask(id);
            setSelectedTask(null);
          }}
          activities={taskActivities[selectedTask.id] || []}
          streamContent={taskStreams[selectedTask.id] || ''}
          products={products}
          comments={taskComments[selectedTask.id]}
          onFetchComments={fetchComments}
          onAddComment={addComment}
        />
      )}

      {showNewForm && (
        <NewTaskForm
          onClose={() => setShowNewForm(false)}
          onCreate={async (title, description, priority, product) => {
            await createTask(title, description, priority, product);
            setShowNewForm(false);
          }}
        />
      )}
    </>
  );
}
