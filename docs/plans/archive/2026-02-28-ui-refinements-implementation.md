# UI Refinements Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Six UI refinements — rename navbar, redesign InputBar with two-section toolbar, bigger animated Soul logo, loading splash screen, fix TaskPanel navbar buttons, and redesign TaskRail with colored count squares.

**Architecture:** All changes are frontend CSS + TSX component edits except one small Go endpoint (`GET /api/models`). No database changes. The InputBar toolbar controls (model selector, chat type, tool permissions, file upload) are wired to local state and passed via `onSend` options — the backend WebSocket handler already accepts `json.RawMessage` for the `data` field so extra fields are ignored gracefully.

**Tech Stack:** React 19, TypeScript, Tailwind CSS v4, Go 1.24

---

### Task 1: CSS Additions — Enhanced Animations + Fade-Out

**Files:**
- Modify: `web/src/styles/globals.css:145-167`

**Step 1: Update float keyframe amplitude and add fade-out**

In `web/src/styles/globals.css`, make these changes:

1. Replace the `float` keyframe (line 145-148) to increase amplitude from 4px to 16px:
```css
@keyframes float {
  0%, 100% { transform: translateY(0px); }
  50% { transform: translateY(-16px); }
}
```

2. Add a new `fade-out` keyframe and utility class after `.animate-float` (line 155):
```css
@keyframes fade-out {
  from { opacity: 1; }
  to { opacity: 0; }
}

.animate-fade-out { animation: fade-out 0.5s ease-out both; }
```

**Step 2: Build to verify no CSS errors**

Run: `cd /home/rishav/soul/web && npx vite build 2>&1 | tail -5`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add web/src/styles/globals.css
git commit -m "style: enhanced float animation + fade-out keyframe"
```

---

### Task 2: Backend — Add GET /api/models Endpoint

**Files:**
- Modify: `internal/server/routes.go`

**Step 1: Add route registration and handler**

In `internal/server/routes.go`, add route registration in `registerRoutes()` after the sessions block (after line 35):

```go
	// Model list endpoint.
	s.mux.HandleFunc("GET /api/models", s.handleModelsList)
```

Then add the handler function at the end of the file (after `handleToolExecute`):

```go
// modelInfo is returned by GET /api/models.
type modelInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// handleModelsList returns available AI models.
func (s *Server) handleModelsList(w http.ResponseWriter, r *http.Request) {
	models := []modelInfo{
		{ID: s.cfg.Model, Name: friendlyModelName(s.cfg.Model), Description: "Default model"},
	}
	writeJSON(w, http.StatusOK, models)
}

func friendlyModelName(model string) string {
	switch {
	case strings.Contains(model, "opus"):
		return "Opus"
	case strings.Contains(model, "sonnet"):
		return "Sonnet"
	case strings.Contains(model, "haiku"):
		return "Haiku"
	default:
		return model
	}
}
```

**Step 2: Run Go tests**

Run: `cd /home/rishav/soul && go test ./internal/server/ -v -count=1 2>&1 | tail -10`
Expected: All tests pass

**Step 3: Verify endpoint works**

Run: `cd /home/rishav/soul && go build -o soul ./cmd/soul && curl -s http://localhost:3000/api/models | python3 -m json.tool`
(Note: only test after server is running in Task 8)

**Step 4: Commit**

```bash
git add internal/server/routes.go
git commit -m "feat: add GET /api/models endpoint"
```

---

### Task 3: ChatNavbar — Rename + SVG Collapse Icon

**Files:**
- Modify: `web/src/components/chat/ChatNavbar.tsx`

**Step 1: Rewrite ChatNavbar.tsx**

Replace the entire file content with:

```tsx
interface ChatNavbarProps {
  onToggleDrawer: () => void;
  onCollapse: () => void;
  canCollapse: boolean;
}

export default function ChatNavbar({ onToggleDrawer, onCollapse, canCollapse }: ChatNavbarProps) {
  return (
    <div className="glass flex items-center gap-2 h-11 px-4 shrink-0">
      <button
        type="button"
        onClick={onToggleDrawer}
        className="text-fg-muted hover:text-fg text-lg cursor-pointer"
        title="Sessions"
      >
        &#9776;
      </button>

      <span className="font-display text-sm font-semibold text-fg flex items-center gap-1.5">
        <span className="text-soul">&#9670;</span> Soul
      </span>

      <div className="flex-1" />

      <button
        type="button"
        onClick={onCollapse}
        disabled={!canCollapse}
        className="w-7 h-7 flex items-center justify-center rounded hover:bg-elevated text-fg-muted hover:text-fg disabled:opacity-20 disabled:cursor-not-allowed transition-colors cursor-pointer"
        title={canCollapse ? 'Collapse chat' : 'Cannot collapse — task panel is collapsed'}
      >
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
          <path d="M10 3L5 8l5 5" />
        </svg>
      </button>
    </div>
  );
}
```

Key changes:
- "Soul Chat" → "Soul"
- `[−]` text → chevron-left SVG in consistent `w-7 h-7` button

**Step 2: Build to verify**

Run: `cd /home/rishav/soul/web && npx vite build 2>&1 | tail -3`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add web/src/components/chat/ChatNavbar.tsx
git commit -m "feat: rename 'Soul Chat' to 'Soul' + SVG collapse icon"
```

---

### Task 4: TaskPanel Navbar — Fix Button + SVG Icons

**Files:**
- Modify: `web/src/components/planner/TaskPanel.tsx:82-137`

**Step 1: Replace the navbar section**

In `TaskPanel.tsx`, replace the navbar `<div>` block (lines 82-137) with:

```tsx
      {/* Navbar */}
      <div className="glass flex items-center gap-2 px-4 shrink-0 h-11">
        <span className="font-display text-sm font-semibold text-fg">Tasks</span>

        {/* View mode buttons */}
        <div className="flex items-center gap-0.5 ml-2">
          {VIEW_BUTTONS.map(({ view, icon, title }) => (
            <button
              key={view}
              type="button"
              onClick={() => setTaskView(view)}
              title={title}
              className={`w-7 h-7 flex items-center justify-center rounded text-sm cursor-pointer transition-colors ${
                taskView === view
                  ? 'bg-overlay text-fg'
                  : 'text-fg-muted hover:text-fg-secondary hover:bg-elevated'
              }`}
            >
              {icon}
            </button>
          ))}
        </div>

        <div className="flex-1" />

        {/* Reset width */}
        {panelWidth !== null && (
          <button
            type="button"
            onClick={() => setPanelWidth(null)}
            className="w-7 h-7 flex items-center justify-center rounded hover:bg-elevated text-fg-muted hover:text-fg transition-colors cursor-pointer"
            title="Reset to auto width"
          >
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
              <path d="M2 8a6 6 0 0 1 10.3-4.2" />
              <path d="M14 2v4h-4" />
              <path d="M14 8a6 6 0 0 1-10.3 4.2" />
              <path d="M2 14v-4h4" />
            </svg>
          </button>
        )}

        {/* Collapse */}
        <button
          type="button"
          onClick={onCollapse}
          disabled={!canCollapse}
          className="w-7 h-7 flex items-center justify-center rounded hover:bg-elevated text-fg-muted hover:text-fg disabled:opacity-20 disabled:cursor-not-allowed transition-colors cursor-pointer"
          title={canCollapse ? 'Collapse tasks' : 'Cannot collapse — chat panel is collapsed'}
        >
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
            <path d="M6 3l5 5-5 5" />
          </svg>
        </button>

        {/* New Task */}
        <button
          type="button"
          onClick={() => setShowNewForm(true)}
          className="bg-soul hover:bg-soul/80 text-deep font-display font-semibold text-xs rounded-md px-3 h-7 whitespace-nowrap shrink-0 flex items-center transition-colors cursor-pointer"
        >
          + New Task
        </button>
      </div>
```

Key changes:
- `↻` → SVG refresh/reset icon in `w-7 h-7` button with hover bg
- `×` → SVG chevron-right in `w-7 h-7` button with hover bg
- `+ New Task` gets `h-7 whitespace-nowrap shrink-0 flex items-center` to fix height growth

**Step 2: Build to verify**

Run: `cd /home/rishav/soul/web && npx vite build 2>&1 | tail -3`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add web/src/components/planner/TaskPanel.tsx
git commit -m "fix: TaskPanel navbar — SVG icons + fixed-height New Task button"
```

---

### Task 5: TaskRail — Colored Count Squares + New Task Button

**Files:**
- Modify: `web/src/components/layout/TaskRail.tsx` (full rewrite)
- Modify: `web/src/components/layout/AppShell.tsx:117-118` (wire onNewTask)

**Step 1: Rewrite TaskRail.tsx**

Replace the entire file:

```tsx
import { useState } from 'react';
import type { TaskStage, PlannerTask } from '../../lib/types.ts';
import NewTaskForm from '../planner/NewTaskForm.tsx';

const STAGES: TaskStage[] = ['backlog', 'brainstorm', 'active', 'blocked', 'validation', 'done'];

const STAGE_BG: Record<TaskStage, string> = {
  backlog: 'bg-stage-backlog',
  brainstorm: 'bg-stage-brainstorm',
  active: 'bg-stage-active',
  blocked: 'bg-stage-blocked',
  validation: 'bg-stage-validation',
  done: 'bg-stage-done',
};

interface TaskRailProps {
  tasksByStage: Record<TaskStage, PlannerTask[]>;
  onExpand: () => void;
  onNewTask: (title: string, description: string, priority: number, product: string) => Promise<void>;
}

export default function TaskRail({ tasksByStage, onExpand, onNewTask }: TaskRailProps) {
  const [showNewForm, setShowNewForm] = useState(false);

  return (
    <>
      <div className="w-10 h-full bg-surface border-l border-border-subtle flex flex-col items-center py-3 gap-2 shrink-0">
        {/* Expand icon */}
        <button
          type="button"
          onClick={onExpand}
          className="w-7 h-7 flex items-center justify-center rounded hover:bg-elevated text-fg-muted hover:text-fg transition-colors cursor-pointer"
          title="Expand tasks"
        >
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
            <path d="M6 3l5 5-5 5" />
          </svg>
        </button>

        {/* New task button */}
        <button
          type="button"
          onClick={(e) => { e.stopPropagation(); setShowNewForm(true); }}
          className="w-7 h-7 rounded bg-soul/80 hover:bg-soul text-deep flex items-center justify-center transition-colors cursor-pointer font-bold text-sm"
          title="New task"
        >
          +
        </button>

        <div className="h-1" />

        {/* Stage count boxes */}
        {STAGES.map((stage) => {
          const count = tasksByStage[stage].length;
          return (
            <div
              key={stage}
              className={`w-7 h-7 rounded-sm flex items-center justify-center ${STAGE_BG[stage]} ${count === 0 ? 'opacity-30' : ''} transition-opacity`}
              title={`${stage}: ${count}`}
            >
              <span className="text-deep font-mono text-[11px] font-bold leading-none">
                {count}
              </span>
            </div>
          );
        })}
      </div>

      {showNewForm && (
        <NewTaskForm
          onClose={() => setShowNewForm(false)}
          onCreate={async (title, desc, priority, product) => {
            await onNewTask(title, desc, priority, product);
            setShowNewForm(false);
          }}
        />
      )}
    </>
  );
}
```

Key changes:
- Dots → `w-7 h-7 rounded-sm` colored boxes with count text
- Added `+` new task button with gold background
- Expand icon in proper `w-7 h-7` button
- `showNewForm` state for the NewTaskForm modal
- Component is no longer a single `<button>` — split into `<>` fragment

**Step 2: Wire onNewTask in AppShell.tsx**

In `web/src/components/layout/AppShell.tsx`, change line 118 from:
```tsx
        <TaskRail tasksByStage={allByStage} onExpand={handleTaskExpand} />
```
to:
```tsx
        <TaskRail
          tasksByStage={allByStage}
          onExpand={handleTaskExpand}
          onNewTask={async (title, desc, priority, product) => {
            await planner.createTask(title, desc, priority, product);
          }}
        />
```

**Step 3: Build to verify**

Run: `cd /home/rishav/soul/web && npx vite build 2>&1 | tail -3`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add web/src/components/layout/TaskRail.tsx web/src/components/layout/AppShell.tsx
git commit -m "feat: TaskRail — colored count squares + new task button"
```

---

### Task 6: ChatView — Bigger Logo + Glow Animation

**Files:**
- Modify: `web/src/components/chat/ChatView.tsx:20-27`

**Step 1: Update the empty state section**

In `ChatView.tsx`, replace the empty state block (lines 20-27):

```tsx
          {messages.length === 0 && (
            <div className="flex items-center justify-center h-full min-h-[200px]">
              <div className="text-center">
                <div className="relative inline-block">
                  {/* Glow ring behind diamond */}
                  <div className="absolute inset-0 -m-8 bg-soul/15 rounded-full blur-3xl animate-soul-pulse" />
                  <div className="relative text-8xl text-soul animate-float">&#9670;</div>
                </div>
                <p className="font-display text-xl text-fg-secondary mt-6">How can I help you?</p>
              </div>
            </div>
          )}
```

Key changes:
- `text-5xl` → `text-8xl`
- Added glow ring: `absolute` div with `bg-soul/15 blur-3xl animate-soul-pulse`
- `text-lg` → `text-xl` for greeting
- `mb-3` → `mt-6` for more spacing

**Step 2: Build to verify**

Run: `cd /home/rishav/soul/web && npx vite build 2>&1 | tail -3`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add web/src/components/chat/ChatView.tsx
git commit -m "feat: bigger Soul logo with glow animation in empty state"
```

---

### Task 7: SplashScreen — Loading Page

**Files:**
- Create: `web/src/components/layout/SplashScreen.tsx`
- Modify: `web/src/App.tsx`

**Step 1: Create SplashScreen component**

Create `web/src/components/layout/SplashScreen.tsx`:

```tsx
import { useState, useEffect } from 'react';

interface SplashScreenProps {
  ready: boolean;
}

export default function SplashScreen({ ready }: SplashScreenProps) {
  const [visible, setVisible] = useState(true);

  useEffect(() => {
    if (ready) {
      // Start fade-out, then unmount
      const timer = setTimeout(() => setVisible(false), 600);
      return () => clearTimeout(timer);
    }
  }, [ready]);

  if (!visible) return null;

  return (
    <div
      className={`fixed inset-0 z-50 bg-deep noise flex flex-col items-center justify-center ${
        ready ? 'animate-fade-out' : ''
      }`}
    >
      {/* Glow ring */}
      <div className="relative">
        <div className="absolute inset-0 -m-16 bg-soul/15 rounded-full blur-3xl animate-soul-pulse" />
        <div className="relative text-9xl text-soul animate-float">&#9670;</div>
      </div>

      {/* Title */}
      <p className="font-display text-2xl tracking-[0.3em] text-fg-secondary mt-10 uppercase">
        Soul
      </p>

      {/* Loading dots */}
      <div className="flex gap-2 mt-8">
        {[0, 1, 2].map((i) => (
          <span
            key={i}
            className="w-1.5 h-1.5 rounded-full bg-soul animate-soul-pulse"
            style={{ animationDelay: `${i * 200}ms` }}
          />
        ))}
      </div>
    </div>
  );
}
```

**Step 2: Wire SplashScreen in App.tsx**

Replace `web/src/App.tsx` with:

```tsx
import AppShell from './components/layout/AppShell.tsx';
import SplashScreen from './components/layout/SplashScreen.tsx';
import { WebSocketContext, useWebSocketProvider } from './hooks/useWebSocket.ts';

export default function App() {
  const ws = useWebSocketProvider();
  return (
    <WebSocketContext.Provider value={ws}>
      <SplashScreen ready={ws.connected} />
      <AppShell />
    </WebSocketContext.Provider>
  );
}
```

**Step 3: Build to verify**

Run: `cd /home/rishav/soul/web && npx vite build 2>&1 | tail -3`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add web/src/components/layout/SplashScreen.tsx web/src/App.tsx
git commit -m "feat: loading splash screen with animated Soul diamond"
```

---

### Task 8: InputBar — Two-Section Redesign with Toolbar

**Files:**
- Modify: `web/src/components/chat/InputBar.tsx` (full rewrite)
- Modify: `web/src/components/chat/ChatView.tsx:40` (update onSend signature)
- Modify: `web/src/hooks/useChat.ts:144-162` (accept options in sendMessage)
- Modify: `web/src/lib/types.ts` (add SendOptions type)

**Step 1: Add SendOptions type**

In `web/src/lib/types.ts`, add at the end:

```ts
export interface SendOptions {
  model?: string;
  chatType?: string;
  disabledTools?: string[];
}
```

**Step 2: Update useChat to accept options**

In `web/src/hooks/useChat.ts`, change the `sendMessage` callback (lines 144-162):

Replace:
```ts
  const sendMessage = useCallback(
    (content: string) => {
      const userMessage: ChatMessage = {
        id: uuid(),
        role: 'user',
        content,
        timestamp: new Date(),
      };
      setMessages((prev) => [...prev, userMessage]);
      setIsStreaming(true);

      send({
        type: 'chat.message',
        session_id: sessionIdRef.current,
        content,
      });
    },
    [send],
  );
```

With:
```ts
  const sendMessage = useCallback(
    (content: string, options?: SendOptions) => {
      const userMessage: ChatMessage = {
        id: uuid(),
        role: 'user',
        content,
        timestamp: new Date(),
      };
      setMessages((prev) => [...prev, userMessage]);
      setIsStreaming(true);

      send({
        type: 'chat.message',
        session_id: sessionIdRef.current,
        content,
        data: options,
      });
    },
    [send],
  );
```

Also add the import at the top of `useChat.ts`:
```ts
import type { ChatMessage, ToolCallMessage, WSMessage, SendOptions } from '../lib/types.ts';
```

And update the return type:
```ts
  return { messages, sendMessage, isStreaming, connected };
```
(sendMessage signature is now `(content: string, options?: SendOptions) => void`)

**Step 3: Rewrite InputBar.tsx**

Replace the entire `web/src/components/chat/InputBar.tsx`:

```tsx
import { useState, useRef, useEffect, useCallback } from 'react';
import type { SendOptions } from '../../lib/types.ts';

interface ToolInfo {
  name: string;
  qualified_name: string;
  product: string;
  description: string;
  requires_approval: boolean;
}

interface ModelInfo {
  id: string;
  name: string;
  description: string;
}

interface InputBarProps {
  onSend: (message: string, options?: SendOptions) => void;
  disabled: boolean;
}

const CHAT_TYPES = ['Chat', 'Code', 'Planner'] as const;

export default function InputBar({ onSend, disabled }: InputBarProps) {
  const [value, setValue] = useState('');
  const [model, setModel] = useState<string>('');
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [chatType, setChatType] = useState<string>('Chat');
  const [tools, setTools] = useState<ToolInfo[]>([]);
  const [disabledTools, setDisabledTools] = useState<Set<string>>(new Set());
  const [showToolPopover, setShowToolPopover] = useState(false);
  const [files, setFiles] = useState<File[]>([]);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const imageInputRef = useRef<HTMLInputElement>(null);
  const toolPopoverRef = useRef<HTMLDivElement>(null);

  // Fetch models on mount
  useEffect(() => {
    fetch('/api/models')
      .then((r) => r.json())
      .then((data: ModelInfo[]) => {
        setModels(data);
        if (data.length > 0) setModel(data[0].id);
      })
      .catch(() => {});
  }, []);

  // Fetch tools on mount
  useEffect(() => {
    fetch('/api/tools')
      .then((r) => r.json())
      .then((data: ToolInfo[]) => {
        if (Array.isArray(data)) setTools(data);
      })
      .catch(() => {});
  }, []);

  // Auto-focus
  useEffect(() => {
    textareaRef.current?.focus();
  }, []);

  // Close tool popover on outside click
  useEffect(() => {
    if (!showToolPopover) return;
    const handler = (e: MouseEvent) => {
      if (toolPopoverRef.current && !toolPopoverRef.current.contains(e.target as Node)) {
        setShowToolPopover(false);
      }
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [showToolPopover]);

  const handleSend = useCallback(() => {
    const trimmed = value.trim();
    if (!trimmed || disabled) return;
    const options: SendOptions = {};
    if (model) options.model = model;
    if (chatType !== 'Chat') options.chatType = chatType.toLowerCase();
    if (disabledTools.size > 0) options.disabledTools = Array.from(disabledTools);
    onSend(trimmed, Object.keys(options).length > 0 ? options : undefined);
    setValue('');
    setFiles([]);
    if (textareaRef.current) textareaRef.current.style.height = 'auto';
  }, [value, disabled, model, chatType, disabledTools, onSend]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        handleSend();
      }
    },
    [handleSend],
  );

  const handleChange = useCallback((e: React.ChangeEvent<HTMLTextAreaElement>) => {
    setValue(e.target.value);
    const el = e.target;
    el.style.height = 'auto';
    el.style.height = `${Math.min(el.scrollHeight, 200)}px`;
  }, []);

  const toggleTool = useCallback((qualifiedName: string) => {
    setDisabledTools((prev) => {
      const next = new Set(prev);
      if (next.has(qualifiedName)) {
        next.delete(qualifiedName);
      } else {
        next.add(qualifiedName);
      }
      return next;
    });
  }, []);

  const handleFileSelect = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const selected = e.target.files;
    if (selected) setFiles((prev) => [...prev, ...Array.from(selected)]);
    e.target.value = '';
  }, []);

  const removeFile = useCallback((index: number) => {
    setFiles((prev) => prev.filter((_, i) => i !== index));
  }, []);

  const selectedModel = models.find((m) => m.id === model);

  return (
    <div className="px-5 py-4">
      <div className="glass rounded-2xl overflow-hidden">
        {/* File attachments */}
        {files.length > 0 && (
          <div className="flex flex-wrap gap-2 px-4 pt-3">
            {files.map((f, i) => (
              <span
                key={i}
                className="inline-flex items-center gap-1.5 bg-elevated rounded-lg px-2.5 py-1 text-xs text-fg-secondary"
              >
                {f.type.startsWith('image/') ? (
                  <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5"><rect x="2" y="2" width="12" height="12" rx="2" /><circle cx="6" cy="6.5" r="1.5" /><path d="M2 11l3-3 2 2 3-3 4 4" /></svg>
                ) : (
                  <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5"><path d="M9 2H4a1 1 0 0 0-1 1v10a1 1 0 0 0 1 1h8a1 1 0 0 0 1-1V6L9 2z" /><path d="M9 2v4h4" /></svg>
                )}
                <span className="max-w-[120px] truncate">{f.name}</span>
                <button
                  type="button"
                  onClick={() => removeFile(i)}
                  className="text-fg-muted hover:text-fg ml-0.5 cursor-pointer"
                >
                  ×
                </button>
              </span>
            ))}
          </div>
        )}

        {/* Textarea */}
        <textarea
          ref={textareaRef}
          value={value}
          onChange={handleChange}
          onKeyDown={handleKeyDown}
          disabled={disabled}
          placeholder="Message Soul..."
          rows={1}
          className="w-full bg-transparent px-4 pt-3 pb-2 text-fg placeholder:text-fg-muted font-body resize-none overflow-y-hidden focus:outline-none disabled:opacity-50 disabled:cursor-not-allowed"
        />

        {/* Toolbar */}
        <div className="flex items-center gap-1.5 px-3 py-2 border-t border-border-subtle">
          {/* Model selector */}
          {models.length > 0 && (
            <div className="relative">
              <select
                value={model}
                onChange={(e) => setModel(e.target.value)}
                className="soul-select pr-6 text-[11px]"
              >
                {models.map((m) => (
                  <option key={m.id} value={m.id}>
                    ◆ {m.name}
                  </option>
                ))}
              </select>
            </div>
          )}

          {/* Chat type */}
          <select
            value={chatType}
            onChange={(e) => setChatType(e.target.value)}
            className="soul-select pr-6 text-[11px]"
          >
            {CHAT_TYPES.map((t) => (
              <option key={t} value={t}>{t}</option>
            ))}
          </select>

          {/* Tool permissions */}
          <div className="relative" ref={toolPopoverRef}>
            <button
              type="button"
              onClick={() => setShowToolPopover(!showToolPopover)}
              className="relative w-7 h-7 flex items-center justify-center rounded hover:bg-elevated text-fg-muted hover:text-fg transition-colors cursor-pointer"
              title="Tool permissions"
            >
              <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                <path d="M6 3h4l2 2v2l-5 5-5-5V5l2-2z" />
                <circle cx="8" cy="6" r="1" />
              </svg>
              {disabledTools.size > 0 && (
                <span className="absolute -top-1 -right-1 w-4 h-4 bg-stage-blocked text-deep text-[9px] font-bold rounded-full flex items-center justify-center">
                  {disabledTools.size}
                </span>
              )}
            </button>

            {showToolPopover && tools.length > 0 && (
              <div className="absolute bottom-full left-0 mb-2 w-64 bg-surface border border-border-default rounded-xl shadow-xl z-40 py-2 max-h-60 overflow-y-auto">
                <div className="px-3 py-1.5 text-[10px] font-display uppercase tracking-widest text-fg-muted border-b border-border-subtle mb-1">
                  Tool Permissions
                </div>
                {tools.map((tool) => {
                  const isDisabled = disabledTools.has(tool.qualified_name);
                  return (
                    <button
                      key={tool.qualified_name}
                      type="button"
                      onClick={() => toggleTool(tool.qualified_name)}
                      className="w-full flex items-center gap-2 px-3 py-1.5 hover:bg-elevated transition-colors text-left cursor-pointer"
                    >
                      <span className={`w-3.5 h-3.5 rounded border flex items-center justify-center shrink-0 transition-colors ${
                        isDisabled
                          ? 'border-fg-muted bg-transparent'
                          : 'border-soul bg-soul'
                      }`}>
                        {!isDisabled && (
                          <svg width="8" height="8" viewBox="0 0 10 10" fill="none" stroke="var(--color-deep)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <path d="M2 5l2.5 2.5L8 3" />
                          </svg>
                        )}
                      </span>
                      <span className="flex-1 min-w-0">
                        <span className={`text-xs block truncate ${isDisabled ? 'text-fg-muted' : 'text-fg'}`}>
                          {tool.name}
                        </span>
                        <span className="text-[10px] text-fg-muted block truncate">{tool.product}</span>
                      </span>
                      {tool.requires_approval && (
                        <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="var(--color-stage-validation)" strokeWidth="1.5" className="shrink-0">
                          <path d="M8 1.5l6 3v4c0 3.5-2.5 5.5-6 7-3.5-1.5-6-3.5-6-7v-4l6-3z" />
                        </svg>
                      )}
                    </button>
                  );
                })}
              </div>
            )}
          </div>

          {/* File attach */}
          <button
            type="button"
            onClick={() => fileInputRef.current?.click()}
            className="w-7 h-7 flex items-center justify-center rounded hover:bg-elevated text-fg-muted hover:text-fg transition-colors cursor-pointer"
            title="Attach file"
          >
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
              <path d="M13.5 7.5l-5.8 5.8a3.5 3.5 0 0 1-5-5l5.8-5.8a2.3 2.3 0 0 1 3.3 3.3L6 11.6a1.2 1.2 0 0 1-1.7-1.7L9.5 4.7" />
            </svg>
          </button>
          <input ref={fileInputRef} type="file" multiple className="hidden" onChange={handleFileSelect} />

          {/* Image attach */}
          <button
            type="button"
            onClick={() => imageInputRef.current?.click()}
            className="w-7 h-7 flex items-center justify-center rounded hover:bg-elevated text-fg-muted hover:text-fg transition-colors cursor-pointer"
            title="Attach image"
          >
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
              <rect x="2" y="2" width="12" height="12" rx="2" />
              <circle cx="5.5" cy="5.5" r="1.5" />
              <path d="M14 10l-3-3-7 7" />
            </svg>
          </button>
          <input ref={imageInputRef} type="file" accept="image/*" multiple className="hidden" onChange={handleFileSelect} />

          {/* Spacer */}
          <div className="flex-1" />

          {/* Send button */}
          <button
            onClick={handleSend}
            disabled={disabled || !value.trim()}
            className="w-8 h-8 bg-soul text-deep rounded-full flex items-center justify-center hover:bg-soul/85 disabled:opacity-20 disabled:cursor-not-allowed transition-colors shrink-0 cursor-pointer"
            title="Send"
          >
            <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor">
              <path d="M8 3l-1 1 3.3 3.3H3v1.4h7.3L7 12l1 1 5-5-5-5z" />
            </svg>
          </button>
        </div>
      </div>
    </div>
  );
}
```

**Step 4: Update ChatView.tsx onSend signature**

In `web/src/components/chat/ChatView.tsx` line 40, the existing code:
```tsx
      <InputBar onSend={sendMessage} disabled={isStreaming} />
```
remains unchanged — `sendMessage` now accepts optional `SendOptions` as second arg, and `InputBar.onSend` matches.

**Step 5: Build to verify**

Run: `cd /home/rishav/soul/web && npx vite build 2>&1 | tail -5`
Expected: Build succeeds

**Step 6: Commit**

```bash
git add web/src/components/chat/InputBar.tsx web/src/hooks/useChat.ts web/src/lib/types.ts
git commit -m "feat: InputBar two-section redesign — model selector, chat type, tool permissions, file attach"
```

---

### Task 9: Build, Deploy, and Browser E2E Test

**Files:** None (integration testing)

**Step 1: Build frontend**

Run: `cd /home/rishav/soul/web && npx vite build 2>&1 | tail -5`
Expected: Build succeeds with module count + asset sizes

**Step 2: Build Go binary**

Run: `cd /home/rishav/soul && go build -o soul ./cmd/soul`
Expected: Clean build, no errors

**Step 3: Run Go tests**

Run: `cd /home/rishav/soul && go test ./... -count=1 2>&1 | tail -10`
Expected: All tests pass

**Step 4: Restart server**

```bash
pkill -f "soul serve" 2>/dev/null; sleep 1
fuser -k 3000/tcp 2>/dev/null; sleep 2
cd /home/rishav/soul && ./soul serve --host 0.0.0.0 &
sleep 3
curl -s http://localhost:3000/api/health
```
Expected: `{"status":"ok"}`

**Step 5: Test new /api/models endpoint**

Run: `curl -s http://localhost:3000/api/models | python3 -m json.tool`
Expected: Array with at least one model object

**Step 6: Create test tasks**

```bash
for t in "API migration:High priority task:2:platform" "Update docs:Low priority:0:docs"; do
  IFS=: read title desc pri prod <<< "$t"
  curl -s -X POST http://localhost:3000/api/tasks \
    -H 'Content-Type: application/json' \
    -d "{\"title\":\"$title\",\"description\":\"$desc\",\"priority\":$pri,\"product\":\"$prod\"}" > /dev/null
done
curl -s -X POST "http://localhost:3000/api/tasks/$(curl -s http://localhost:3000/api/tasks | python3 -c 'import sys,json;print(json.load(sys.stdin)[0]["id"])')/move" \
  -H 'Content-Type: application/json' -d '{"stage":"active","comment":"testing"}' > /dev/null
echo "Test tasks created"
```

**Step 7: Browser E2E testing checklist**

Open `http://192.168.0.128:3000/` in browser and verify:

1. **Splash screen** — Animated Soul diamond with glow + "SOUL" text + loading dots, fades out once connected
2. **ChatNavbar** — Shows "Soul" (not "Soul Chat"), chevron-left collapse icon
3. **Empty state** — Large diamond (`text-8xl`) with pulsing glow ring behind it, "How can I help you?" below
4. **InputBar** — Two-section layout: textarea on top (no scrollbar), toolbar below with:
   - Model selector dropdown (shows "◆ Sonnet")
   - Chat type dropdown (Chat/Code/Planner)
   - Tool permissions button (wrench icon, opens popover if tools connected)
   - File attach button (paperclip icon)
   - Image attach button (image icon)
   - Round gold Send button (right-aligned)
5. **TaskPanel navbar** — SVG refresh icon (not ↻), SVG chevron-right collapse (not ×), `+ New Task` fixed height
6. **TaskRail** — Colored `w-7 h-7` squares with count numbers, `+` gold new task button, chevron expand icon
7. **TaskRail + button** — Click `+` button on rail, NewTaskForm modal opens

**Step 8: Clean up test tasks**

```bash
for id in $(curl -s http://localhost:3000/api/tasks | python3 -c "import sys,json;[print(t['id']) for t in json.load(sys.stdin)]"); do
  curl -s -X DELETE "http://localhost:3000/api/tasks/$id" > /dev/null
done
echo "Cleaned"
```

**Step 9: Final commit if any fixes needed**

```bash
git add -A && git status
# Only commit if there are changes
```
