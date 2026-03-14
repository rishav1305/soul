# Frontend — Routing + Tasks UI Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add client-side routing with React Router, a Dashboard page, and a Tasks Kanban page — making the autonomous execution pipeline visible and controllable from the browser.

**Architecture:** React Router v7 with a shared layout (header + nav). The existing Shell component is refactored into a layout wrapper. Chat stays at `/chat` (default route), Dashboard at `/`, and Tasks Kanban at `/tasks` with task detail at `/tasks/:id`. All task data fetched via REST from `/api/tasks/*` (proxied to the tasks server). Real-time task updates received via the existing WebSocket connection.

**Tech Stack:** React 19, React Router v7, TypeScript 5.9, Tailwind v4 (zinc/obsidian palette), Vite 7

**Deferred:**
- Product pages (`/p/:product`) — no product system built yet
- Service worker route caching updates
- Visual regression testing

---

## File Structure

| File | Responsibility |
|------|----------------|
| `web/src/main.tsx` | MODIFY — add RouterProvider |
| `web/src/router.tsx` | CREATE — route definitions with lazy loading |
| `web/src/layouts/AppLayout.tsx` | CREATE — shared header + nav + outlet |
| `web/src/pages/ChatPage.tsx` | CREATE — wraps existing Shell chat logic |
| `web/src/pages/DashboardPage.tsx` | CREATE — overview with task summary + system health |
| `web/src/pages/TasksPage.tsx` | CREATE — Kanban board |
| `web/src/pages/TaskDetailPage.tsx` | CREATE — single task view with activity timeline |
| `web/src/components/Shell.tsx` | MODIFY — extract chat-specific JSX into ChatPage |
| `web/src/components/TaskCard.tsx` | CREATE — task card for Kanban columns |
| `web/src/components/ActivityTimeline.tsx` | CREATE — structured activity log display |
| `web/src/hooks/useTasks.ts` | CREATE — REST API hook for tasks CRUD |
| `web/src/hooks/useTaskEvents.ts` | CREATE — WebSocket listener for real-time task updates |
| `web/src/lib/api.ts` | CREATE — fetch wrapper for REST API calls |
| `web/src/lib/types.ts` | MODIFY — add Task/Activity types (manual, not generated) |

---

## Chunk 1: Routing Foundation + API Layer

### Task 1: Install React Router and Create API Client

**Files:**
- Modify: `web/package.json`
- Create: `web/src/lib/api.ts`

- [ ] **Step 1: Install react-router**

Run: `cd /home/rishav/soul-v2/web && npm install react-router@^7`

- [ ] **Step 2: Create API client**

Create `web/src/lib/api.ts`:

```typescript
const BASE = '';

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(body.error || `HTTP ${res.status}`);
  }
  if (res.status === 204) return undefined as T;
  return res.json();
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'POST', body: body ? JSON.stringify(body) : undefined }),
  patch: <T>(path: string, body: unknown) =>
    request<T>(path, { method: 'PATCH', body: JSON.stringify(body) }),
  delete: (path: string) => request<void>(path, { method: 'DELETE' }),
};
```

- [ ] **Step 3: Verify TypeScript compiles**

Run: `cd /home/rishav/soul-v2/web && npx tsc --noEmit`
Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add web/package.json web/package-lock.json web/src/lib/api.ts
git commit -m "feat: install react-router and create API client"
```

---

### Task 2: Add Task Types

**Files:**
- Modify: `web/src/lib/types.ts`

Since types.ts is auto-generated, we add task types at the bottom with a manual section marker.

- [ ] **Step 1: Add task types to types.ts**

Append to the end of `web/src/lib/types.ts`:

```typescript
// --- Manual types (not auto-generated) ---

/** tasks */
export interface Task {
  id: number;
  title: string;
  description: string;
  stage: TaskStage;
  workflow: string;
  product: string;
  metadata: string;
  createdAt: string;
  updatedAt: string;
}

export type TaskStage = 'backlog' | 'active' | 'validation' | 'done' | 'blocked';

/** tasks */
export interface TaskActivity {
  id: number;
  taskId: number;
  eventType: string;
  data: string;
  createdAt: string;
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /home/rishav/soul-v2/web && npx tsc --noEmit`

- [ ] **Step 3: Commit**

```bash
git add web/src/lib/types.ts
git commit -m "feat: add Task and TaskActivity types"
```

---

### Task 3: Create useTasks Hook

**Files:**
- Create: `web/src/hooks/useTasks.ts`

- [ ] **Step 1: Create useTasks hook**

```typescript
import { useState, useEffect, useCallback } from 'react';
import type { Task, TaskStage } from '../lib/types';
import { api } from '../lib/api';

interface UseTasksReturn {
  tasks: Task[];
  loading: boolean;
  error: string | null;
  refresh: () => void;
  createTask: (title: string, description?: string) => Promise<Task>;
  updateTask: (id: number, fields: Partial<Pick<Task, 'title' | 'description' | 'stage'>>) => Promise<Task>;
  deleteTask: (id: number) => Promise<void>;
  startTask: (id: number) => Promise<void>;
  stopTask: (id: number) => Promise<void>;
}

export function useTasks(): UseTasksReturn {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(() => {
    setLoading(true);
    api.get<Task[]>('/api/tasks')
      .then(data => {
        setTasks(data);
        setError(null);
      })
      .catch(err => setError(err.message))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => { refresh(); }, [refresh]);

  const createTask = useCallback(async (title: string, description = '') => {
    const task = await api.post<Task>('/api/tasks', { title, description });
    setTasks(prev => [task, ...prev]);
    return task;
  }, []);

  const updateTask = useCallback(async (id: number, fields: Partial<Pick<Task, 'title' | 'description' | 'stage'>>) => {
    const task = await api.patch<Task>(`/api/tasks/${id}`, fields);
    setTasks(prev => prev.map(t => t.id === id ? task : t));
    return task;
  }, []);

  const deleteTask = useCallback(async (id: number) => {
    await api.delete(`/api/tasks/${id}`);
    setTasks(prev => prev.filter(t => t.id !== id));
  }, []);

  const startTask = useCallback(async (id: number) => {
    await api.post<{ status: string }>(`/api/tasks/${id}/start`);
    refresh();
  }, [refresh]);

  const stopTask = useCallback(async (id: number) => {
    await api.post<{ status: string }>(`/api/tasks/${id}/stop`);
    refresh();
  }, [refresh]);

  return { tasks, loading, error, refresh, createTask, updateTask, deleteTask, startTask, stopTask };
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /home/rishav/soul-v2/web && npx tsc --noEmit`

- [ ] **Step 3: Commit**

```bash
git add web/src/hooks/useTasks.ts
git commit -m "feat: add useTasks hook for REST API integration"
```

---

### Task 4: Create useTaskEvents Hook

**Files:**
- Create: `web/src/hooks/useTaskEvents.ts`

Listens for real-time task events via the existing WebSocket connection (task.created, task.updated, task.started SSE events are relayed through the WS hub).

- [ ] **Step 1: Create useTaskEvents hook**

```typescript
import { useEffect, useCallback, useRef } from 'react';
import type { Task } from '../lib/types';

type TaskEventHandler = (eventType: string, task: Task) => void;

export function useTaskEvents(onEvent: TaskEventHandler): void {
  const handlerRef = useRef(onEvent);
  handlerRef.current = onEvent;

  useEffect(() => {
    const handler = (event: MessageEvent) => {
      try {
        const msg = JSON.parse(event.data);
        if (msg.type && msg.type.startsWith('task.') && msg.data) {
          const task = typeof msg.data === 'string' ? JSON.parse(msg.data) : msg.data;
          handlerRef.current(msg.type, task);
        }
      } catch {
        // Not a task event or invalid JSON — ignore.
      }
    };

    // The WebSocket connection is managed by useWebSocket in useChat.
    // We listen on the global message event bus instead.
    window.addEventListener('ws:task-event', handler as EventListener);
    return () => window.removeEventListener('ws:task-event', handler as EventListener);
  }, []);
}
```

Note: This requires a small update to the WebSocket hook to dispatch task events on a global event bus. We'll do that in Task 7 when wiring everything together. For now the hook is structurally complete.

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /home/rishav/soul-v2/web && npx tsc --noEmit`

- [ ] **Step 3: Commit**

```bash
git add web/src/hooks/useTaskEvents.ts
git commit -m "feat: add useTaskEvents hook for real-time task updates"
```

---

## Chunk 2: Router + Layout + Pages

### Task 5: Create AppLayout

**Files:**
- Create: `web/src/layouts/AppLayout.tsx`

The shared layout provides the header with navigation links and an Outlet for child routes.

- [ ] **Step 1: Create AppLayout**

```typescript
import { NavLink, Outlet } from 'react-router';
import { ConnectionBanner } from '../components/ConnectionBanner';
import type { ConnectionState } from '../lib/types';

function navLinkClass({ isActive }: { isActive: boolean }) {
  return `px-3 py-1 text-sm rounded-md transition-colors ${
    isActive
      ? 'bg-elevated text-fg'
      : 'text-fg-muted hover:text-fg hover:bg-elevated/50'
  }`;
}

export function AppLayout() {
  return (
    <div data-testid="app-layout" className="h-screen bg-deep text-fg flex flex-col noise">
      <header className="glass flex items-center justify-between px-4 h-11 shrink-0">
        <div className="flex items-center gap-3">
          <span className="text-soul text-xl drop-shadow-[0_0_8px_rgba(232,168,73,0.4)]" aria-hidden="true">&#9670;</span>
          <h1 className="text-base font-semibold text-fg tracking-tight">Soul</h1>
          <nav className="hidden sm:flex items-center gap-1 ml-4" data-testid="main-nav">
            <NavLink to="/" end className={navLinkClass}>Dashboard</NavLink>
            <NavLink to="/chat" className={navLinkClass}>Chat</NavLink>
            <NavLink to="/tasks" className={navLinkClass}>Tasks</NavLink>
          </nav>
        </div>
      </header>
      <div className="flex-1 min-h-0">
        <Outlet />
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /home/rishav/soul-v2/web && npx tsc --noEmit`

- [ ] **Step 3: Commit**

```bash
git add web/src/layouts/AppLayout.tsx
git commit -m "feat: add AppLayout with navigation header"
```

---

### Task 6: Create ChatPage

**Files:**
- Create: `web/src/pages/ChatPage.tsx`

Extracts the chat-specific functionality from Shell into a dedicated page component. Uses the existing `useChat` hook.

- [ ] **Step 1: Create ChatPage**

```typescript
import { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import { useChat } from '../hooks/useChat';
import { useSwipeDrawer } from '../hooks/useSwipeDrawer';
import { MessageList } from '../components/MessageList';
import { ChatInput } from '../components/ChatInput';
import type { ChatInputHandle } from '../components/ChatInput';
import { SessionList } from '../components/SessionList';
import { ConnectionBanner } from '../components/ConnectionBanner';
import { SearchBar } from '../components/SearchBar';

export function ChatPage() {
  const {
    messages,
    isStreaming,
    status,
    authError,
    reconnectAttempt,
    sendMessage,
    stopGeneration,
    editAndResend,
    retryMessage,
    reauth,
    sessions,
    currentSessionID,
    createSession,
    switchSession,
    deleteSession,
    renameSession,
  } = useChat();

  const { isOpen, close, toggle, handlers } = useSwipeDrawer();
  const inputRef = useRef<ChatInputHandle>(null);

  const [searchOpen, setSearchOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');

  const closeSearch = useCallback(() => {
    setSearchOpen(false);
    setSearchQuery('');
  }, []);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'f') {
        e.preventDefault();
        setSearchOpen(true);
      }
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        inputRef.current?.focus();
      }
      if ((e.metaKey || e.ctrlKey) && e.shiftKey && e.key === 'N') {
        e.preventDefault();
        createSession();
      }
      if (e.key === 'Escape' && searchOpen) {
        closeSearch();
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [searchOpen, closeSearch, createSession]);

  const matchCount = useMemo(() => {
    if (!searchQuery) return 0;
    const escaped = searchQuery.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    const re = new RegExp(escaped, 'gi');
    return messages.reduce((n, msg) => n + (msg.content.match(re)?.length ?? 0), 0);
  }, [messages, searchQuery]);

  const isDisabled = status !== 'connected';

  const handleSwitch = (id: string) => {
    switchSession(id);
    close();
  };

  const handleCreate = () => {
    createSession();
    close();
  };

  return (
    <div data-testid="chat-page" className="h-full flex" {...handlers}>
      <ConnectionBanner status={status} reconnectAttempt={reconnectAttempt} />

      {/* Backdrop — mobile only */}
      {isOpen && (
        <div
          data-testid="sidebar-backdrop"
          className="fixed inset-0 bg-black/60 z-30 md:hidden"
          onClick={close}
        />
      )}

      {/* Sidebar */}
      <div
        data-testid="sidebar-drawer"
        className={`
          fixed inset-y-0 left-0 z-40 w-64
          transform transition-transform duration-200 ease-out
          md:relative md:translate-x-0 md:transition-none
          ${isOpen ? 'translate-x-0' : '-translate-x-full'}
        `}
      >
        <SessionList
          sessions={sessions}
          activeSessionID={currentSessionID}
          onCreate={handleCreate}
          onSwitch={handleSwitch}
          onDelete={deleteSession}
          onRename={renameSession}
        />
      </div>

      {/* Mobile sidebar toggle */}
      <button
        data-testid="sidebar-toggle"
        type="button"
        onClick={toggle}
        className="fixed bottom-4 left-4 z-50 md:hidden p-2 rounded-full bg-elevated text-fg-muted hover:text-fg shadow-lg"
        aria-label="Toggle sessions"
      >
        <svg width="18" height="18" viewBox="0 0 20 20" fill="none">
          <path d="M3 5h14M3 10h14M3 15h14" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </svg>
      </button>

      {/* Chat area */}
      <div className="flex-1 flex flex-col min-w-0">
        {searchOpen && (
          <SearchBar query={searchQuery} onChange={setSearchQuery} onClose={closeSearch} matchCount={matchCount} />
        )}
        <MessageList messages={messages} isStreaming={isStreaming} onSend={sendMessage} onEdit={editAndResend} onRetry={retryMessage} searchQuery={searchQuery} />
        <ChatInput ref={inputRef} onSend={sendMessage} onStop={stopGeneration} disabled={isDisabled} isStreaming={isStreaming} />
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /home/rishav/soul-v2/web && npx tsc --noEmit`

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/ChatPage.tsx
git commit -m "feat: add ChatPage extracted from Shell"
```

---

### Task 7: Create DashboardPage

**Files:**
- Create: `web/src/pages/DashboardPage.tsx`

Shows system overview: task counts by stage, recent activity, and system health.

- [ ] **Step 1: Create DashboardPage**

```typescript
import { useEffect, useState } from 'react';
import { Link } from 'react-router';
import { useTasks } from '../hooks/useTasks';
import type { TaskStage } from '../lib/types';

const STAGE_COLORS: Record<TaskStage, string> = {
  backlog: 'bg-zinc-600',
  active: 'bg-blue-500',
  validation: 'bg-yellow-500',
  done: 'bg-green-500',
  blocked: 'bg-red-500',
};

const STAGE_LABELS: Record<TaskStage, string> = {
  backlog: 'Backlog',
  active: 'Active',
  validation: 'Validation',
  done: 'Done',
  blocked: 'Blocked',
};

export function DashboardPage() {
  const { tasks, loading, error } = useTasks();

  const counts = (Object.keys(STAGE_LABELS) as TaskStage[]).map(stage => ({
    stage,
    label: STAGE_LABELS[stage],
    color: STAGE_COLORS[stage],
    count: tasks.filter(t => t.stage === stage).length,
  }));

  return (
    <div data-testid="dashboard-page" className="h-full overflow-y-auto p-6">
      <div className="max-w-4xl mx-auto space-y-8">
        <div>
          <h2 className="text-xl font-semibold text-fg mb-1">Dashboard</h2>
          <p className="text-sm text-fg-muted">System overview</p>
        </div>

        {error && (
          <div data-testid="dashboard-error" className="p-3 rounded-lg bg-red-900/30 text-red-400 text-sm">
            {error}
          </div>
        )}

        {/* Task counts */}
        <div className="grid grid-cols-2 sm:grid-cols-5 gap-3">
          {counts.map(({ stage, label, color, count }) => (
            <Link
              key={stage}
              to={`/tasks?stage=${stage}`}
              data-testid={`count-${stage}`}
              className="flex flex-col items-center p-4 rounded-lg bg-surface hover:bg-elevated transition-colors"
            >
              <span className={`inline-block w-2 h-2 rounded-full ${color} mb-2`} />
              <span className="text-2xl font-bold text-fg">{loading ? '-' : count}</span>
              <span className="text-xs text-fg-muted mt-1">{label}</span>
            </Link>
          ))}
        </div>

        {/* Recent tasks */}
        <div>
          <h3 className="text-sm font-medium text-fg-muted mb-3">Recent Tasks</h3>
          {loading ? (
            <div className="text-fg-muted text-sm">Loading...</div>
          ) : tasks.length === 0 ? (
            <div className="text-fg-muted text-sm">No tasks yet. <Link to="/tasks" className="text-soul hover:underline">Create one</Link>.</div>
          ) : (
            <div className="space-y-2">
              {tasks.slice(0, 10).map(task => (
                <Link
                  key={task.id}
                  to={`/tasks/${task.id}`}
                  data-testid={`recent-task-${task.id}`}
                  className="flex items-center justify-between p-3 rounded-lg bg-surface hover:bg-elevated transition-colors"
                >
                  <div className="flex items-center gap-3 min-w-0">
                    <span className={`shrink-0 w-2 h-2 rounded-full ${STAGE_COLORS[task.stage as TaskStage] || 'bg-zinc-600'}`} />
                    <span className="text-sm text-fg truncate">{task.title}</span>
                  </div>
                  <span className="text-xs text-fg-muted shrink-0 ml-3">{task.stage}</span>
                </Link>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /home/rishav/soul-v2/web && npx tsc --noEmit`

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/DashboardPage.tsx
git commit -m "feat: add DashboardPage with task counts and recent tasks"
```

---

## Chunk 3: Tasks Kanban + Detail + Wiring

### Task 8: Create TaskCard Component

**Files:**
- Create: `web/src/components/TaskCard.tsx`

Reusable card for Kanban columns and lists.

- [ ] **Step 1: Create TaskCard**

```typescript
import { Link } from 'react-router';
import type { Task } from '../lib/types';
import { formatRelativeTime } from '../lib/utils';

const WORKFLOW_BADGE: Record<string, string> = {
  micro: 'bg-green-900/40 text-green-400',
  quick: 'bg-blue-900/40 text-blue-400',
  full: 'bg-purple-900/40 text-purple-400',
};

interface TaskCardProps {
  task: Task;
  onStart?: (id: number) => void;
  onStop?: (id: number) => void;
}

export function TaskCard({ task, onStart, onStop }: TaskCardProps) {
  const isActive = task.stage === 'active';

  return (
    <div data-testid={`task-card-${task.id}`} className="p-3 rounded-lg bg-surface hover:bg-elevated transition-colors group">
      <div className="flex items-start justify-between gap-2">
        <Link to={`/tasks/${task.id}`} className="text-sm text-fg hover:text-soul transition-colors min-w-0 truncate flex-1">
          {task.title}
        </Link>
        {task.workflow && (
          <span className={`shrink-0 px-1.5 py-0.5 text-[10px] font-medium rounded ${WORKFLOW_BADGE[task.workflow] || 'bg-zinc-800 text-zinc-400'}`}>
            {task.workflow}
          </span>
        )}
      </div>

      {task.description && (
        <p className="text-xs text-fg-muted mt-1 line-clamp-2">{task.description}</p>
      )}

      <div className="flex items-center justify-between mt-2">
        <span className="text-[10px] text-fg-muted">{formatRelativeTime(task.updatedAt)}</span>
        <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
          {(task.stage === 'backlog' || task.stage === 'blocked') && onStart && (
            <button
              data-testid={`start-task-${task.id}`}
              onClick={() => onStart(task.id)}
              className="px-2 py-0.5 text-[10px] rounded bg-green-900/40 text-green-400 hover:bg-green-900/60 transition-colors"
            >
              Start
            </button>
          )}
          {isActive && onStop && (
            <button
              data-testid={`stop-task-${task.id}`}
              onClick={() => onStop(task.id)}
              className="px-2 py-0.5 text-[10px] rounded bg-red-900/40 text-red-400 hover:bg-red-900/60 transition-colors"
            >
              Stop
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /home/rishav/soul-v2/web && npx tsc --noEmit`

- [ ] **Step 3: Commit**

```bash
git add web/src/components/TaskCard.tsx
git commit -m "feat: add TaskCard component for Kanban display"
```

---

### Task 9: Create TasksPage (Kanban)

**Files:**
- Create: `web/src/pages/TasksPage.tsx`

Kanban board with columns: Backlog, Active, Validation, Done, Blocked. Includes a "New Task" form.

- [ ] **Step 1: Create TasksPage**

```typescript
import { useState } from 'react';
import { useTasks } from '../hooks/useTasks';
import { TaskCard } from '../components/TaskCard';
import type { TaskStage } from '../lib/types';

const COLUMNS: { stage: TaskStage; label: string; color: string }[] = [
  { stage: 'backlog', label: 'Backlog', color: 'border-zinc-600' },
  { stage: 'active', label: 'Active', color: 'border-blue-500' },
  { stage: 'validation', label: 'Validation', color: 'border-yellow-500' },
  { stage: 'done', label: 'Done', color: 'border-green-500' },
  { stage: 'blocked', label: 'Blocked', color: 'border-red-500' },
];

export function TasksPage() {
  const { tasks, loading, error, createTask, startTask, stopTask, refresh } = useTasks();
  const [showCreate, setShowCreate] = useState(false);
  const [newTitle, setNewTitle] = useState('');
  const [newDesc, setNewDesc] = useState('');
  const [creating, setCreating] = useState(false);

  const handleCreate = async () => {
    if (!newTitle.trim()) return;
    setCreating(true);
    try {
      await createTask(newTitle.trim(), newDesc.trim());
      setNewTitle('');
      setNewDesc('');
      setShowCreate(false);
    } catch {
      // Error handled by useTasks
    } finally {
      setCreating(false);
    }
  };

  return (
    <div data-testid="tasks-page" className="h-full flex flex-col">
      {/* Toolbar */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-elevated">
        <h2 className="text-lg font-semibold text-fg">Tasks</h2>
        <button
          data-testid="new-task-btn"
          onClick={() => setShowCreate(!showCreate)}
          className="px-3 py-1.5 text-sm rounded-lg bg-soul/20 text-soul hover:bg-soul/30 transition-colors"
        >
          + New Task
        </button>
      </div>

      {/* Create form */}
      {showCreate && (
        <div data-testid="create-task-form" className="px-4 py-3 border-b border-elevated bg-surface">
          <input
            data-testid="new-task-title"
            value={newTitle}
            onChange={e => setNewTitle(e.target.value)}
            placeholder="Task title"
            className="w-full bg-elevated rounded px-3 py-2 text-sm text-fg placeholder:text-fg-muted outline-none focus:ring-1 focus:ring-soul/50"
            onKeyDown={e => e.key === 'Enter' && handleCreate()}
            autoFocus
          />
          <textarea
            data-testid="new-task-desc"
            value={newDesc}
            onChange={e => setNewDesc(e.target.value)}
            placeholder="Description (optional)"
            className="w-full bg-elevated rounded px-3 py-2 text-sm text-fg placeholder:text-fg-muted outline-none focus:ring-1 focus:ring-soul/50 mt-2 resize-none"
            rows={2}
          />
          <div className="flex justify-end gap-2 mt-2">
            <button onClick={() => setShowCreate(false)} className="px-3 py-1 text-xs text-fg-muted hover:text-fg transition-colors">
              Cancel
            </button>
            <button
              data-testid="create-task-submit"
              onClick={handleCreate}
              disabled={creating || !newTitle.trim()}
              className="px-3 py-1 text-xs rounded bg-soul text-deep font-medium hover:bg-soul/90 disabled:opacity-50 transition-colors"
            >
              {creating ? 'Creating...' : 'Create'}
            </button>
          </div>
        </div>
      )}

      {error && (
        <div className="px-4 py-2 text-sm text-red-400">{error}</div>
      )}

      {/* Kanban columns */}
      <div className="flex-1 overflow-x-auto p-4">
        <div className="flex gap-4 min-w-max h-full">
          {COLUMNS.map(({ stage, label, color }) => {
            const columnTasks = tasks.filter(t => t.stage === stage);
            return (
              <div key={stage} data-testid={`column-${stage}`} className={`w-64 flex flex-col border-t-2 ${color} rounded-lg bg-deep/50`}>
                <div className="flex items-center justify-between px-3 py-2">
                  <span className="text-xs font-medium text-fg-muted uppercase tracking-wider">{label}</span>
                  <span className="text-xs text-fg-muted">{loading ? '-' : columnTasks.length}</span>
                </div>
                <div className="flex-1 overflow-y-auto px-2 pb-2 space-y-2">
                  {columnTasks.map(task => (
                    <TaskCard key={task.id} task={task} onStart={startTask} onStop={stopTask} />
                  ))}
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /home/rishav/soul-v2/web && npx tsc --noEmit`

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/TasksPage.tsx
git commit -m "feat: add TasksPage with Kanban board and task creation"
```

---

### Task 10: Create ActivityTimeline and TaskDetailPage

**Files:**
- Create: `web/src/components/ActivityTimeline.tsx`
- Create: `web/src/pages/TaskDetailPage.tsx`

- [ ] **Step 1: Create ActivityTimeline**

```typescript
import { formatRelativeTime } from '../lib/utils';
import type { TaskActivity } from '../lib/types';

interface ActivityTimelineProps {
  activities: TaskActivity[];
}

const EVENT_ICONS: Record<string, string> = {
  'task.created': 'text-green-400',
  'task.started': 'text-blue-400',
  'task.stopped': 'text-red-400',
  'task.blocked': 'text-red-400',
  'executor.classify': 'text-purple-400',
  'executor.worktree': 'text-cyan-400',
  'executor.agent_start': 'text-blue-400',
  'executor.agent_done': 'text-green-400',
  'executor.verify_l1': 'text-yellow-400',
  'executor.commit': 'text-green-400',
  'executor.complete': 'text-green-400',
  'agent.tool_call': 'text-fg-muted',
};

export function ActivityTimeline({ activities }: ActivityTimelineProps) {
  if (activities.length === 0) {
    return <p className="text-sm text-fg-muted">No activity yet.</p>;
  }

  return (
    <div data-testid="activity-timeline" className="space-y-3">
      {activities.map(act => {
        const color = EVENT_ICONS[act.eventType] || 'text-fg-muted';
        let detail = '';
        try {
          const parsed = JSON.parse(act.data);
          detail = Object.entries(parsed)
            .map(([k, v]) => `${k}: ${typeof v === 'object' ? JSON.stringify(v) : v}`)
            .join(', ');
        } catch {
          detail = act.data;
        }

        return (
          <div key={act.id} data-testid={`activity-${act.id}`} className="flex gap-3 text-sm">
            <div className="shrink-0 mt-1">
              <span className={`inline-block w-2 h-2 rounded-full ${color.replace('text-', 'bg-')}`} />
            </div>
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <span className={`font-mono text-xs ${color}`}>{act.eventType}</span>
                <span className="text-[10px] text-fg-muted">{formatRelativeTime(act.createdAt)}</span>
              </div>
              {detail && (
                <p className="text-xs text-fg-muted mt-0.5 break-all">{detail}</p>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
}
```

- [ ] **Step 2: Create TaskDetailPage**

```typescript
import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router';
import { api } from '../lib/api';
import type { Task, TaskActivity, TaskStage } from '../lib/types';
import { ActivityTimeline } from '../components/ActivityTimeline';

const STAGE_COLORS: Record<TaskStage, string> = {
  backlog: 'bg-zinc-600',
  active: 'bg-blue-500',
  validation: 'bg-yellow-500',
  done: 'bg-green-500',
  blocked: 'bg-red-500',
};

export function TaskDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [task, setTask] = useState<Task | null>(null);
  const [activities, setActivities] = useState<TaskActivity[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    setLoading(true);
    Promise.all([
      api.get<Task>(`/api/tasks/${id}`),
      api.get<TaskActivity[]>(`/api/tasks/${id}/activity`),
    ])
      .then(([t, acts]) => {
        setTask(t);
        setActivities(acts);
        setError(null);
      })
      .catch(err => setError(err.message))
      .finally(() => setLoading(false));
  }, [id]);

  const handleStart = async () => {
    if (!id) return;
    try {
      await api.post(`/api/tasks/${id}/start`);
      const t = await api.get<Task>(`/api/tasks/${id}`);
      setTask(t);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Start failed');
    }
  };

  const handleStop = async () => {
    if (!id) return;
    try {
      await api.post(`/api/tasks/${id}/stop`);
      const t = await api.get<Task>(`/api/tasks/${id}`);
      setTask(t);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Stop failed');
    }
  };

  if (loading) {
    return <div className="p-6 text-fg-muted">Loading...</div>;
  }

  if (error || !task) {
    return (
      <div className="p-6">
        <Link to="/tasks" className="text-sm text-soul hover:underline">&larr; Back to tasks</Link>
        <div className="mt-4 text-red-400">{error || 'Task not found'}</div>
      </div>
    );
  }

  return (
    <div data-testid="task-detail-page" className="h-full overflow-y-auto p-6">
      <div className="max-w-3xl mx-auto">
        <Link to="/tasks" className="text-sm text-soul hover:underline">&larr; Back to tasks</Link>

        <div className="mt-4 space-y-6">
          {/* Header */}
          <div className="flex items-start justify-between gap-4">
            <div>
              <h2 className="text-xl font-semibold text-fg">{task.title}</h2>
              {task.description && (
                <p className="text-sm text-fg-muted mt-1">{task.description}</p>
              )}
            </div>
            <div className="flex items-center gap-2">
              <span className={`px-2 py-1 text-xs rounded-full text-white ${STAGE_COLORS[task.stage as TaskStage] || 'bg-zinc-600'}`}>
                {task.stage}
              </span>
              {task.workflow && (
                <span className="px-2 py-1 text-xs rounded-full bg-elevated text-fg-muted">{task.workflow}</span>
              )}
            </div>
          </div>

          {/* Actions */}
          <div className="flex gap-2">
            {(task.stage === 'backlog' || task.stage === 'blocked') && (
              <button
                data-testid="detail-start"
                onClick={handleStart}
                className="px-4 py-2 text-sm rounded-lg bg-green-700 text-white hover:bg-green-600 transition-colors"
              >
                Start Execution
              </button>
            )}
            {task.stage === 'active' && (
              <button
                data-testid="detail-stop"
                onClick={handleStop}
                className="px-4 py-2 text-sm rounded-lg bg-red-700 text-white hover:bg-red-600 transition-colors"
              >
                Stop Execution
              </button>
            )}
          </div>

          {/* Activity */}
          <div>
            <h3 className="text-sm font-medium text-fg-muted mb-3">Activity Log</h3>
            <ActivityTimeline activities={activities} />
          </div>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 3: Verify TypeScript compiles**

Run: `cd /home/rishav/soul-v2/web && npx tsc --noEmit`

- [ ] **Step 4: Commit**

```bash
git add web/src/components/ActivityTimeline.tsx web/src/pages/TaskDetailPage.tsx
git commit -m "feat: add TaskDetailPage with activity timeline"
```

---

### Task 11: Create Router and Wire main.tsx

**Files:**
- Create: `web/src/router.tsx`
- Modify: `web/src/main.tsx`

- [ ] **Step 1: Create router**

```typescript
import { createBrowserRouter } from 'react-router';
import { AppLayout } from './layouts/AppLayout';

export const router = createBrowserRouter([
  {
    element: <AppLayout />,
    children: [
      {
        index: true,
        lazy: () => import('./pages/DashboardPage').then(m => ({ Component: m.DashboardPage })),
      },
      {
        path: 'chat',
        lazy: () => import('./pages/ChatPage').then(m => ({ Component: m.ChatPage })),
      },
      {
        path: 'tasks',
        lazy: () => import('./pages/TasksPage').then(m => ({ Component: m.TasksPage })),
      },
      {
        path: 'tasks/:id',
        lazy: () => import('./pages/TaskDetailPage').then(m => ({ Component: m.TaskDetailPage })),
      },
    ],
  },
]);
```

- [ ] **Step 2: Update main.tsx**

Replace the contents of `web/src/main.tsx`:

```typescript
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { RouterProvider } from 'react-router';
import { ErrorBoundary } from './components/ErrorBoundary';
import { router } from './router';
import './app.css';

const root = document.getElementById('root');
if (root) {
  createRoot(root).render(
    <StrictMode>
      <ErrorBoundary>
        <RouterProvider router={router} />
      </ErrorBoundary>
    </StrictMode>,
  );
}

if ('serviceWorker' in navigator) {
  navigator.serviceWorker.register('/sw.js').catch(() => {
    // Service worker registration failed — app works without it
  });
}
```

- [ ] **Step 3: Verify TypeScript compiles**

Run: `cd /home/rishav/soul-v2/web && npx tsc --noEmit`

- [ ] **Step 4: Build frontend**

Run: `cd /home/rishav/soul-v2/web && npx vite build`
Expected: builds successfully with code splitting

- [ ] **Step 5: Commit**

```bash
git add web/src/router.tsx web/src/main.tsx
git commit -m "feat: add React Router with lazy-loaded pages"
```

---

### Task 12: Build, Verify, Update Docs

**Files:**
- Modify: `CLAUDE.md`
- Modify: `web/vite.config.ts` (optional — if code splitting needs updating)

- [ ] **Step 1: Build full project**

Run: `cd /home/rishav/soul-v2 && make build`
Expected: both binaries build, frontend builds with new page chunks

- [ ] **Step 2: Verify go vet passes**

Run: `go vet ./...`

- [ ] **Step 3: Update CLAUDE.md frontend section**

Add to Architecture section under `web/src/`:
```
  layouts/                    Layout components (AppLayout)
  pages/                      Route-level page components
    ChatPage.tsx              Chat interface (extracted from Shell)
    DashboardPage.tsx         System overview — task counts, recent tasks
    TasksPage.tsx             Kanban board — Backlog/Active/Validation/Done/Blocked
    TaskDetailPage.tsx        Single task view with activity timeline
```

- [ ] **Step 4: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md with frontend routing architecture"
```

---

## Key Design Decisions

1. **Lazy loading** — All pages use React Router `lazy()` for code splitting. The chat page (largest) loads on demand, keeping the initial bundle small for dashboard views.

2. **No Shell refactor** — The Shell component is preserved as-is (it still works standalone). ChatPage is a clean extraction that imports the same hooks/components. Shell can be deleted later once routing is fully validated.

3. **REST for tasks, WS for real-time** — Tasks CRUD uses REST API (simpler, cacheable). Real-time updates (task started, stage changed) come via the existing WebSocket connection's SSE relay.

4. **Minimal routing** — React Router v7 with a flat route structure. No nested layouts beyond AppLayout. Product pages deferred.

5. **Obsidian theme continuity** — All new pages use the existing `deep/surface/elevated/soul` color tokens. No new CSS dependencies.

6. **data-testid everywhere** — Every interactive element has a data-testid attribute per project conventions.
