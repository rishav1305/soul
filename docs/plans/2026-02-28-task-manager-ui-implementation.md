# Task Manager UI Redesign Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Redesign the Soul two-panel layout with collapsible panels, 4 task view modes, filters, dynamic panel width, and chat session management.

**Architecture:** State machine panel system via `useLayoutStore` hook. Each panel (chat/tasks) has `rail | open` states. Task panel has 4 view modes (rail/list/kanban/grid). Filters, auto-resize, and manual drag override coordinated through a single state hook persisted to localStorage.

**Tech Stack:** React 19, TypeScript, TailwindCSS 4, existing Go backend with SQLite

---

### Task 1: Types + useLayoutStore hook

**Files:**
- Modify: `web/src/lib/types.ts`
- Create: `web/src/hooks/useLayoutStore.ts`

**Step 1: Add new types to types.ts**

Add these types after the existing `PlannerTask` interface in `web/src/lib/types.ts`:

```typescript
export type PanelState = 'rail' | 'open';
export type TaskView = 'list' | 'kanban' | 'grid';
export type GridSubView = 'grid' | 'table' | 'grouped';

export interface TaskFilters {
  stage: TaskStage | 'all';
  priority: number | 'all'; // 0-3 or 'all'
  product: string | 'all';
}

export interface LayoutState {
  chatState: PanelState;
  taskState: PanelState;
  taskView: TaskView;
  gridSubView: GridSubView;
  panelWidth: number | null; // null = auto, number = manual %
  filters: TaskFilters;
}

export interface ChatSession {
  id: number;
  title: string;
  status: 'running' | 'idle' | 'completed';
  created_at: string;
  updated_at: string;
}

export interface ChatMessageRecord {
  id: number;
  session_id: number;
  role: 'user' | 'assistant' | 'system';
  content: string;
  created_at: string;
}
```

**Step 2: Create useLayoutStore.ts**

Create `web/src/hooks/useLayoutStore.ts`:

```typescript
import { useState, useCallback, useMemo } from 'react';
import type { LayoutState, PanelState, TaskView, GridSubView, TaskFilters, TaskStage } from '../lib/types.ts';

const STORAGE_KEY = 'soul-layout';

const DEFAULT_STATE: LayoutState = {
  chatState: 'open',
  taskState: 'open',
  taskView: 'kanban',
  gridSubView: 'grid',
  panelWidth: null,
  filters: { stage: 'all', priority: 'all', product: 'all' },
};

function loadState(): LayoutState {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) return { ...DEFAULT_STATE, ...JSON.parse(raw) };
  } catch { /* ignore */ }
  return { ...DEFAULT_STATE };
}

function saveState(state: LayoutState) {
  try { localStorage.setItem(STORAGE_KEY, JSON.stringify(state)); } catch { /* ignore */ }
}

// Auto-compute task panel width % based on visible task count.
export function autoWidth(taskCount: number): number {
  if (taskCount === 0) return 15;
  if (taskCount <= 3) return 25;
  if (taskCount <= 10) return 40;
  if (taskCount <= 20) return 55;
  return 75;
}

export function useLayoutStore() {
  const [state, setState] = useState<LayoutState>(loadState);

  const update = useCallback((partial: Partial<LayoutState>) => {
    setState(prev => {
      const next = { ...prev, ...partial };
      saveState(next);
      return next;
    });
  }, []);

  const setChatState = useCallback((s: PanelState) => {
    setState(prev => {
      // Can't collapse if other panel is also rail
      if (s === 'rail' && prev.taskState === 'rail') return prev;
      const next = { ...prev, chatState: s };
      saveState(next);
      return next;
    });
  }, []);

  const setTaskState = useCallback((s: PanelState) => {
    setState(prev => {
      if (s === 'rail' && prev.chatState === 'rail') return prev;
      const next = { ...prev, taskState: s };
      saveState(next);
      return next;
    });
  }, []);

  const setTaskView = useCallback((v: TaskView) => {
    update({ taskView: v, taskState: 'open' });
  }, [update]);

  const setGridSubView = useCallback((v: GridSubView) => {
    update({ gridSubView: v });
  }, [update]);

  const setPanelWidth = useCallback((w: number | null) => {
    update({ panelWidth: w });
  }, [update]);

  const setFilters = useCallback((f: Partial<TaskFilters>) => {
    setState(prev => {
      const next = { ...prev, filters: { ...prev.filters, ...f } };
      saveState(next);
      return next;
    });
  }, []);

  const canCollapse = useCallback((panel: 'chat' | 'task'): boolean => {
    if (panel === 'chat') return state.taskState === 'open';
    return state.chatState === 'open';
  }, [state.chatState, state.taskState]);

  return {
    ...state,
    setChatState,
    setTaskState,
    setTaskView,
    setGridSubView,
    setPanelWidth,
    setFilters,
    canCollapse,
  };
}
```

**Step 3: Verify build**

Run: `cd web && npx vite build 2>&1 | tail -5`
Expected: Build succeeds (types are only imported, not rendered yet).

**Step 4: Commit**

```bash
git add web/src/lib/types.ts web/src/hooks/useLayoutStore.ts
git commit -m "feat: add layout types and useLayoutStore state machine"
```

---

### Task 2: AppShell + ResizeDivider + Rails

**Files:**
- Create: `web/src/components/layout/AppShell.tsx`
- Create: `web/src/components/layout/ResizeDivider.tsx`
- Create: `web/src/components/layout/ChatRail.tsx`
- Create: `web/src/components/layout/TaskRail.tsx`
- Modify: `web/src/App.tsx`

**Step 1: Create ChatRail.tsx**

Create `web/src/components/layout/ChatRail.tsx`:

```typescript
interface ChatRailProps {
  unreadCount: number;
  onExpand: () => void;
}

export default function ChatRail({ unreadCount, onExpand }: ChatRailProps) {
  return (
    <button
      type="button"
      onClick={onExpand}
      className="w-10 h-full bg-zinc-950 border-r border-zinc-800 flex flex-col items-center py-3 gap-3 hover:bg-zinc-900 transition-colors cursor-pointer shrink-0"
    >
      <span className="text-zinc-400 text-lg">&#9670;</span>
      {unreadCount > 0 && (
        <span className="text-xs font-medium text-sky-400 bg-sky-950 rounded-full px-1.5 py-0.5">
          {unreadCount > 9 ? '9+' : unreadCount}
        </span>
      )}
      <span className="mt-auto text-zinc-500 text-sm">&#128172;</span>
    </button>
  );
}
```

**Step 2: Create TaskRail.tsx**

Create `web/src/components/layout/TaskRail.tsx`:

```typescript
import type { TaskStage, PlannerTask } from '../../lib/types.ts';

const STAGES: TaskStage[] = ['backlog', 'brainstorm', 'active', 'blocked', 'validation', 'done'];

const DOT_COLORS: Record<TaskStage, string> = {
  backlog: 'bg-zinc-400',
  brainstorm: 'bg-purple-400',
  active: 'bg-sky-400',
  blocked: 'bg-red-400',
  validation: 'bg-amber-400',
  done: 'bg-green-400',
};

interface TaskRailProps {
  tasks: PlannerTask[];
  onExpand: () => void;
}

export default function TaskRail({ tasks, onExpand }: TaskRailProps) {
  const counts: Record<TaskStage, number> = { backlog: 0, brainstorm: 0, active: 0, blocked: 0, validation: 0, done: 0 };
  for (const t of tasks) counts[t.stage]++;

  return (
    <button
      type="button"
      onClick={onExpand}
      className="w-10 h-full bg-zinc-950 border-l border-zinc-800 flex flex-col items-center py-3 gap-1 hover:bg-zinc-900 transition-colors cursor-pointer shrink-0"
    >
      <span className="text-zinc-400 text-xs font-bold mb-2">&#8862;</span>
      {STAGES.map(stage => (
        <div key={stage} className="flex items-center gap-1">
          <span className={`w-1.5 h-1.5 rounded-full ${DOT_COLORS[stage]}`} />
          <span className="text-[10px] text-zinc-500">{counts[stage]}</span>
        </div>
      ))}
    </button>
  );
}
```

**Step 3: Create ResizeDivider.tsx**

Create `web/src/components/layout/ResizeDivider.tsx`:

```typescript
import { useCallback, useRef } from 'react';

interface ResizeDividerProps {
  onResize: (chatPercent: number) => void;
}

export default function ResizeDivider({ onResize }: ResizeDividerProps) {
  const dragging = useRef(false);

  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    dragging.current = true;

    const handleMouseMove = (ev: MouseEvent) => {
      if (!dragging.current) return;
      const pct = (ev.clientX / window.innerWidth) * 100;
      // Clamp: chat min 25%, tasks min 15% => chat max 85%
      const clamped = Math.min(85, Math.max(25, pct));
      onResize(clamped);
    };

    const handleMouseUp = () => {
      dragging.current = false;
      document.removeEventListener('mousemove', handleMouseMove);
      document.removeEventListener('mouseup', handleMouseUp);
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
    };

    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
    document.addEventListener('mousemove', handleMouseMove);
    document.addEventListener('mouseup', handleMouseUp);
  }, [onResize]);

  return (
    <div
      onMouseDown={handleMouseDown}
      className="w-1 hover:w-2 bg-zinc-800 hover:bg-zinc-600 cursor-col-resize transition-all shrink-0"
    />
  );
}
```

**Step 4: Create AppShell.tsx**

Create `web/src/components/layout/AppShell.tsx`:

```typescript
import { useState, useMemo } from 'react';
import { useLayoutStore, autoWidth } from '../../hooks/useLayoutStore.ts';
import { usePlanner } from '../../hooks/usePlanner.ts';
import ChatRail from './ChatRail.tsx';
import TaskRail from './TaskRail.tsx';
import ResizeDivider from './ResizeDivider.tsx';
import ChatPanel from '../chat/ChatPanel.tsx';
import TaskPanel from '../planner/TaskPanel.tsx';
import type { PlannerTask } from '../../lib/types.ts';

// Filter tasks based on current filters.
function applyFilters(tasks: PlannerTask[], filters: { stage: string; priority: number | string; product: string }): PlannerTask[] {
  return tasks.filter(t => {
    if (filters.stage !== 'all' && t.stage !== filters.stage) return false;
    if (filters.priority !== 'all' && t.priority !== filters.priority) return false;
    if (filters.product !== 'all' && t.product !== filters.product) return false;
    return true;
  });
}

export default function AppShell() {
  const layout = useLayoutStore();
  const { tasks, tasksByStage, loading, createTask, moveTask, deleteTask } = usePlanner();
  const [unreadCount, setUnreadCount] = useState(0);

  const filteredTasks = useMemo(
    () => applyFilters(tasks, layout.filters),
    [tasks, layout.filters],
  );

  // Unique products for filter dropdown.
  const products = useMemo(
    () => [...new Set(tasks.map(t => t.product).filter(Boolean))],
    [tasks],
  );

  // Compute panel widths.
  const chatOpen = layout.chatState === 'open';
  const taskOpen = layout.taskState === 'open';

  let chatWidth: string;
  let taskWidth: string;

  if (!chatOpen && !taskOpen) {
    // Shouldn't happen due to constraints, but handle gracefully.
    chatWidth = 'calc(100% - 40px)';
    taskWidth = '40px';
  } else if (!chatOpen) {
    chatWidth = '40px';
    taskWidth = 'calc(100% - 40px)';
  } else if (!taskOpen) {
    taskWidth = '40px';
    chatWidth = 'calc(100% - 40px)';
  } else {
    // Both open — use manual or auto width.
    const taskPct = layout.panelWidth ?? autoWidth(filteredTasks.length);
    chatWidth = `${100 - taskPct}%`;
    taskWidth = `${taskPct}%`;
  }

  return (
    <div className="h-screen bg-zinc-950 text-zinc-100 flex overflow-hidden">
      {/* Chat side */}
      <div
        className="h-full shrink-0 transition-[width] duration-200 ease-in-out"
        style={{ width: chatWidth }}
      >
        {chatOpen ? (
          <ChatPanel
            onCollapse={() => layout.setChatState('rail')}
            canCollapse={layout.canCollapse('chat')}
            onUnreadChange={setUnreadCount}
          />
        ) : (
          <ChatRail unreadCount={unreadCount} onExpand={() => layout.setChatState('open')} />
        )}
      </div>

      {/* Resize divider — only when both panels are open */}
      {chatOpen && taskOpen && (
        <ResizeDivider onResize={(chatPct) => layout.setPanelWidth(100 - chatPct)} />
      )}

      {/* Task side */}
      <div
        className="h-full shrink-0 transition-[width] duration-200 ease-in-out"
        style={{ width: taskWidth }}
      >
        {taskOpen ? (
          <TaskPanel
            layout={layout}
            tasks={tasks}
            filteredTasks={filteredTasks}
            tasksByStage={tasksByStage}
            products={products}
            loading={loading}
            createTask={createTask}
            moveTask={moveTask}
            deleteTask={deleteTask}
          />
        ) : (
          <TaskRail tasks={tasks} onExpand={() => layout.setTaskState('open')} />
        )}
      </div>
    </div>
  );
}
```

**Step 5: Update App.tsx**

Replace `web/src/App.tsx`:

```typescript
import AppShell from './components/layout/AppShell.tsx';
import { WebSocketContext, useWebSocketProvider } from './hooks/useWebSocket.ts';

export default function App() {
  const ws = useWebSocketProvider();

  return (
    <WebSocketContext.Provider value={ws}>
      <AppShell />
    </WebSocketContext.Provider>
  );
}
```

**Step 6: Create stub ChatPanel.tsx and TaskPanel.tsx**

Create `web/src/components/chat/ChatPanel.tsx` (stub — wraps existing ChatView):

```typescript
import ChatView from './ChatView.tsx';

interface ChatPanelProps {
  onCollapse: () => void;
  canCollapse: boolean;
  onUnreadChange: (count: number) => void;
}

export default function ChatPanel({ onCollapse, canCollapse }: ChatPanelProps) {
  return (
    <div className="flex flex-col h-full">
      <div className="h-10 border-b border-zinc-800 flex items-center px-3 shrink-0 gap-2">
        <button type="button" className="text-zinc-500 hover:text-zinc-300 text-sm cursor-pointer">&#9776;</button>
        <span className="text-sm font-bold text-zinc-100">&#9670; Soul Chat</span>
        <div className="flex-1" />
        {canCollapse && (
          <button
            type="button"
            onClick={onCollapse}
            className="text-zinc-500 hover:text-zinc-300 text-sm cursor-pointer"
            title="Collapse"
          >
            &#8722;
          </button>
        )}
      </div>
      <ChatView />
    </div>
  );
}
```

Create `web/src/components/planner/TaskPanel.tsx` (stub — wraps existing KanbanBoard):

```typescript
import { useState } from 'react';
import type { PlannerTask, TaskStage } from '../../lib/types.ts';
import KanbanBoard from './KanbanBoard.tsx';
import TaskDetail from './TaskDetail.tsx';
import NewTaskForm from './NewTaskForm.tsx';

interface TaskPanelProps {
  layout: any; // useLayoutStore return type
  tasks: PlannerTask[];
  filteredTasks: PlannerTask[];
  tasksByStage: Record<TaskStage, PlannerTask[]>;
  products: string[];
  loading: boolean;
  createTask: (title: string, description: string, priority: number, product: string) => Promise<PlannerTask>;
  moveTask: (id: number, stage: TaskStage, comment: string) => Promise<PlannerTask>;
  deleteTask: (id: number) => Promise<void>;
}

export default function TaskPanel({ layout, filteredTasks, tasksByStage, products, loading, createTask, moveTask, deleteTask }: TaskPanelProps) {
  const [selectedTask, setSelectedTask] = useState<PlannerTask | null>(null);
  const [showNewForm, setShowNewForm] = useState(false);

  const handleCreate = async (title: string, description: string, priority: number, product: string) => {
    await createTask(title, description, priority, product);
    setShowNewForm(false);
  };

  const handleMove = async (id: number, stage: TaskStage, comment: string) => {
    await moveTask(id, stage, comment);
    setSelectedTask(null);
  };

  const handleDelete = async (id: number) => {
    await deleteTask(id);
    setSelectedTask(null);
  };

  return (
    <div className="flex flex-col h-full bg-zinc-950">
      {/* Task Navbar */}
      <div className="h-10 border-b border-zinc-800 flex items-center px-3 shrink-0 gap-1">
        <span className="text-sm font-bold text-zinc-100 mr-2">Tasks</span>
        {/* View mode buttons — placeholder, will be wired in Task 4 */}
        <div className="flex gap-0.5">
          {(['list', 'kanban', 'grid'] as const).map(v => (
            <button
              key={v}
              type="button"
              onClick={() => layout.setTaskView(v)}
              className={`px-1.5 py-0.5 text-xs rounded cursor-pointer ${layout.taskView === v ? 'bg-zinc-700 text-zinc-100' : 'text-zinc-500 hover:text-zinc-300'}`}
            >
              {v === 'list' ? '≡' : v === 'kanban' ? '⊞' : '⊟'}
            </button>
          ))}
        </div>
        <button
          type="button"
          onClick={() => layout.setTaskState('rail')}
          className={`text-zinc-500 hover:text-zinc-300 text-sm cursor-pointer ml-1 ${!layout.canCollapse('task') ? 'opacity-30 pointer-events-none' : ''}`}
          title="Collapse"
        >
          &times;
        </button>
        {layout.panelWidth !== null && (
          <button
            type="button"
            onClick={() => layout.setPanelWidth(null)}
            className="text-zinc-500 hover:text-zinc-300 text-xs cursor-pointer ml-1"
            title="Reset width to auto"
          >
            &#8635;
          </button>
        )}
        <div className="flex-1" />
        <button
          type="button"
          onClick={() => setShowNewForm(true)}
          className="px-2 py-1 text-xs font-medium rounded bg-sky-600 hover:bg-sky-500 text-white cursor-pointer"
        >
          + New Task
        </button>
      </div>

      {/* Content — currently just KanbanBoard, view switching in Task 4 */}
      {loading ? (
        <div className="flex-1 flex items-center justify-center">
          <span className="text-zinc-500 text-sm">Loading tasks...</span>
        </div>
      ) : (
        <div className="flex-1 overflow-hidden">
          <KanbanBoard
            tasksByStage={tasksByStage}
            onTaskClick={setSelectedTask}
          />
        </div>
      )}

      {selectedTask && (
        <TaskDetail task={selectedTask} onClose={() => setSelectedTask(null)} onMove={handleMove} onDelete={handleDelete} />
      )}
      {showNewForm && (
        <NewTaskForm onClose={() => setShowNewForm(false)} onCreate={handleCreate} />
      )}
    </div>
  );
}
```

**Step 7: Modify KanbanBoard to accept props**

Modify `web/src/components/planner/KanbanBoard.tsx` to accept `tasksByStage` and `onTaskClick` as props instead of managing its own state:

```typescript
import type { PlannerTask, TaskStage } from '../../lib/types.ts';
import StageColumn from './StageColumn.tsx';

const STAGES: TaskStage[] = ['backlog', 'brainstorm', 'active', 'blocked', 'validation', 'done'];

interface KanbanBoardProps {
  tasksByStage: Record<TaskStage, PlannerTask[]>;
  onTaskClick: (task: PlannerTask) => void;
}

export default function KanbanBoard({ tasksByStage, onTaskClick }: KanbanBoardProps) {
  return (
    <div className="flex h-full overflow-x-auto overflow-y-hidden">
      {STAGES.map((stage) => (
        <StageColumn
          key={stage}
          stage={stage}
          tasks={tasksByStage[stage]}
          onTaskClick={onTaskClick}
        />
      ))}
    </div>
  );
}
```

**Step 8: Verify build**

Run: `cd web && npx vite build 2>&1 | tail -5`
Expected: Build succeeds. UI shows AppShell with chat panel (navbar + ChatView) and task panel (navbar + KanbanBoard).

**Step 9: Commit**

```bash
git add web/src/components/layout/ web/src/components/chat/ChatPanel.tsx web/src/components/planner/TaskPanel.tsx web/src/components/planner/KanbanBoard.tsx web/src/App.tsx
git commit -m "feat: add AppShell layout with collapsible panels and resize divider"
```

---

### Task 3: FilterBar component

**Files:**
- Create: `web/src/components/planner/FilterBar.tsx`
- Modify: `web/src/components/planner/TaskPanel.tsx`

**Step 1: Create FilterBar.tsx**

Create `web/src/components/planner/FilterBar.tsx`:

```typescript
import type { TaskStage, TaskFilters } from '../../lib/types.ts';

const STAGES: { value: TaskStage | 'all'; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 'backlog', label: 'Backlog' },
  { value: 'brainstorm', label: 'Brainstorm' },
  { value: 'active', label: 'Active' },
  { value: 'blocked', label: 'Blocked' },
  { value: 'validation', label: 'Validation' },
  { value: 'done', label: 'Done' },
];

const PRIORITIES: { value: number | 'all'; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 3, label: 'Critical' },
  { value: 2, label: 'High' },
  { value: 1, label: 'Normal' },
  { value: 0, label: 'Low' },
];

interface FilterBarProps {
  filters: TaskFilters;
  products: string[];
  onChange: (f: Partial<TaskFilters>) => void;
}

export default function FilterBar({ filters, products, onChange }: FilterBarProps) {
  return (
    <div className="flex items-center gap-3 px-3 py-1.5 border-b border-zinc-800 text-xs shrink-0">
      <label className="flex items-center gap-1 text-zinc-500">
        Stage:
        <select
          value={String(filters.stage)}
          onChange={e => onChange({ stage: e.target.value as TaskStage | 'all' })}
          className="bg-zinc-800 border border-zinc-700 rounded px-1.5 py-0.5 text-zinc-300 text-xs"
        >
          {STAGES.map(s => <option key={s.value} value={s.value}>{s.label}</option>)}
        </select>
      </label>

      <label className="flex items-center gap-1 text-zinc-500">
        Priority:
        <select
          value={String(filters.priority)}
          onChange={e => {
            const v = e.target.value;
            onChange({ priority: v === 'all' ? 'all' : Number(v) });
          }}
          className="bg-zinc-800 border border-zinc-700 rounded px-1.5 py-0.5 text-zinc-300 text-xs"
        >
          {PRIORITIES.map(p => <option key={String(p.value)} value={String(p.value)}>{p.label}</option>)}
        </select>
      </label>

      <label className="flex items-center gap-1 text-zinc-500">
        Product:
        <select
          value={filters.product}
          onChange={e => onChange({ product: e.target.value })}
          className="bg-zinc-800 border border-zinc-700 rounded px-1.5 py-0.5 text-zinc-300 text-xs"
        >
          <option value="all">All</option>
          {products.map(p => <option key={p} value={p}>{p}</option>)}
        </select>
      </label>
    </div>
  );
}
```

**Step 2: Wire FilterBar into TaskPanel**

Add FilterBar between the navbar and content in `web/src/components/planner/TaskPanel.tsx`. Import `FilterBar` and render it after the navbar div, before the content:

```typescript
import FilterBar from './FilterBar.tsx';
// ... in the return JSX, after the navbar div:
<FilterBar filters={layout.filters} products={products} onChange={layout.setFilters} />
```

**Step 3: Verify build**

Run: `cd web && npx vite build 2>&1 | tail -5`
Expected: Build succeeds.

**Step 4: Commit**

```bash
git add web/src/components/planner/FilterBar.tsx web/src/components/planner/TaskPanel.tsx
git commit -m "feat: add filter bar with stage, priority, and product dropdowns"
```

---

### Task 4: ListView component (View 2)

**Files:**
- Create: `web/src/components/planner/ListView.tsx`
- Create: `web/src/components/planner/TaskContent.tsx`
- Modify: `web/src/components/planner/TaskPanel.tsx`

**Step 1: Create ListView.tsx**

Create `web/src/components/planner/ListView.tsx`:

```typescript
import { useState } from 'react';
import type { PlannerTask, TaskStage, TaskSubstep } from '../../lib/types.ts';

// Active-first ordering for the list view.
const STAGE_ORDER: TaskStage[] = ['active', 'backlog', 'brainstorm', 'blocked', 'validation', 'done'];

const STAGE_LABELS: Record<TaskStage, string> = {
  active: 'Active', backlog: 'Backlog', brainstorm: 'Brainstorm',
  blocked: 'Blocked', validation: 'Validation', done: 'Done',
};

const STAGE_DOT_COLORS: Record<TaskStage, string> = {
  backlog: 'bg-zinc-400', brainstorm: 'bg-purple-400', active: 'bg-sky-400',
  blocked: 'bg-red-400', validation: 'bg-amber-400', done: 'bg-green-400',
};

const PRIORITY_LABELS: Record<number, string> = { 0: 'Low', 1: 'Norm', 2: 'High', 3: 'Crit' };
const PRIORITY_COLORS: Record<number, string> = { 0: 'text-zinc-500', 1: 'text-zinc-300', 2: 'text-amber-400', 3: 'text-red-400' };

const SUBSTEP_LABELS: Record<TaskSubstep, string> = {
  tdd: 'TDD', implementing: 'Impl', reviewing: 'Review',
  qa_test: 'QA', e2e_test: 'E2E', security_review: 'Sec',
};
const SUBSTEP_ORDER: TaskSubstep[] = ['tdd', 'implementing', 'reviewing', 'qa_test', 'e2e_test', 'security_review'];

interface ListViewProps {
  tasks: PlannerTask[];
  onTaskClick: (task: PlannerTask) => void;
}

export default function ListView({ tasks, onTaskClick }: ListViewProps) {
  const [collapsed, setCollapsed] = useState<Set<TaskStage>>(new Set());

  const grouped: Record<TaskStage, PlannerTask[]> = {
    active: [], backlog: [], brainstorm: [], blocked: [], validation: [], done: [],
  };
  for (const t of tasks) grouped[t.stage].push(t);

  const toggle = (stage: TaskStage) => {
    setCollapsed(prev => {
      const next = new Set(prev);
      next.has(stage) ? next.delete(stage) : next.add(stage);
      return next;
    });
  };

  return (
    <div className="h-full overflow-y-auto px-2 py-1">
      {STAGE_ORDER.map(stage => {
        const items = grouped[stage];
        const isEmpty = items.length === 0;
        const isCollapsed = collapsed.has(stage) || isEmpty;

        return (
          <div key={stage} className="mb-1">
            <button
              type="button"
              onClick={() => !isEmpty && toggle(stage)}
              className={`flex items-center gap-2 w-full px-2 py-1.5 text-left rounded hover:bg-zinc-900 cursor-pointer ${isEmpty ? 'opacity-50' : ''}`}
            >
              <span className="text-xs text-zinc-500">{isCollapsed ? '▶' : '▼'}</span>
              <span className={`w-2 h-2 rounded-full ${STAGE_DOT_COLORS[stage]}`} />
              <span className="text-xs font-semibold text-zinc-300 uppercase tracking-wide">
                {STAGE_LABELS[stage]}
              </span>
              <span className="text-xs text-zinc-600">({items.length})</span>
            </button>

            {!isCollapsed && items.map(task => (
              <button
                key={task.id}
                type="button"
                onClick={() => onTaskClick(task)}
                className="flex items-center gap-3 w-full px-4 py-1 text-left text-xs hover:bg-zinc-900 rounded cursor-pointer"
              >
                <span className="text-zinc-600 w-8 shrink-0">#{task.id}</span>
                <span className="text-zinc-200 flex-1 truncate">{task.title}</span>
                <span className={`shrink-0 ${PRIORITY_COLORS[task.priority] ?? 'text-zinc-500'}`}>
                  {PRIORITY_LABELS[task.priority] ?? '?'}
                </span>
                {task.product && (
                  <span className="shrink-0 text-zinc-500 bg-zinc-800 px-1 rounded">{task.product}</span>
                )}
                {task.substep && (
                  <span className="shrink-0 text-sky-400 bg-sky-950 px-1 rounded">
                    {SUBSTEP_LABELS[task.substep]} {SUBSTEP_ORDER.indexOf(task.substep) + 1}/6
                  </span>
                )}
              </button>
            ))}
          </div>
        );
      })}
    </div>
  );
}
```

**Step 2: Create TaskContent.tsx**

Create `web/src/components/planner/TaskContent.tsx`:

```typescript
import type { PlannerTask, TaskStage, TaskView } from '../../lib/types.ts';
import KanbanBoard from './KanbanBoard.tsx';
import ListView from './ListView.tsx';

interface TaskContentProps {
  taskView: TaskView;
  filteredTasks: PlannerTask[];
  tasksByStage: Record<TaskStage, PlannerTask[]>;
  gridSubView: string;
  onGridSubViewChange: (v: any) => void;
  onTaskClick: (task: PlannerTask) => void;
}

export default function TaskContent({ taskView, filteredTasks, tasksByStage, onTaskClick }: TaskContentProps) {
  if (filteredTasks.length === 0) {
    return (
      <div className="flex-1 flex items-center justify-center">
        <span className="text-zinc-500 text-sm">No tasks match filters</span>
      </div>
    );
  }

  switch (taskView) {
    case 'list':
      return <ListView tasks={filteredTasks} onTaskClick={onTaskClick} />;
    case 'kanban':
      return <KanbanBoard tasksByStage={tasksByStage} onTaskClick={onTaskClick} />;
    case 'grid':
      // Stub for grid view — will be implemented in Task 5
      return (
        <div className="flex-1 flex items-center justify-center">
          <span className="text-zinc-500 text-sm">Grid view (coming next)</span>
        </div>
      );
    default:
      return null;
  }
}
```

**Step 3: Wire TaskContent into TaskPanel**

Replace the KanbanBoard rendering in `TaskPanel.tsx` with `TaskContent`, passing the appropriate props. Import `TaskContent` and replace the content area.

**Step 4: Verify build**

Run: `cd web && npx vite build 2>&1 | tail -5`
Expected: Build succeeds.

**Step 5: Commit**

```bash
git add web/src/components/planner/ListView.tsx web/src/components/planner/TaskContent.tsx web/src/components/planner/TaskPanel.tsx
git commit -m "feat: add list view and TaskContent view switcher"
```

---

### Task 5: GridView components (View 4)

**Files:**
- Create: `web/src/components/planner/grid/CompactGrid.tsx`
- Create: `web/src/components/planner/grid/TableView.tsx`
- Create: `web/src/components/planner/grid/GroupedList.tsx`
- Create: `web/src/components/planner/GridView.tsx`
- Modify: `web/src/components/planner/TaskContent.tsx`

**Step 1: Create CompactGrid.tsx**

Create `web/src/components/planner/grid/CompactGrid.tsx`:

```typescript
import type { PlannerTask, TaskStage } from '../../../lib/types.ts';

const DOT_COLORS: Record<TaskStage, string> = {
  backlog: 'bg-zinc-400', brainstorm: 'bg-purple-400', active: 'bg-sky-400',
  blocked: 'bg-red-400', validation: 'bg-amber-400', done: 'bg-green-400',
};
const STAGE_LABELS: Record<TaskStage, string> = {
  backlog: 'Backlog', brainstorm: 'Brainstorm', active: 'Active',
  blocked: 'Blocked', validation: 'Validation', done: 'Done',
};
const PRIORITY_BORDER: Record<number, string> = {
  0: 'border-l-zinc-600', 1: 'border-l-sky-500', 2: 'border-l-amber-500', 3: 'border-l-red-500',
};

interface CompactGridProps {
  tasks: PlannerTask[];
  onTaskClick: (task: PlannerTask) => void;
}

export default function CompactGrid({ tasks, onTaskClick }: CompactGridProps) {
  // Sort by priority desc, then stage.
  const sorted = [...tasks].sort((a, b) => b.priority - a.priority);

  return (
    <div className="grid gap-2 p-2" style={{ gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))' }}>
      {sorted.map(task => (
        <button
          key={task.id}
          type="button"
          onClick={() => onTaskClick(task)}
          className={`text-left bg-zinc-900 rounded-lg border-l-4 ${PRIORITY_BORDER[task.priority] ?? 'border-l-zinc-600'} border border-zinc-800 p-2 hover:bg-zinc-800/80 transition-colors cursor-pointer`}
        >
          <div className="flex items-center justify-between mb-1">
            <span className="text-xs font-medium text-zinc-100 truncate">{task.title}</span>
            <span className="text-[10px] text-zinc-600 shrink-0 ml-1">#{task.id}</span>
          </div>
          <div className="flex items-center gap-1">
            <span className={`w-1.5 h-1.5 rounded-full ${DOT_COLORS[task.stage]}`} />
            <span className="text-[10px] text-zinc-500">{STAGE_LABELS[task.stage]}</span>
          </div>
        </button>
      ))}
    </div>
  );
}
```

**Step 2: Create TableView.tsx**

Create `web/src/components/planner/grid/TableView.tsx`:

```typescript
import { useState } from 'react';
import type { PlannerTask, TaskStage, TaskSubstep } from '../../../lib/types.ts';

const STAGE_LABELS: Record<TaskStage, string> = {
  backlog: 'Backlog', brainstorm: 'Brainstorm', active: 'Active',
  blocked: 'Blocked', validation: 'Validation', done: 'Done',
};
const PRIORITY_LABELS: Record<number, string> = { 0: 'Low', 1: 'Normal', 2: 'High', 3: 'Critical' };
const SUBSTEP_LABELS: Record<TaskSubstep, string> = {
  tdd: 'TDD', implementing: 'Implementing', reviewing: 'Reviewing',
  qa_test: 'QA Test', e2e_test: 'E2E Test', security_review: 'Security Review',
};

type SortKey = 'id' | 'title' | 'stage' | 'priority' | 'product' | 'substep' | 'updated_at';

interface TableViewProps {
  tasks: PlannerTask[];
  onTaskClick: (task: PlannerTask) => void;
}

export default function TableView({ tasks, onTaskClick }: TableViewProps) {
  const [sortKey, setSortKey] = useState<SortKey>('priority');
  const [sortAsc, setSortAsc] = useState(false);

  const handleSort = (key: SortKey) => {
    if (sortKey === key) { setSortAsc(!sortAsc); }
    else { setSortKey(key); setSortAsc(true); }
  };

  const sorted = [...tasks].sort((a, b) => {
    let cmp = 0;
    switch (sortKey) {
      case 'id': cmp = a.id - b.id; break;
      case 'title': cmp = a.title.localeCompare(b.title); break;
      case 'stage': cmp = a.stage.localeCompare(b.stage); break;
      case 'priority': cmp = a.priority - b.priority; break;
      case 'product': cmp = a.product.localeCompare(b.product); break;
      case 'substep': cmp = (a.substep || '').localeCompare(b.substep || ''); break;
    }
    return sortAsc ? cmp : -cmp;
  });

  const arrow = (key: SortKey) => sortKey === key ? (sortAsc ? ' ↑' : ' ↓') : '';

  return (
    <div className="h-full overflow-auto">
      <table className="w-full text-xs">
        <thead className="sticky top-0 bg-zinc-900">
          <tr className="border-b border-zinc-800">
            {([
              ['id', 'ID'], ['title', 'Title'], ['stage', 'Stage'],
              ['priority', 'Priority'], ['product', 'Product'], ['substep', 'Substep'],
            ] as [SortKey, string][]).map(([key, label]) => (
              <th
                key={key}
                onClick={() => handleSort(key)}
                className="px-2 py-1.5 text-left text-zinc-500 font-medium cursor-pointer hover:text-zinc-300 select-none"
              >
                {label}{arrow(key)}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {sorted.map(task => (
            <tr
              key={task.id}
              onClick={() => onTaskClick(task)}
              className="border-b border-zinc-800/50 hover:bg-zinc-900 cursor-pointer"
            >
              <td className="px-2 py-1 text-zinc-500">#{task.id}</td>
              <td className="px-2 py-1 text-zinc-200 truncate max-w-[200px]">{task.title}</td>
              <td className="px-2 py-1 text-zinc-400">{STAGE_LABELS[task.stage]}</td>
              <td className="px-2 py-1 text-zinc-400">{PRIORITY_LABELS[task.priority]}</td>
              <td className="px-2 py-1 text-zinc-500">{task.product || '—'}</td>
              <td className="px-2 py-1 text-zinc-500">{task.substep ? SUBSTEP_LABELS[task.substep] : '—'}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
```

**Step 3: Create GroupedList.tsx**

Create `web/src/components/planner/grid/GroupedList.tsx`. Similar to ListView but with description preview and timestamps per row. Reuse the ListView logic but add extra columns. (The subagent should reference ListView.tsx and extend each row with `task.description` truncated to 60 chars and a relative time for `task.created_at`.)

**Step 4: Create GridView.tsx**

Create `web/src/components/planner/GridView.tsx`:

```typescript
import type { PlannerTask, GridSubView } from '../../lib/types.ts';
import CompactGrid from './grid/CompactGrid.tsx';
import TableView from './grid/TableView.tsx';
import GroupedList from './grid/GroupedList.tsx';

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
      <div className="flex items-center gap-1 px-3 py-1.5 shrink-0">
        {TABS.map(tab => (
          <button
            key={tab.value}
            type="button"
            onClick={() => onSubViewChange(tab.value)}
            className={`px-2 py-0.5 text-xs rounded cursor-pointer ${subView === tab.value ? 'text-zinc-100 border-b-2 border-sky-500' : 'text-zinc-500 hover:text-zinc-300'}`}
          >
            {tab.label}
          </button>
        ))}
      </div>
      <div className="flex-1 overflow-hidden">
        {subView === 'grid' && <CompactGrid tasks={tasks} onTaskClick={onTaskClick} />}
        {subView === 'table' && <TableView tasks={tasks} onTaskClick={onTaskClick} />}
        {subView === 'grouped' && <GroupedList tasks={tasks} onTaskClick={onTaskClick} />}
      </div>
    </div>
  );
}
```

**Step 5: Wire GridView into TaskContent**

Update the `'grid'` case in `TaskContent.tsx` to render `<GridView>` with the correct props.

**Step 6: Verify build**

Run: `cd web && npx vite build 2>&1 | tail -5`
Expected: Build succeeds.

**Step 7: Commit**

```bash
git add web/src/components/planner/grid/ web/src/components/planner/GridView.tsx web/src/components/planner/TaskContent.tsx
git commit -m "feat: add grid view with compact grid, table, and grouped sub-views"
```

---

### Task 6: Filtered KanbanBoard + wiring

**Files:**
- Modify: `web/src/components/planner/KanbanBoard.tsx`
- Modify: `web/src/components/layout/AppShell.tsx`

**Step 1: Update KanbanBoard to filter columns**

Modify `KanbanBoard.tsx` to only render columns for stages that have tasks (when a stage filter is active, show only that stage):

```typescript
// In KanbanBoard, filter STAGES to only show non-empty ones, or all if no stage filter
const visibleStages = STAGES.filter(stage => tasksByStage[stage].length > 0);
// If nothing visible, show all stages (empty board)
const stagesToRender = visibleStages.length > 0 ? visibleStages : STAGES;
```

**Step 2: Update AppShell to compute filtered tasksByStage**

In `AppShell.tsx`, compute a `filteredByStage` that groups `filteredTasks` by stage:

```typescript
const filteredByStage = useMemo(() => {
  const grouped: Record<TaskStage, PlannerTask[]> = {
    backlog: [], brainstorm: [], active: [], blocked: [], validation: [], done: [],
  };
  for (const t of filteredTasks) grouped[t.stage].push(t);
  return grouped;
}, [filteredTasks]);
```

Pass `filteredByStage` instead of `tasksByStage` to `TaskPanel`.

**Step 3: Verify build**

Run: `cd web && npx vite build 2>&1 | tail -5`
Expected: Build succeeds.

**Step 4: Commit**

```bash
git add web/src/components/planner/KanbanBoard.tsx web/src/components/layout/AppShell.tsx
git commit -m "feat: wire filtered tasks through all view modes"
```

---

### Task 7: Chat session backend (Go)

**Files:**
- Modify: `internal/planner/store.go` — add session tables to schema + CRUD methods
- Create: `internal/server/sessions.go` — REST handlers
- Modify: `internal/server/routes.go` — register session routes

**Step 1: Add session schema + methods to store.go**

Append to the `schema` const in `internal/planner/store.go`:

```sql
CREATE TABLE IF NOT EXISTS chat_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'idle',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS chat_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id INTEGER NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TEXT NOT NULL
);
```

Add methods:
- `CreateSession(title string) (ChatSession, error)`
- `ListSessions(limit int) ([]ChatSession, error)` — ORDER BY updated_at DESC LIMIT
- `GetSessionMessages(sessionID int64) ([]ChatMessageRecord, error)`
- `AddMessage(sessionID int64, role, content string) error`
- `UpdateSessionStatus(id int64, status string) error`

Add Go types:
```go
type ChatSession struct {
    ID        int64  `json:"id"`
    Title     string `json:"title"`
    Status    string `json:"status"`
    CreatedAt string `json:"created_at"`
    UpdatedAt string `json:"updated_at"`
}

type ChatMessageRecord struct {
    ID        int64  `json:"id"`
    SessionID int64  `json:"session_id"`
    Role      string `json:"role"`
    Content   string `json:"content"`
    CreatedAt string `json:"created_at"`
}
```

**Step 2: Create sessions.go**

Create `internal/server/sessions.go` with handlers:
- `handleSessionCreate` — `POST /api/sessions` — creates session, returns JSON
- `handleSessionList` — `GET /api/sessions` — returns last 10 sessions
- `handleSessionMessages` — `GET /api/sessions/{id}/messages` — returns messages for session

**Step 3: Register routes**

Add to `routes.go` before the API catch-all:
```go
s.mux.HandleFunc("POST /api/sessions", s.handleSessionCreate)
s.mux.HandleFunc("GET /api/sessions", s.handleSessionList)
s.mux.HandleFunc("GET /api/sessions/{id}/messages", s.handleSessionMessages)
```

**Step 4: Run tests**

Run: `go test ./... 2>&1`
Expected: All tests pass.

**Step 5: Build**

Run: `go build ./cmd/soul/ 2>&1`
Expected: Clean build.

**Step 6: Commit**

```bash
git add internal/planner/store.go internal/server/sessions.go internal/server/routes.go
git commit -m "feat: add chat session backend with REST endpoints"
```

---

### Task 8: Chat session frontend (SessionDrawer + ChatPanel wiring)

**Files:**
- Create: `web/src/hooks/useSessions.ts`
- Create: `web/src/components/chat/SessionDrawer.tsx`
- Create: `web/src/components/chat/ChatNavbar.tsx`
- Modify: `web/src/components/chat/ChatPanel.tsx`

**Step 1: Create useSessions.ts**

Create `web/src/hooks/useSessions.ts`:

```typescript
import { useState, useEffect, useCallback } from 'react';
import type { ChatSession } from '../lib/types.ts';

export function useSessions() {
  const [sessions, setSessions] = useState<ChatSession[]>([]);
  const [activeSessionId, setActiveSessionId] = useState<number | null>(null);

  const fetchSessions = useCallback(async () => {
    try {
      const res = await fetch('/api/sessions');
      if (!res.ok) return;
      const data: ChatSession[] = await res.json();
      setSessions(data);
    } catch { /* ignore */ }
  }, []);

  useEffect(() => { fetchSessions(); }, [fetchSessions]);

  const createSession = useCallback(async () => {
    const res = await fetch('/api/sessions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title: '' }),
    });
    if (!res.ok) throw new Error('Failed to create session');
    const session: ChatSession = await res.json();
    setSessions(prev => [session, ...prev].slice(0, 10));
    setActiveSessionId(session.id);
    return session;
  }, []);

  const switchSession = useCallback((id: number) => {
    setActiveSessionId(id);
  }, []);

  return { sessions, activeSessionId, createSession, switchSession, fetchSessions };
}
```

**Step 2: Create ChatNavbar.tsx**

Create `web/src/components/chat/ChatNavbar.tsx`:

```typescript
interface ChatNavbarProps {
  onToggleDrawer: () => void;
  onCollapse: () => void;
  canCollapse: boolean;
}

export default function ChatNavbar({ onToggleDrawer, onCollapse, canCollapse }: ChatNavbarProps) {
  return (
    <div className="h-10 border-b border-zinc-800 flex items-center px-3 shrink-0 gap-2">
      <button type="button" onClick={onToggleDrawer} className="text-zinc-500 hover:text-zinc-300 text-sm cursor-pointer">&#9776;</button>
      <span className="text-sm font-bold text-zinc-100">&#9670; Soul Chat</span>
      <div className="flex-1" />
      {canCollapse && (
        <button type="button" onClick={onCollapse} className="text-zinc-500 hover:text-zinc-300 text-sm cursor-pointer" title="Collapse">&#8722;</button>
      )}
    </div>
  );
}
```

**Step 3: Create SessionDrawer.tsx**

Create `web/src/components/chat/SessionDrawer.tsx`:

```typescript
import type { ChatSession } from '../../lib/types.ts';

const STATUS_ICONS: Record<string, string> = {
  running: '●',
  idle: '○',
  completed: '✓',
};
const STATUS_COLORS: Record<string, string> = {
  running: 'text-green-400',
  idle: 'text-zinc-400',
  completed: 'text-zinc-600',
};

interface SessionDrawerProps {
  sessions: ChatSession[];
  activeSessionId: number | null;
  onSelect: (id: number) => void;
  onNew: () => void;
  onClose: () => void;
}

function relativeTime(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  const days = Math.floor(hrs / 24);
  return `${days}d ago`;
}

export default function SessionDrawer({ sessions, activeSessionId, onSelect, onNew, onClose }: SessionDrawerProps) {
  return (
    <>
      {/* Backdrop */}
      <div className="absolute inset-0 z-10" onClick={onClose} />

      {/* Drawer */}
      <div className="absolute left-0 top-10 bottom-0 w-52 bg-zinc-900 border-r border-zinc-700 z-20 flex flex-col animate-slide-in">
        <div className="flex items-center justify-between px-3 py-2 border-b border-zinc-800">
          <span className="text-xs font-semibold text-zinc-300">Sessions</span>
          <button type="button" onClick={onClose} className="text-zinc-500 hover:text-zinc-300 text-sm cursor-pointer">&times;</button>
        </div>

        <button
          type="button"
          onClick={onNew}
          className="mx-2 mt-2 px-2 py-1.5 text-xs font-medium rounded bg-sky-600 hover:bg-sky-500 text-white cursor-pointer"
        >
          + New Chat
        </button>

        <div className="flex-1 overflow-y-auto mt-2">
          {sessions.map(s => (
            <button
              key={s.id}
              type="button"
              onClick={() => { onSelect(s.id); onClose(); }}
              className={`w-full text-left px-3 py-2 text-xs hover:bg-zinc-800 cursor-pointer ${s.id === activeSessionId ? 'bg-zinc-800' : ''}`}
            >
              <div className="flex items-center gap-1.5">
                <span className={`${STATUS_COLORS[s.status]}`}>{STATUS_ICONS[s.status]}</span>
                <span className="text-zinc-300 truncate flex-1">{s.title || 'Untitled'}</span>
              </div>
              <div className="text-[10px] text-zinc-600 ml-4 mt-0.5">{relativeTime(s.updated_at)}</div>
            </button>
          ))}
        </div>
      </div>
    </>
  );
}
```

**Step 4: Wire into ChatPanel.tsx**

Update `ChatPanel.tsx` to use `ChatNavbar`, `SessionDrawer`, and `useSessions`. Toggle drawer state, pass active session ID to ChatView.

**Step 5: Verify build**

Run: `cd web && npx vite build 2>&1 | tail -5`
Expected: Build succeeds.

**Step 6: Commit**

```bash
git add web/src/hooks/useSessions.ts web/src/components/chat/ChatNavbar.tsx web/src/components/chat/SessionDrawer.tsx web/src/components/chat/ChatPanel.tsx
git commit -m "feat: add session drawer with session switching"
```

---

### Task 9: Build, deploy, and E2E test

**Files:** None new — integration testing.

**Step 1: Build frontend**

Run: `cd web && npx vite build 2>&1 | tail -5`
Expected: Build succeeds.

**Step 2: Build Go binary**

Run: `cd /home/rishav/soul && go build ./cmd/soul/ && go install ./cmd/soul/`
Expected: Clean build.

**Step 3: Restart server**

```bash
pkill -f "soul serve"
sleep 1
(unset ANTHROPIC_API_KEY; SOUL_HOST=0.0.0.0 /home/rishav/go/bin/soul serve --compliance-bin /home/rishav/.soul/products/compliance-go > /tmp/soul-server.log 2>&1) &
sleep 2
tail -5 /tmp/soul-server.log
```
Expected: "Planner store opened" + "Soul server listening on 0.0.0.0:3000"

**Step 4: Test REST API**

```bash
# Create test tasks
curl -s -X POST localhost:3000/api/tasks -H 'Content-Type: application/json' -d '{"title":"Test 1","priority":2}'
curl -s -X POST localhost:3000/api/tasks -H 'Content-Type: application/json' -d '{"title":"Test 2","priority":3,"product":"compliance"}'
# Create a session
curl -s -X POST localhost:3000/api/sessions -H 'Content-Type: application/json' -d '{"title":"Test session"}'
curl -s localhost:3000/api/sessions
```
Expected: Tasks and sessions created/listed correctly.

**Step 5: Browser E2E test**

Open browser, verify:
1. Two-panel layout with no global header
2. Chat panel navbar with ☰ and collapse button
3. Task panel navbar with view mode buttons (≡, ⊞, ⊟) and collapse button
4. Filter bar with Stage/Priority/Product dropdowns
5. Click view mode buttons to switch between list/kanban/grid
6. Collapse task panel → task rail with stage dots + counts
7. Collapse chat panel → chat rail with ◆ icon
8. Drag resize divider between panels
9. Click ☰ → session drawer slides open
10. Clean up test tasks

**Step 6: Commit final state**

```bash
git add -A
git commit -m "feat: task manager UI redesign - collapsible panels, view modes, filters, sessions"
```
