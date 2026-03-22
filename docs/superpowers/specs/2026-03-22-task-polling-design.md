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

Add `OnChange func(event string, task Task)` callback field to the `Store` struct. Wire it to `broadcaster.Broadcast()` in the tasks server at startup.

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

### Event Payloads

| Event | Payload |
|---|---|
| `task.created` | Full task JSON |
| `task.updated` | Full task JSON |
| `task.stage_changed` | Full task JSON |
| `task.substep_changed` | Full task JSON |
| `task.deleted` | `{"id": <int>}` |
| `task.activity` | `{"taskId": <int>, "activity": <Activity>}` |
| `task.comment` | `{"taskId": <int>, "comment": <Comment>}` |

### Key Benefit

The executor's `store.Update(id, {stage: "validation"})` automatically broadcasts without any executor code changes. Any future code path that mutates tasks gets event broadcasting for free.

## Section 2: Delta Heartbeat Endpoint

### New Endpoint

`GET /api/tasks/sync?since=<unix_ms>`

**Response:**
```json
{
  "tasks": [/* tasks modified after `since` */],
  "deleted": [/* task IDs deleted after `since` */],
  "serverTime": 1711108800000,
  "fullSync": false
}
```

**Behavior:**
- `since=0` or omitted → full task list (`fullSync: true`)
- Otherwise → `WHERE updated_at > datetime(? / 1000, 'unixepoch')` query (converts Unix ms to SQLite DATETIME format) + tombstone lookup for deleted IDs
- If `since` is older than the oldest tombstone (>24h stale), return full list with `fullSync: true`

### Store Additions

- `Store.ListModifiedSince(since time.Time) ([]Task, error)` — `WHERE updated_at > datetime(? / 1000, 'unixepoch')`
- `Store.ListDeletedSince(since time.Time) ([]int64, error)` — reads from tombstone table
- `Store.AllCommentsAfterID(taskID, lastID int64) ([]Comment, error)` — all authors, no filter
- `Store.ActivityAfterID(taskID, lastID int64) ([]Activity, error)` — activity entries after given ID

### Tombstone Table

```sql
CREATE TABLE IF NOT EXISTS task_tombstones (
  id INTEGER NOT NULL,
  deleted_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_tombstones_deleted_at ON task_tombstones(deleted_at);
```

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
│   ├── Kanban: GET /api/tasks/sync?since= every 30s
│   └── Detail: GET /api/tasks/{id}/activity?after= + /comments?after= every 5s
└── State
    └── Map<taskId, Task> + per-task activity[] + comments[]
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

```typescript
function useTaskSync(options?: {
  taskId?: number;
  mode?: 'kanban' | 'detail';
}): {
  tasks: Task[];           // all tasks (kanban mode)
  task: Task | null;       // single task (detail mode)
  activities: Activity[];  // detail mode only
  comments: Comment[];     // detail mode only
  loading: boolean;
  refresh: () => void;     // manual force-refresh
}
```

### Behavior

1. **On mount:** Full fetch (`since=0`), store `serverTime`
2. **WS events:** Apply immediately to local state
3. **Heartbeat timer:**
   - `kanban` mode → 30s interval, calls `/api/tasks/sync?since=lastServerTime`, merges delta
   - `detail` mode → 5s interval, fetches activity + comments delta via `?after=` params
4. **On WS reconnect:** Immediate delta sync to catch anything missed during disconnect
5. **Tab visibility:** Pause heartbeat when hidden, immediate sync on tab focus

### Migration

- `useTasks` → deprecated, replaced by `useTaskSync({ mode: 'kanban' })`
- `useTaskEvents` → absorbed into `useTaskSync` internals, no longer exported
- `TasksPage` → switch to `useTaskSync({ mode: 'kanban' })`
- `TaskDetailPage` → switch to `useTaskSync({ taskId, mode: 'detail' })`

## Section 4: Error Handling & Edge Cases

### WS Disconnection

- Heartbeat continues via HTTP when WS is down — Kanban stays alive
- On WS reconnect, `useTaskSync` triggers immediate delta sync
- If both WS and HTTP fail, show "connection lost" indicator, backoff heartbeat (exponential up to 60s)

### Event Ordering

- WS events and heartbeat can arrive out of order
- Resolution: compare `updatedAt` — skip update if local copy is newer
- Activities and comments are append-only with sequential IDs — natural ordering

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
