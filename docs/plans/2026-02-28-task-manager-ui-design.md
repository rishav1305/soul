# Task Manager UI Redesign

**Date:** 2026-02-28
**Status:** Approved
**Approach:** State Machine Panels (Approach 1)

## Overview

Redesign the Soul two-panel layout to support collapsible panels, multiple task view modes, filters, dynamic panel widths, and chat session management. Replace the global `◆ Soul` header with independent per-panel navbars.

## 1. Layout Architecture

No global header. Each panel gets its own navbar row.

```
┌─────────────────────────┬─────────────────────────────────┐
│ ☰ ◆ Soul Chat       [−] │ Tasks  [▫][≡][⊞][⊟][×] +New   │
├─────────────────────────┼─────────────────────────────────┤
│                         │ Stage:[All] Pri:[All] Prod:[All]│
│                         ├─────────────────────────────────┤
│   Chat content          │   Task content (varies by view) │
│                         │                                 │
└─────────────────────────┴─────────────────────────────────┘
```

**Navbar controls:**
- Chat navbar: `☰` session drawer toggle, `◆ Soul Chat` title, `[−]` collapse-to-rail
- Task navbar: `Tasks` title, view mode buttons `[▫][≡][⊞][⊟]`, `[×]` collapse-to-rail, `+ New Task`
- Active view mode button highlighted

**Panel states:**
- `open` — full panel with navbar + content
- `rail` — 40px vertical strip
- Constraint: at least one panel must be `open`. Collapsing the last open panel is a no-op (button disabled).

## 2. View Modes (Task Panel)

Four view modes selectable from the Task navbar:

### View 1 — Rail (`▫`)

Panel collapses to ~40px vertical strip:

```
┌──┐
│⊞ │  ← expand button
│──│
│● │ 3  ← backlog (zinc dot + count)
│● │ 1  ← brainstorm (purple)
│● │ 5  ← active (sky)
│● │ 0  ← blocked (red)
│● │ 2  ← validation (amber)
│● │ 4  ← done (green)
└──┘
```

Clicking anywhere on the rail expands back to previous view mode.

### View 2 — List (`≡`)

Compact grouped list. Tasks grouped by stage in collapsible sections:

```
▼ ACTIVE (3)
  #1  Fix auth bug          High   compliance   [TDD 1/6]
  #3  Compliance scan       Crit   compliance   [QA 4/6]
  #7  Update API docs       Norm
▼ BACKLOG (2)
  #2  Deploy website        Norm
  #5  Refactor WS handler   High
▶ BLOCKED (0)                        ← auto-collapsed when empty
▶ DONE (4)
```

- Empty stages collapsed by default, non-empty expanded
- Clicking a row opens TaskDetail modal
- Stage order: active → backlog → brainstorm → blocked → validation → done (active-first)

### View 3 — Kanban (`⊞`)

Current 6-column layout. Receives filtered tasks from parent. In Kanban view, filtered-out stages hide their columns entirely.

### View 4 — Grid (`⊟`)

Three sub-views toggled by pill selector inside content area. Compact grid is default.

```
[Grid · Table · Grouped]
```

- **Grid** (default): Responsive card grid, ~180px wide cards wrapping to fill space. Each card: title, #id, stage dot+label, priority border. Sorted by priority desc then stage.
- **Table**: Full-width sortable spreadsheet. Columns: ID, Title, Stage, Priority, Product, Substep, Updated. Click headers to sort.
- **Grouped list**: Same as View 2 but with more detail per row (description preview, timestamps).

## 3. Filters

Filter bar below Task navbar, above content. Visible in all views except Rail.

```
│ Stage: [All ▾]  Priority: [All ▾]  Product: [All ▾] │
```

- **Stage:** All / Backlog / Brainstorm / Active / Blocked / Validation / Done
- **Priority:** All / Critical / High / Normal / Low
- **Product:** All / (dynamically populated from existing tasks)
- Additive AND logic
- Filter state persists when switching view modes
- Filter changes trigger auto-resize recalculation

## 4. Dynamic Panel Width

Auto-resize based on visible (post-filter) task count:

| Visible Tasks | Chat Width | Tasks Width |
|---------------|-----------|-------------|
| 0             | 85%       | 15%         |
| 1-3           | 75%       | 25%         |
| 4-10          | 60%       | 40%         |
| 11-20         | 45%       | 55%         |
| 20+           | 25%       | 75%         |

- Min chat: 25%, min tasks (when open): 15%
- **Manual override:** 4px draggable divider between panels. Once dragged, auto-resize stops. Reset icon `↻` in Task navbar re-enables auto.
- Rail state: other panel gets `calc(100% - 40px)`
- Width transitions: `transition: width 200ms ease`

## 5. Chat Panel

### Chat Rail (collapsed state)

```
┌──┐
│◆ │  ← Soul icon (click to expand)
│  │
│3 │  ← unread message count
│  │
│💬│  ← chat bubble icon
└──┘
```

- 40px wide, same as Task rail
- Unread count increments when messages arrive while collapsed, resets on expand
- If count > 9, show `9+`

### Session Management

Sidebar drawer toggled by `☰` hamburger icon. Shows last 10 sessions.

```
┌──────────────────────┐
│ Sessions          [×] │
├───────────────────────┤
│ [+ New Chat]          │
│───────────────────────│
│ ● Current session     │  ← highlighted
│   "Fix auth bug..."   │
│   2 min ago           │
│───────────────────────│
│ ● Running: scan       │
│   "Compliance scan"   │
│   15 min ago          │
│───────────────────────│
│ ○ Planner setup       │
│   Yesterday 2:30pm    │
│───────────────────────│
│ ✓ DB migration        │
│   Feb 26, 11:00am     │
└───────────────────────┘
```

**Session states:**
- `● running` (green) — active, streaming or awaiting input
- `○ idle` (hollow) — exists but no active streaming
- `✓ completed` (checkmark) — finished, read-only

**Behavior:**
- Drawer overlays chat content (~200px wide), slide animation (200ms)
- Click outside to close
- Each entry: status dot, title (from first user message), relative timestamp
- Click to switch session (loads message history)
- `+ New Chat` creates fresh session
- Running sessions show blue unread dot if messages arrive while viewing another session
- Max 10 entries

## 6. State Management

### `useLayoutStore` hook

```typescript
interface LayoutState {
  chatState: 'rail' | 'open';
  taskState: 'rail' | 'open';
  taskView: 'list' | 'kanban' | 'grid';
  gridSubView: 'grid' | 'table' | 'grouped';
  panelWidth: number | null;  // null = auto, number = manual %
  filters: {
    stage: TaskStage | 'all';
    priority: number | 'all';
    product: string | 'all';
  };
}
```

- `taskState: 'rail'` overrides `taskView` visually. Expanding from rail restores previous `taskView`.
- Collapsing checks: if other panel is also `rail`, action blocked.
- `panelWidth: null` = auto-compute from task count. Manual drag sets number. Reset returns to null.
- Entire state persisted to `localStorage`.

## 7. Component Structure

### New files

```
web/src/
├── hooks/
│   ├── useLayoutStore.ts          # Panel state machine + localStorage
│   └── useSessions.ts             # Chat session CRUD + list
├── components/
│   ├── layout/
│   │   ├── AppShell.tsx           # Top-level: arranges panels + divider
│   │   ├── ResizeDivider.tsx      # 4px draggable divider
│   │   ├── ChatRail.tsx           # 40px collapsed chat strip
│   │   └── TaskRail.tsx           # 40px collapsed task strip
│   ├── chat/
│   │   ├── ChatPanel.tsx          # ChatNavbar + SessionDrawer + ChatView
│   │   ├── ChatNavbar.tsx         # ☰ title + collapse button
│   │   └── SessionDrawer.tsx      # Slide-out session list
│   └── planner/
│       ├── TaskPanel.tsx          # TaskNavbar + FilterBar + TaskContent
│       ├── TaskNavbar.tsx         # Title + view buttons + collapse + new task
│       ├── FilterBar.tsx          # 3 dropdown filters
│       ├── TaskContent.tsx        # Switches view based on taskView
│       ├── ListView.tsx           # View 2: grouped collapsible list
│       ├── GridView.tsx           # View 4: sub-view container
│       │   ├── CompactGrid.tsx    # Responsive card grid (default)
│       │   ├── TableView.tsx      # Sortable spreadsheet
│       │   └── GroupedList.tsx    # Expanded grouped list
│       ├── KanbanBoard.tsx        # View 3: modified to accept filtered tasks
│       ├── StageColumn.tsx        # Existing, unchanged
│       └── TaskCard.tsx           # Existing, unchanged
```

### Modified files

- `App.tsx` — replace layout with `AppShell`
- `KanbanBoard.tsx` — accept filtered tasks prop, remove internal navbar
- `usePlanner.ts` — add filter logic, expose `filteredTasks`
- `lib/types.ts` — add `ChatSession`, `LayoutState` types

### Backend — new files

```
internal/server/sessions.go        # 3 REST handlers
```

### Backend — modified files

- `internal/planner/store.go` — add chat_sessions + chat_messages tables
- `internal/server/routes.go` — add 3 session routes
- `internal/server/ws.go` — tag messages with session ID
- `internal/server/server.go` — wire session handlers

### New DB tables

```sql
CREATE TABLE chat_sessions (
    id INTEGER PRIMARY KEY,
    title TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'idle',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE chat_messages (
    id INTEGER PRIMARY KEY,
    session_id INTEGER NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TEXT NOT NULL
);
```

## 8. Interactions + Edge Cases

**Resize divider:**
- 4px wide, `cursor: col-resize`. 8px hit area on hover.
- Dragging sets manual `panelWidth`. Reset icon `↻` in Task navbar returns to auto.
- Min constraints: chat >= 25%, tasks >= 15%.
- Hidden when either panel is in rail state.

**Auto-resize triggers:**
- Task created/deleted/moved
- Filter changed
- Panel expanded from rail to open

**View mode transitions:**
- Instant switch, no content animation.
- Active button: `bg-zinc-700` highlight.
- Grid sub-view tabs: underline indicator.

**Session drawer:**
- Slide animation 200ms.
- Click outside closes.
- Brief opacity fade on session switch while messages load.
- Blue unread dot on running sessions with new messages.

**Chat rail unread:**
- Increments per assistant message while collapsed.
- Resets on expand. `9+` cap.

**Edge cases:**
- Zero tasks: empty state "No tasks yet. Create one to get started." Min 15% width.
- All filtered out: "No tasks match filters" + "Clear filters" link.
- Empty session: `◆ How can I help you?` placeholder.
- Page reload: layout from localStorage, active session re-fetched.
