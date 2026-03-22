# Task Polling Design — Event-Driven + Periodic Heartbeat

**Date:** 2026-03-22
**Author:** Shuri
**Status:** Reviewed
**Task:** #12

## Problem

The soul-v2 task system has a fully built backend SSE pipeline (tasks server → chat server SSE relay → WS broadcast to all clients) but the frontend ignores `task.*` WS messages. The Kanban board and task detail page only update on explicit user actions — no real-time updates, no periodic refresh.

## Decision

**Approach 2 — Unified event bus with delta heartbeat.**

- Store-level change hook broadcasts all task mutations automatically via SSE
- New delta sync endpoint returns only tasks changed since last sync
- Single `useTaskSync` frontend hook consumes both WS events (real-time) and heartbeat polling (reconciliation)
- Fast heartbeat (5s) on task detail page for activity/comments/fields
- Slow heartbeat (30s) on Kanban board as a safety net for missed events

## Section 1: Store-Level Change Hook

Every `Store` mutation triggers an automatic SSE broadcast. No caller needs to manually emit events.

### Store Changes

Add `OnChange func(event string, payload any)` callback field to the `Store` struct. Wire it to `broadcaster.Broadcast()` in the tasks server at startup. The `payload` parameter is typed per event — each store method constructs the appropriate payload struct before calling `OnChange`.

**Mutation hooks:**

| Store Method | Detect | Event Type |
|---|---|---|
| `Create()` | New task | `task.created` |
| `Update()` | Stage changed | `task.stage_changed` |
| `Update()` | Substep changed | `task.substep_changed` |
| `Update()` | Any other field | `task.updated` |
| `Delete()` | Task removed | `task.deleted` |
| `AddActivity()` | New activity entry | `task.activity` |
| `InsertComment()` | New comment | `task.comment` |

**Multi-field PATCH priority:** When a single `Update()` call changes multiple fields, emit exactly one event using this priority order: `task.stage_changed` > `task.substep_changed` > `task.updated`. The payload always includes the full updated task, so the client sees all field changes regardless of which event type fires.

### Event Payloads

Each event carries a typed payload. Store methods must return the inserted/updated object so it can be passed to `OnChange`.

| Event | Payload Type | Payload |
|---|---|---|
| `task.created` | `Task` | Full task JSON |
| `task.updated` | `Task` | Full task JSON |
| `task.stage_changed` | `Task` | Full task JSON |
| `task.substep_changed` | `Task` | Full task JSON |
| `task.deleted` | `TaskDeleted` | `{"id": <int>}` |
| `task.activity` | `TaskActivity` | `{"taskId": <int>, "activity": <Activity>}` |
| `task.comment` | `TaskComment` | `{"taskId": <int>, "comment": <Comment>}` |

**Required store method changes:**
- `AddActivity()` must return the inserted `Activity` (currently returns only `error`) — change signature to `AddActivity(...) (Activity, error)`, do an `INSERT ... RETURNING *` or re-read after insert
- `InsertComment()` must return the inserted `Comment` (currently returns only `error`) — same pattern
- `Delete()` must capture the task ID before deletion for the `TaskDeleted` payload

**Payload structs** (new, in `store/types.go`):
```go
type TaskDeleted struct {
    ID int64 `json:"id"`
}
type TaskActivity struct {
    TaskID   int64    `json:"taskId"`
    Activity Activity `json:"activity"`
}
type TaskComment struct {
    TaskID  int64   `json:"taskId"`
    Comment Comment `json:"comment"`
}
```

### Key Benefit

The executor's `store.Update(id, {stage: "validation"})` automatically broadcasts without any executor code changes. Any future code path that mutates tasks gets event broadcasting for free.

## Section 2: Delta Heartbeat Endpoint

### New Endpoint

`GET /api/tasks/sync?cursor=<opaque_token>`

**Response:**
```json
{
  "tasks": [/* tasks modified since cursor */],
  "deleted": [/* task IDs deleted since cursor */],
  "cursor": "1711108800.42",
  "fullSync": false
}
```

The cursor is an opaque token encoding a monotonic sequence number (not a wall-clock timestamp). This avoids SQLite's second-precision `CURRENT_TIMESTAMP` losing same-second writes.

**Implementation:** Add an auto-increment `seq INTEGER` column to the `tasks` table. Each `Update()` bumps `seq` to `MAX(seq) + 1` (or use a global sequence table). The cursor encodes the last-seen `seq` value. The sync query becomes `WHERE seq > ?` — no datetime conversion, no precision issues.

**Behavior:**
- `cursor` empty or omitted → full task list (`fullSync: true`), response includes current max `seq` as cursor
- Otherwise → `WHERE seq > ?` query + tombstone lookup for deleted IDs
- If `now - tombstone_retention (24h)` has elapsed since the cursor's creation time (tracked in a `cursor_meta` field or inferred from `seq` gap), return full list with `fullSync: true`

**Stale detection rule:** The server tracks `retention_start_seq` — the `seq` value at `now - 24h`. If the client's cursor `seq < retention_start_seq`, tombstones may be pruned and the response is `fullSync: true`. When no tombstones exist, the same rule applies — an empty tombstone table with a stale cursor still triggers full sync.

**Race-safety:** The `serverTime` watermark is replaced by `cursor` (seq-based). The server computes the cursor **before** executing the query (snapshot the current max `seq`), so writes that arrive during query execution will be included in the next sync, not lost in a query-window gap.

### Store Additions

- `Store.ListModifiedSince(seq int64) ([]Task, error)` — `WHERE seq > ?`
- `Store.ListDeletedSince(seq int64) ([]int64, error)` — reads from tombstone table `WHERE seq > ?`
- `Store.MaxSeq() (int64, error)` — returns current max sequence number
- `Store.RetentionStartSeq() (int64, error)` — returns seq at `now - 24h` (for stale detection)
- `Store.AllCommentsAfterID(taskID, lastID int64) ([]Comment, error)` — all authors, no filter
- `Store.ActivityAfterID(taskID, lastID int64) ([]Activity, error)` — activity entries after given ID, ascending order

### Tombstone Table

```sql
CREATE TABLE IF NOT EXISTS task_tombstones (
  id INTEGER NOT NULL,
  seq INTEGER NOT NULL,
  deleted_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_tombstones_seq ON task_tombstones(seq);
CREATE INDEX idx_tombstones_deleted_at ON task_tombstones(deleted_at);
```

**Sequence column on tasks table:**
```sql
ALTER TABLE tasks ADD COLUMN seq INTEGER NOT NULL DEFAULT 0;
CREATE INDEX idx_tasks_seq ON tasks(seq);
```

A global sequence counter is maintained via a `sync_meta` table:
```sql
CREATE TABLE IF NOT EXISTS sync_meta (
  key TEXT PRIMARY KEY,
  value INTEGER NOT NULL
);
-- Initialize: INSERT OR IGNORE INTO sync_meta VALUES ('seq', 0);
```

`Store.nextSeq()` increments and returns the next value atomically within the calling transaction.

- `Store.Delete()` wraps tombstone insert + task delete in a single `BEGIN/COMMIT` transaction to prevent inconsistent state
- `Store.PruneTombstones()` deletes rows older than 24h
- Pruning runs once on server startup, then every hour via background goroutine

### Activity/Comments Delta

Existing endpoints extended with delta support:
- `GET /api/tasks/{id}/activity?after=<lastActivityId>` — returns only newer entries
- `GET /api/tasks/{id}/comments?after=<lastCommentId>` — returns only newer entries

**Note:** The existing `CommentsAfter(lastID)` filters `WHERE author = 'user'` (built for the comment watcher). The heartbeat endpoint needs **all** comments (including agent/system). Add a new `Store.AllCommentsAfterID(taskID, lastID int64) ([]Comment, error)` method without the author filter. Reuse the same `WHERE id > ?` pattern for activity via `Store.ActivityAfterID(taskID, lastID int64) ([]Activity, error)`.

## Section 3: Frontend Unified `useTaskSync` Hook

Single hook that replaces `useTasks` and absorbs `useTaskEvents`. Two update paths feed one state.

### Architecture

```
useTaskSync
├── WS event path (real-time)
│   └── useChat.ts onMessage → window 'ws:task-event' → apply to state
├── Heartbeat path (reconciliation)
│   ├── Kanban: GET /api/tasks/sync?cursor= every 30s
│   └── Detail: GET /api/tasks/{id} + /activity?after= + /comments?after= every 5s (parallel)
├── Actions (HTTP mutations)
│   └── create/update/delete/start/stop/addComment → optimistic update → HTTP → reconcile
└── State
    └── Map<taskId, Task> + per-task activity[] + comments[] + error + connected
```

### WS Event Wiring

Fix in `useChat.ts` `handleMessage` callback. The callback signature is `(type, data, sessionID, messageId)` — there is no `msg` object in scope. Add this **before** the existing `switch(type)` block:

```typescript
// Forward task events to useTaskSync via DOM event
if (type?.startsWith('task.')) {
  window.dispatchEvent(new CustomEvent('ws:task-event', { detail: { type, data } }));
  return; // task events don't need chat message handling
}
```

**Important:** The old `useTaskEvents` hook reads `event.data` (MessageEvent shape) but `CustomEvent` carries payload in `event.detail`. Since `useTaskEvents` is being absorbed into `useTaskSync`, the new hook must read from `(event as CustomEvent).detail`, not `event.data`.

**Type system:** `OutboundMessageType` in `types.ts` is a strict union generated from specs — it does not include `task.*` variants. Since the forwarding runs before the typed `switch`, use a raw string check on `type` (cast via `as string`) to avoid tsc errors. Alternatively, update the YAML spec to include task event types and run `make types`.

### Hook API

The hook is split into two concerns: **sync** (real-time state) and **actions** (mutations). Both are returned from the same hook so pages have a single import.

```typescript
function useTaskSync(options?: {
  taskId?: number;
  mode?: 'kanban' | 'detail';
}): {
  // State (real-time via WS + heartbeat)
  tasks: Task[];           // all tasks (kanban mode)
  task: Task | null;       // single task (detail mode)
  activities: Activity[];  // detail mode only
  comments: Comment[];     // detail mode only
  loading: boolean;
  error: string | null;    // connection/fetch error state
  connected: boolean;      // WS connection status
  refresh: () => void;     // manual force-refresh

  // Actions (HTTP mutations — optimistic update + sync)
  createTask: (task: Partial<Task>) => Promise<Task>;
  updateTask: (id: number, fields: Partial<Task>) => Promise<Task>;
  deleteTask: (id: number) => Promise<void>;
  startTask: (id: number) => Promise<void>;
  stopTask: (id: number) => Promise<void>;
  addComment: (id: number, body: string) => Promise<void>;
}
```

**Action behavior:** Each action does an optimistic local state update, fires the HTTP request, then lets the WS event or next heartbeat reconcile the final state. On HTTP error, the optimistic update is rolled back and `error` is set.

### Behavior

1. **On mount:** Full fetch (no cursor), store returned `cursor`
2. **WS events:** Apply immediately to local state
3. **Heartbeat timer:**
   - `kanban` mode → 30s interval, calls `/api/tasks/sync?cursor=`, merges delta
   - `detail` mode → 5s interval, fetches three things in parallel:
     - `GET /api/tasks/{id}` — task fields (stage, substep, metadata) for when WS is down
     - `GET /api/tasks/{id}/activity?after=<lastActivityId>` — new activity entries
     - `GET /api/tasks/{id}/comments?after=<lastCommentId>` — new comments
4. **On WS reconnect:** Immediate delta sync to catch anything missed during disconnect
5. **Tab visibility:** Pause heartbeat when hidden, immediate sync on tab focus

### Migration

- `useTasks` → deprecated, replaced by `useTaskSync({ mode: 'kanban' })`
- `useTaskEvents` → absorbed into `useTaskSync` internals, no longer exported
- `TasksPage` → switch to `useTaskSync({ mode: 'kanban' })`
- `TaskDetailPage` → switch to `useTaskSync({ taskId, mode: 'detail' })`
- `DashboardPage` → switch from `useTasks` to `useTaskSync({ mode: 'kanban' })` (uses task counts by stage)

## Section 4: Error Handling & Edge Cases

### WS Disconnection

- Heartbeat continues via HTTP when WS is down — Kanban stays alive
- On WS reconnect, `useTaskSync` triggers immediate delta sync
- If both WS and HTTP fail, show "connection lost" indicator, backoff heartbeat (exponential up to 60s)

### Event Ordering & Merge Semantics

**Task updates** (WS events and heartbeat can arrive out of order):
- Resolution: compare `seq` — skip update if local `seq` is >= incoming `seq`
- On `fullSync: true`, replace the entire local task map (not merge)

**Activity/comments** (append-only):
- Delta endpoints (`?after=`) return entries in **ascending ID order** (oldest first). The current `ListActivity` query uses `ORDER BY id DESC` (newest first) — delta endpoints must use `ORDER BY id ASC` explicitly.
- **WS merge:** On `task.activity` / `task.comment` event, append to the end of the local array. Deduplicate by `id` — if an entry with the same `id` already exists, skip.
- **Heartbeat merge:** Append delta results to the end of the local array. Same dedup rule.
- **Display order:** The UI component is responsible for display ordering (e.g., `activities.toReversed()` for newest-first in the timeline). The internal state array is always in ascending ID order.

### Tombstone Expiry

- Auto-pruned after 24h via `Store.PruneTombstones()`
- Runs on startup + every hour via background goroutine
- Client stale >24h → sync endpoint detects gap, returns full list with `fullSync: true`

### Heartbeat Backpressure

- If previous heartbeat response hasn't returned, skip current tick
- Prevents request pileup on slow connections

### SSE Relay Resilience

- Chat server's `StartSSERelay` already has exponential backoff reconnect — no changes needed
- Tasks server restart → relay reconnects, heartbeat covers the gap

## Files Changed

### Backend (Go)

| File | Change |
|---|---|
| `internal/tasks/store/store.go` | Add `OnChange` callback, wire into `Create`, `Update`, `Delete`, `AddActivity`, `InsertComment` |
| `internal/tasks/store/store.go` | Add `ListModifiedSince`, `ListDeletedSince`, `PruneTombstones` methods |
| `internal/tasks/store/store.go` | Add `task_tombstones` table to schema migration |
| `internal/tasks/server/server.go` | Wire `store.OnChange` → `broadcaster.Broadcast` at startup |
| `internal/tasks/server/server.go` | Add `GET /api/tasks/sync` handler |
| `internal/tasks/server/server.go` | Add `?after=` param support to activity and comments endpoints |
| `internal/tasks/server/server.go` | Remove manual `Broadcast` calls from `handleCreateTask`, `handleUpdateTask`, `handleStartTask` (now automatic via store hook) |

### Frontend (TypeScript)

| File | Change |
|---|---|
| `web/src/hooks/useChat.ts` | Add `task.*` → `window.dispatchEvent` forwarding before the `switch(type)` block in `handleMessage` |
| `web/src/lib/types.ts` | Either update YAML spec + `make types` to include `task.*` in `OutboundMessageType`, or use raw string check before the typed switch |
| `web/src/hooks/useTaskSync.ts` | **New file** — unified hook with WS events + heartbeat |
| `web/src/hooks/useTasks.ts` | Deprecate (replace imports with useTaskSync) |
| `web/src/hooks/useTaskEvents.ts` | Remove (absorbed into useTaskSync) |
| `web/src/pages/TasksPage.tsx` | Switch from `useTasks` to `useTaskSync({ mode: 'kanban' })` |
| `web/src/pages/TaskDetailPage.tsx` | Switch to `useTaskSync({ taskId, mode: 'detail' })` |
| `web/src/pages/DashboardPage.tsx` | Switch from `useTasks` to `useTaskSync({ mode: 'kanban' })` |

## Testing Strategy

### Backend

- `store_test.go`: Verify `OnChange` fires correct event types for each mutation
- `store_test.go`: Test `ListModifiedSince` returns only changed tasks, `ListDeletedSince` returns tombstoned IDs
- `store_test.go`: Test `PruneTombstones` removes old entries, preserves recent ones
- `server_test.go`: Test `/api/tasks/sync` endpoint with various `since` values including stale (>24h)
- `server_test.go`: Test `?after=` params on activity and comments endpoints
- Integration: Verify store hook → SSE broadcast → WS relay pipeline end-to-end

### Frontend

- `useTaskSync.test.ts`: Mock WS events update state immediately
- `useTaskSync.test.ts`: Mock heartbeat fetches delta and merges correctly
- `useTaskSync.test.ts`: Verify `updatedAt` ordering prevents stale overwrites
- `useTaskSync.test.ts`: Tab visibility pauses/resumes heartbeat
- `useTaskSync.test.ts`: WS reconnect triggers immediate sync
- E2E: Create task via API, verify Kanban updates within 2s (WS path) and within 35s (heartbeat path)
