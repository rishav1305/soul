# Task Polling Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire real-time task updates end-to-end — store-level change hooks broadcast mutations via SSE, a new delta sync endpoint enables cursor-based heartbeat polling, and a unified `useTaskSync` frontend hook replaces `useTasks`/`useTaskEvents` to give the Kanban board and task detail page live updates.

**Architecture:** Backend adds a monotonic sequence counter (`sync_meta` table), an `OnChange` callback on all store mutations, and a `GET /api/tasks/sync?cursor=` delta endpoint. Frontend adds a single `useTaskSync` hook that merges WS events (real-time) with periodic heartbeat polling (reconciliation), with mode-specific behavior for Kanban (30s full-task delta) and detail (5s task+activity+comments delta).

**Tech Stack:** Go 1.24, SQLite (WAL mode), React 19, TypeScript 5.9, Vite 7

**Spec:** `docs/superpowers/specs/2026-03-22-task-polling-design.md`

---

## File Structure

### New files
| File | Responsibility |
|---|---|
| `internal/tasks/store/types.go` | Event payload structs: `TaskDeleted`, `TaskActivity`, `TaskComment` |
| `internal/tasks/store/cursor.go` | `EncodeCursor` / `DecodeCursor` — base64 JSON `{seq, ts}` |
| `internal/tasks/store/cursor_test.go` | Cursor encode/decode round-trip + edge cases |
| `internal/tasks/store/store_test.go` | All store tests (OnChange, seq, delta queries, tombstones, return values) |
| `internal/tasks/server/server_test.go` | Sync endpoint, delta activity/comments endpoints |
| `web/src/hooks/useTaskSync.ts` | Unified hook — WS events + heartbeat + actions |

### Modified files
| File | Changes |
|---|---|
| `internal/tasks/store/store.go` | `OnChange` callback, `nextSeq()`, `seq` column, `sync_meta`/`task_tombstones` tables, wire OnChange into all mutations, change `AddActivity`/`InsertComment` return types, add `ListModifiedSince`/`ListDeletedSince`/`MaxSeq`/`PruneTombstones`/`AllCommentsAfterID`/`ActivityAfterID` |
| `internal/tasks/server/server.go` | Wire `OnChange→Broadcast`, register `GET /api/tasks/sync`, add `?after=` to activity/comments, remove manual Broadcast calls, start tombstone pruning goroutine |
| `web/src/hooks/useChat.ts` | Forward `task.*` WS messages via `CustomEvent` before the typed switch |
| `web/src/pages/TasksPage.tsx` | Replace `useTasks` with `useTaskSync({ mode: 'kanban' })` |
| `web/src/pages/TaskDetailPage.tsx` | Replace manual fetching with `useTaskSync({ taskId, mode: 'detail' })` |
| `web/src/pages/DashboardPage.tsx` | Replace `useTasks` with `useTaskSync({ mode: 'kanban' })` |
| `web/src/lib/types.ts` | Add `seq: number` and `substep: string` to `Task` interface (manual types section, not auto-generated) |
| `web/src/hooks/useTasks.ts` | Delete (all imports migrated) |
| `web/src/hooks/useTaskEvents.ts` | Delete (absorbed into useTaskSync) |

---

## Task 1: Event Payload Structs

**Files:**
- Create: `internal/tasks/store/types.go`

- [ ] **Step 1: Create the payload structs file**

```go
package store

// TaskDeleted is the OnChange payload for task.deleted events.
type TaskDeleted struct {
	ID int64 `json:"id"`
}

// TaskActivity is the OnChange payload for task.activity events.
type TaskActivity struct {
	TaskID   int64    `json:"taskId"`
	Activity Activity `json:"activity"`
}

// TaskComment is the OnChange payload for task.comment events.
type TaskComment struct {
	TaskID  int64   `json:"taskId"`
	Comment Comment `json:"comment"`
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /home/rishav/soul-v2 && go build ./internal/tasks/store/...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/tasks/store/types.go
git commit -m "feat(tasks): add event payload structs for OnChange hook"
```

---

## Task 2: Cursor Encode/Decode

**Files:**
- Create: `internal/tasks/store/cursor.go`
- Create: `internal/tasks/store/cursor_test.go`

- [ ] **Step 1: Write cursor tests**

```go
package store

import (
	"testing"
	"time"
)

func TestCursorRoundTrip(t *testing.T) {
	now := time.Now().Unix()
	token := EncodeCursor(42, now)
	seq, ts, err := DecodeCursor(token)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if seq != 42 {
		t.Errorf("seq = %d, want 42", seq)
	}
	if ts != now {
		t.Errorf("ts = %d, want %d", ts, now)
	}
}

func TestDecodeCursorEmpty(t *testing.T) {
	seq, ts, err := DecodeCursor("")
	if err != nil {
		t.Fatalf("decode empty: %v", err)
	}
	if seq != 0 || ts != 0 {
		t.Errorf("expected zero values for empty cursor, got seq=%d ts=%d", seq, ts)
	}
}

func TestDecodeCursorInvalid(t *testing.T) {
	_, _, err := DecodeCursor("not-base64-json")
	if err == nil {
		t.Error("expected error for invalid cursor")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/store/ -run TestCursor -v`
Expected: FAIL — `EncodeCursor` and `DecodeCursor` undefined

- [ ] **Step 3: Write cursor implementation**

```go
package store

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

type cursor struct {
	Seq int64 `json:"seq"`
	Ts  int64 `json:"ts"`
}

// EncodeCursor produces an opaque base64-encoded cursor token.
func EncodeCursor(seq, ts int64) string {
	b, _ := json.Marshal(cursor{Seq: seq, Ts: ts})
	return base64.RawURLEncoding.EncodeToString(b)
}

// DecodeCursor parses an opaque cursor token.
// Empty string returns (0, 0, nil) — meaning "full sync".
func DecodeCursor(token string) (seq, ts int64, err error) {
	if token == "" {
		return 0, 0, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return 0, 0, fmt.Errorf("cursor: bad base64: %w", err)
	}
	var c cursor
	if err := json.Unmarshal(b, &c); err != nil {
		return 0, 0, fmt.Errorf("cursor: bad json: %w", err)
	}
	return c.Seq, c.Ts, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/store/ -run TestCursor -v`
Expected: PASS (3 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/tasks/store/cursor.go internal/tasks/store/cursor_test.go
git commit -m "feat(tasks): cursor encode/decode for delta sync"
```

---

## Task 3: Schema Migration — seq column, sync_meta, tombstones

**Files:**
- Modify: `internal/tasks/store/store.go` (the `migrate()` method)

- [ ] **Step 1: Write migration test**

Add to `internal/tasks/store/store_test.go`:

```go
package store

import (
	"os"
	"path/filepath"
	"testing"
)

// NOTE: newTestStore already exists in store_test.go — reuse it, do not redefine.

func TestMigration_TablesExist(t *testing.T) {
	s := newTestStore(t)

	// Verify sync_meta table exists and has initial seq=0.
	var val int64
	err := s.db.QueryRow("SELECT value FROM sync_meta WHERE key = 'seq'").Scan(&val)
	if err != nil {
		t.Fatalf("sync_meta query: %v", err)
	}
	if val != 0 {
		t.Errorf("initial seq = %d, want 0", val)
	}

	// Verify task_tombstones table exists.
	_, err = s.db.Exec("SELECT id, seq, deleted_at FROM task_tombstones LIMIT 0")
	if err != nil {
		t.Fatalf("task_tombstones not created: %v", err)
	}

	// Verify seq column on tasks.
	task, err := s.Create("test", "", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	var seq int64
	err = s.db.QueryRow("SELECT seq FROM tasks WHERE id = ?", task.ID).Scan(&seq)
	if err != nil {
		t.Fatalf("seq column query: %v", err)
	}
	// seq should be 0 (default) until we wire nextSeq into Create in Task 4.
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/store/ -run TestMigration -v`
Expected: FAIL — `sync_meta` table doesn't exist yet

- [ ] **Step 3: Add migration code to store.go**

In `store.go`, append to the end of the `migrate()` method (before `return nil`):

```go
	// Add seq column to tasks (ignore if already exists).
	_, err = s.db.Exec("ALTER TABLE tasks ADD COLUMN seq INTEGER NOT NULL DEFAULT 0")
	if err != nil && !strings.Contains(err.Error(), "duplicate column") {
		return fmt.Errorf("tasks: migrate seq column: %w", err)
	}

	// Create seq index.
	if _, err := s.db.Exec("CREATE INDEX IF NOT EXISTS idx_tasks_seq ON tasks(seq)"); err != nil {
		return fmt.Errorf("tasks: migrate seq index: %w", err)
	}

	// Tombstone table for tracking deleted task IDs.
	const tombstoneSchema = `
	CREATE TABLE IF NOT EXISTS task_tombstones (
		id INTEGER NOT NULL,
		seq INTEGER NOT NULL,
		deleted_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_tombstones_seq ON task_tombstones(seq);
	CREATE INDEX IF NOT EXISTS idx_tombstones_deleted_at ON task_tombstones(deleted_at);
	`
	if _, err := s.db.Exec(tombstoneSchema); err != nil {
		return fmt.Errorf("tasks: migrate tombstones: %w", err)
	}

	// Global monotonic sequence counter.
	const syncMetaSchema = `
	CREATE TABLE IF NOT EXISTS sync_meta (
		key TEXT PRIMARY KEY,
		value INTEGER NOT NULL
	);
	`
	if _, err := s.db.Exec(syncMetaSchema); err != nil {
		return fmt.Errorf("tasks: migrate sync_meta: %w", err)
	}
	if _, err := s.db.Exec("INSERT OR IGNORE INTO sync_meta (key, value) VALUES ('seq', 0)"); err != nil {
		return fmt.Errorf("tasks: init sync_meta seq: %w", err)
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/store/ -run TestMigration -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tasks/store/store.go internal/tasks/store/store_test.go
git commit -m "feat(tasks): schema migration — seq column, sync_meta, tombstones"
```

---

## Task 4: nextSeq + OnChange Callback + Wire into Mutations

**Files:**
- Modify: `internal/tasks/store/store.go`

This is the core task. It adds:
1. `OnChange` callback field to `Store`
2. `nextSeq()` internal method
3. Wires `OnChange` into `Create`, `Update`, `Delete`, `AddActivity`, `InsertComment`
4. Changes `AddActivity` return type to `(Activity, error)`
5. Changes `InsertComment` return type to `(Comment, error)`

- [ ] **Step 1: Write OnChange + nextSeq tests**

Add to `store_test.go`:

```go
func TestNextSeq_Monotonic(t *testing.T) {
	s := newTestStore(t)
	tx, err := s.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	seq1, err := s.nextSeqTx(tx)
	if err != nil {
		t.Fatal(err)
	}
	seq2, err := s.nextSeqTx(tx)
	if err != nil {
		t.Fatal(err)
	}
	tx.Commit()
	if seq1 != 1 || seq2 != 2 {
		t.Errorf("seq1=%d seq2=%d, want 1, 2", seq1, seq2)
	}
}

func TestOnChange_Create(t *testing.T) {
	s := newTestStore(t)
	var gotEvent string
	var gotPayload any
	s.OnChange = func(event string, payload any) {
		gotEvent = event
		gotPayload = payload
	}
	task, err := s.Create("test", "desc", "general")
	if err != nil {
		t.Fatal(err)
	}
	if gotEvent != "task.created" {
		t.Errorf("event = %q, want task.created", gotEvent)
	}
	if p, ok := gotPayload.(*Task); !ok || p.ID != task.ID {
		t.Errorf("payload mismatch")
	}
}

func TestOnChange_Update_StageChanged(t *testing.T) {
	s := newTestStore(t)
	task, _ := s.Create("test", "", "")
	var gotEvent string
	s.OnChange = func(event string, payload any) {
		gotEvent = event
	}
	s.Update(task.ID, map[string]interface{}{"stage": "active"})
	if gotEvent != "task.stage_changed" {
		t.Errorf("event = %q, want task.stage_changed", gotEvent)
	}
}

func TestOnChange_Update_SubstepChanged(t *testing.T) {
	s := newTestStore(t)
	task, _ := s.Create("test", "", "")
	var gotEvent string
	s.OnChange = func(event string, payload any) {
		gotEvent = event
	}
	s.Update(task.ID, map[string]interface{}{"substep": "reviewing"})
	if gotEvent != "task.substep_changed" {
		t.Errorf("event = %q, want task.substep_changed", gotEvent)
	}
}

func TestOnChange_Update_OtherField(t *testing.T) {
	s := newTestStore(t)
	task, _ := s.Create("test", "", "")
	var gotEvent string
	s.OnChange = func(event string, payload any) {
		gotEvent = event
	}
	s.Update(task.ID, map[string]interface{}{"title": "new title"})
	if gotEvent != "task.updated" {
		t.Errorf("event = %q, want task.updated", gotEvent)
	}
}

func TestOnChange_Update_MultiField_Priority(t *testing.T) {
	s := newTestStore(t)
	task, _ := s.Create("test", "", "")
	var gotEvent string
	s.OnChange = func(event string, payload any) {
		gotEvent = event
	}
	// Stage + substep + title in one PATCH — stage wins
	s.Update(task.ID, map[string]interface{}{
		"stage": "active", "substep": "reviewing", "title": "new",
	})
	if gotEvent != "task.stage_changed" {
		t.Errorf("event = %q, want task.stage_changed (priority)", gotEvent)
	}
}

func TestOnChange_Delete(t *testing.T) {
	s := newTestStore(t)
	task, _ := s.Create("test", "", "")
	var gotEvent string
	var gotPayload any
	s.OnChange = func(event string, payload any) {
		gotEvent = event
		gotPayload = payload
	}
	s.Delete(task.ID)
	if gotEvent != "task.deleted" {
		t.Errorf("event = %q, want task.deleted", gotEvent)
	}
	if p, ok := gotPayload.(TaskDeleted); !ok || p.ID != task.ID {
		t.Errorf("payload = %v, want TaskDeleted{ID: %d}", gotPayload, task.ID)
	}
}

func TestAddActivity_ReturnsActivity(t *testing.T) {
	s := newTestStore(t)
	task, _ := s.Create("test", "", "")
	act, err := s.AddActivity(task.ID, "task.started", map[string]interface{}{"reason": "test"})
	if err != nil {
		t.Fatal(err)
	}
	if act.ID == 0 || act.TaskID != task.ID || act.EventType != "task.started" {
		t.Errorf("unexpected activity: %+v", act)
	}
}

func TestInsertComment_ReturnsComment(t *testing.T) {
	s := newTestStore(t)
	task, _ := s.Create("test", "", "")
	cmt, err := s.InsertComment(task.ID, "user", "feedback", "looks good")
	if err != nil {
		t.Fatal(err)
	}
	if cmt.ID == 0 || cmt.TaskID != task.ID || cmt.Body != "looks good" {
		t.Errorf("unexpected comment: %+v", cmt)
	}
}

func TestOnChange_AddActivity(t *testing.T) {
	s := newTestStore(t)
	task, _ := s.Create("test", "", "")
	var gotEvent string
	s.OnChange = func(event string, payload any) {
		gotEvent = event
	}
	s.AddActivity(task.ID, "test.event", nil)
	if gotEvent != "task.activity" {
		t.Errorf("event = %q, want task.activity", gotEvent)
	}
}

func TestOnChange_InsertComment(t *testing.T) {
	s := newTestStore(t)
	task, _ := s.Create("test", "", "")
	var gotEvent string
	s.OnChange = func(event string, payload any) {
		gotEvent = event
	}
	s.InsertComment(task.ID, "user", "feedback", "test")
	if gotEvent != "task.comment" {
		t.Errorf("event = %q, want task.comment", gotEvent)
	}
}

func TestCreate_SetsSeq(t *testing.T) {
	s := newTestStore(t)
	t1, _ := s.Create("first", "", "")
	t2, _ := s.Create("second", "", "")
	var seq1, seq2 int64
	s.db.QueryRow("SELECT seq FROM tasks WHERE id = ?", t1.ID).Scan(&seq1)
	s.db.QueryRow("SELECT seq FROM tasks WHERE id = ?", t2.ID).Scan(&seq2)
	if seq1 == 0 || seq2 == 0 {
		t.Errorf("seq should be nonzero: seq1=%d seq2=%d", seq1, seq2)
	}
	if seq2 <= seq1 {
		t.Errorf("seq2 (%d) should be > seq1 (%d)", seq2, seq1)
	}
}

func TestUpdate_BumpsSeq(t *testing.T) {
	s := newTestStore(t)
	task, _ := s.Create("test", "", "")
	var seqBefore int64
	s.db.QueryRow("SELECT seq FROM tasks WHERE id = ?", task.ID).Scan(&seqBefore)
	s.Update(task.ID, map[string]interface{}{"title": "updated"})
	var seqAfter int64
	s.db.QueryRow("SELECT seq FROM tasks WHERE id = ?", task.ID).Scan(&seqAfter)
	if seqAfter <= seqBefore {
		t.Errorf("seq should increase: before=%d after=%d", seqBefore, seqAfter)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/store/ -run "TestNextSeq|TestOnChange|TestAddActivity_Returns|TestInsertComment_Returns|TestCreate_Sets|TestUpdate_Bumps" -v`
Expected: FAIL — `OnChange` field, `nextSeqTx` method not defined, signature mismatches

- [ ] **Step 3: Implement OnChange + nextSeq + wire into mutations**

Modify `store.go`:

**a) Add `OnChange` field to `Store` struct:**
```go
type Store struct {
	db       *sql.DB
	dbPath   string
	OnChange func(event string, payload any)
}
```

**b) Add `nextSeqTx` helper (operates within a transaction):**
```go
// nextSeqTx increments the global sequence counter within tx and returns the new value.
func (s *Store) nextSeqTx(tx *sql.Tx) (int64, error) {
	var seq int64
	err := tx.QueryRow("UPDATE sync_meta SET value = value + 1 WHERE key = 'seq' RETURNING value").Scan(&seq)
	if err != nil {
		return 0, fmt.Errorf("tasks: next seq: %w", err)
	}
	return seq, nil
}
```

**c) Rewrite `Create` to use transaction + stamp seq + fire OnChange:**
```go
func (s *Store) Create(title, description, product string) (*Task, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("tasks: create begin: %w", err)
	}
	defer tx.Rollback()

	seq, err := s.nextSeqTx(tx)
	if err != nil {
		return nil, err
	}

	res, err := tx.Exec(
		"INSERT INTO tasks (title, description, product, seq) VALUES (?, ?, ?, ?)",
		title, description, product, seq,
	)
	if err != nil {
		return nil, fmt.Errorf("tasks: create: %w", err)
	}
	id, _ := res.LastInsertId()

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("tasks: create commit: %w", err)
	}

	task, err := s.Get(id)
	if err != nil {
		return nil, err
	}

	if s.OnChange != nil {
		s.OnChange("task.created", task)
	}
	return task, nil
}
```

**d) Rewrite `Update` to use transaction + stamp seq + classify event + fire OnChange:**
```go
func (s *Store) Update(id int64, fields map[string]interface{}) (*Task, error) {
	if stage, ok := fields["stage"]; ok {
		if sv, ok := stage.(string); ok && !validStages[sv] {
			return nil, fmt.Errorf("tasks: invalid stage: %q", sv)
		}
	}

	var setClauses []string
	var args []interface{}
	allowed := map[string]bool{"title": true, "description": true, "stage": true, "workflow": true, "product": true, "substep": true, "metadata": true}

	for k, v := range fields {
		if !allowed[k] {
			continue
		}
		setClauses = append(setClauses, k+" = ?")
		args = append(args, v)
	}
	if len(setClauses) == 0 {
		return s.Get(id)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("tasks: update begin: %w", err)
	}
	defer tx.Rollback()

	seq, err := s.nextSeqTx(tx)
	if err != nil {
		return nil, err
	}

	setClauses = append(setClauses, "seq = ?", "updated_at = CURRENT_TIMESTAMP")
	args = append(args, seq, id)

	result, err := tx.Exec(
		"UPDATE tasks SET "+strings.Join(setClauses, ", ")+" WHERE id = ?",
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("tasks: update: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, fmt.Errorf("tasks: not found: %d", id)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("tasks: update commit: %w", err)
	}

	task, err := s.Get(id)
	if err != nil {
		return nil, err
	}

	if s.OnChange != nil {
		// Priority: stage_changed > substep_changed > updated
		event := "task.updated"
		if _, ok := fields["stage"]; ok {
			event = "task.stage_changed"
		} else if _, ok := fields["substep"]; ok {
			event = "task.substep_changed"
		}
		s.OnChange(event, task)
	}
	return task, nil
}
```

**e) Rewrite `Delete` to use transaction + tombstone + fire OnChange:**
```go
func (s *Store) Delete(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("tasks: delete begin: %w", err)
	}
	defer tx.Rollback()

	seq, err := s.nextSeqTx(tx)
	if err != nil {
		return err
	}

	result, err := tx.Exec("DELETE FROM tasks WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("tasks: delete: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("tasks: not found: %d", id)
	}

	if _, err := tx.Exec("INSERT INTO task_tombstones (id, seq) VALUES (?, ?)", id, seq); err != nil {
		return fmt.Errorf("tasks: tombstone insert: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("tasks: delete commit: %w", err)
	}

	if s.OnChange != nil {
		s.OnChange("task.deleted", TaskDeleted{ID: id})
	}
	return nil
}
```

**f) Rewrite `AddActivity` to return `(Activity, error)` + fire OnChange:**
```go
func (s *Store) AddActivity(taskID int64, eventType string, data map[string]interface{}) (Activity, error) {
	dataJSON := "{}"
	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			return Activity{}, fmt.Errorf("tasks: marshal activity data: %w", err)
		}
		dataJSON = string(b)
	}
	res, err := s.db.Exec(
		"INSERT INTO task_activity (task_id, event_type, data) VALUES (?, ?, ?)",
		taskID, eventType, dataJSON,
	)
	if err != nil {
		return Activity{}, fmt.Errorf("tasks: add activity: %w", err)
	}
	id, _ := res.LastInsertId()

	var act Activity
	err = s.db.QueryRow(
		"SELECT id, task_id, event_type, data, created_at FROM task_activity WHERE id = ?", id,
	).Scan(&act.ID, &act.TaskID, &act.EventType, &act.Data, &act.CreatedAt)
	if err != nil {
		return Activity{}, fmt.Errorf("tasks: read inserted activity: %w", err)
	}

	if s.OnChange != nil {
		s.OnChange("task.activity", TaskActivity{TaskID: taskID, Activity: act})
	}
	return act, nil
}
```

**g) Rewrite `InsertComment` to return `(Comment, error)` + fire OnChange:**
```go
func (s *Store) InsertComment(taskID int64, author, typ, body string) (Comment, error) {
	createdAt := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		"INSERT INTO task_comments (task_id, author, type, body, created_at) VALUES (?, ?, ?, ?, ?)",
		taskID, author, typ, body, createdAt,
	)
	if err != nil {
		return Comment{}, fmt.Errorf("tasks: insert comment: %w", err)
	}
	id, _ := res.LastInsertId()

	cmt := Comment{
		ID:        id,
		TaskID:    taskID,
		Author:    author,
		Type:      typ,
		Body:      body,
		CreatedAt: createdAt,
	}

	if s.OnChange != nil {
		s.OnChange("task.comment", TaskComment{TaskID: taskID, Comment: cmt})
	}
	return cmt, nil
}
```

**h) Update `Get` to also scan the `seq` column.** Add `Seq int64 \`json:"seq"\`` to the `Task` struct and update all `SELECT` and `Scan` calls that read tasks:

- `Get()`: add `seq` to SELECT + Scan
- `List()`: add `seq` to SELECT + Scan
- `NextReady()`: add `seq` to SELECT + Scan

- [ ] **Step 4: Fix all callers of AddActivity and InsertComment**

`AddActivity` changes from `error` to `(Activity, error)`. `InsertComment` changes from `(int64, error)` to `(Comment, error)`. All callers must be updated.

**Exhaustive caller list:**

**AddActivity callers (13 sites):**
- `internal/tasks/server/server.go:159` — bare call `s.store.AddActivity(...)` → change to `_, _ = s.store.AddActivity(...)`
- `internal/tasks/executor/executor.go:82` — `if err := e.store.AddActivity(...)` → `if _, err := e.store.AddActivity(...)`
- `internal/tasks/executor/executor.go:109` — same pattern
- `internal/tasks/executor/executor.go:150` — `_ = e.store.AddActivity(...)` → `_, _ = e.store.AddActivity(...)`
- `internal/tasks/executor/executor.go:178,194,200,211,228,246,254,263` — same patterns (mix of `_ =` and bare calls)

**InsertComment callers (7 sites):**
- `internal/tasks/server/server.go:380` — `commentID, err := s.store.InsertComment(...)` → `cmt, err := s.store.InsertComment(...)`, then use `cmt.ID` instead of `commentID`
- `internal/tasks/watcher/watcher.go:72,85,96,134,153` — all use `_, err :=` pattern → change first `_` type is now `Comment` not `int64` — the discard still compiles, no change needed

**Test callers (also need fixing):**
- `internal/tasks/store/store_test.go:307` — `id, err := s.InsertComment(...)` → `cmt, err := s.InsertComment(...)`, use `cmt.ID`
- `internal/tasks/store/store_test.go:136` — `err := s.AddActivity(...)` → `_, err := s.AddActivity(...)`
- `internal/tasks/watcher/watcher_test.go:147` — `id3, _ := s.InsertComment(...)` → `cmt3, _ := s.InsertComment(...)`, use `cmt3.ID`

Run: `grep -rn '\.AddActivity\|\.InsertComment' /home/rishav/soul-v2/internal/ --include='*.go'` to verify all sites are covered.

- [ ] **Step 5: Run all tests**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/... -v -count=1`
Expected: All PASS

- [ ] **Step 6: Run static verification**

Run: `cd /home/rishav/soul-v2 && go vet ./internal/tasks/...`
Expected: no errors

- [ ] **Step 7: Commit**

```bash
git add internal/tasks/store/store.go internal/tasks/store/store_test.go
git commit -m "feat(tasks): OnChange hook + nextSeq + wire all mutations"
```

---

## Task 5: Delta Query Methods

**Files:**
- Modify: `internal/tasks/store/store.go`
- Modify: `internal/tasks/store/store_test.go`

Adds: `ListModifiedSince`, `ListDeletedSince`, `MaxSeq`, `PruneTombstones`, `AllCommentsAfterID`, `ActivityAfterID`

- [ ] **Step 1: Write tests for all delta methods**

Add to `store_test.go`:

```go
func TestListModifiedSince(t *testing.T) {
	s := newTestStore(t)
	t1, _ := s.Create("first", "", "")
	t2, _ := s.Create("second", "", "")

	var seq1 int64
	s.db.QueryRow("SELECT seq FROM tasks WHERE id = ?", t1.ID).Scan(&seq1)

	tasks, err := s.ListModifiedSince(seq1)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].ID != t2.ID {
		t.Errorf("expected only second task, got %d tasks", len(tasks))
	}
}

func TestListDeletedSince(t *testing.T) {
	s := newTestStore(t)
	task, _ := s.Create("doomed", "", "")
	seq, _ := s.MaxSeq()
	s.Delete(task.ID)

	deleted, err := s.ListDeletedSince(seq)
	if err != nil {
		t.Fatal(err)
	}
	if len(deleted) != 1 || deleted[0] != task.ID {
		t.Errorf("expected [%d], got %v", task.ID, deleted)
	}
}

func TestMaxSeq(t *testing.T) {
	s := newTestStore(t)
	seq0, _ := s.MaxSeq()
	if seq0 != 0 {
		t.Errorf("initial MaxSeq = %d, want 0", seq0)
	}
	s.Create("test", "", "")
	seq1, _ := s.MaxSeq()
	if seq1 != 1 {
		t.Errorf("after create MaxSeq = %d, want 1", seq1)
	}
}

func TestPruneTombstones(t *testing.T) {
	s := newTestStore(t)
	task, _ := s.Create("old", "", "")
	s.Delete(task.ID)

	// Backdate the tombstone to 25h ago.
	s.db.Exec("UPDATE task_tombstones SET deleted_at = datetime('now', '-25 hours') WHERE id = ?", task.ID)

	pruned, err := s.PruneTombstones()
	if err != nil {
		t.Fatal(err)
	}
	if pruned != 1 {
		t.Errorf("pruned = %d, want 1", pruned)
	}

	// Verify it's gone.
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM task_tombstones").Scan(&count)
	if count != 0 {
		t.Errorf("tombstones remaining = %d, want 0", count)
	}
}

func TestPruneTombstones_PreservesRecent(t *testing.T) {
	s := newTestStore(t)
	task, _ := s.Create("recent", "", "")
	s.Delete(task.ID)

	pruned, _ := s.PruneTombstones()
	if pruned != 0 {
		t.Errorf("should not prune recent tombstone, pruned %d", pruned)
	}
}

func TestAllCommentsAfterID(t *testing.T) {
	s := newTestStore(t)
	task, _ := s.Create("test", "", "")
	s.InsertComment(task.ID, "user", "feedback", "first")
	cmt2, _ := s.InsertComment(task.ID, "agent", "response", "second")
	cmt3, _ := s.InsertComment(task.ID, "user", "feedback", "third")

	// Get comments after the first one.
	comments, err := s.AllCommentsAfterID(task.ID, cmt2.ID-1)
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}
	// Should be ascending order.
	if comments[0].ID != cmt2.ID || comments[1].ID != cmt3.ID {
		t.Errorf("unexpected order: %d, %d", comments[0].ID, comments[1].ID)
	}
	// Should include agent comments (not just user).
	if comments[0].Author != "agent" {
		t.Errorf("expected agent comment, got %q", comments[0].Author)
	}
}

func TestActivityAfterID(t *testing.T) {
	s := newTestStore(t)
	task, _ := s.Create("test", "", "")
	act1, _ := s.AddActivity(task.ID, "task.created", nil)
	act2, _ := s.AddActivity(task.ID, "task.started", nil)

	activities, err := s.ActivityAfterID(task.ID, act1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(activities) != 1 || activities[0].ID != act2.ID {
		t.Errorf("expected 1 activity (act2), got %d", len(activities))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/store/ -run "TestList(Modified|Deleted)|TestMaxSeq|TestPrune|TestAllComments|TestActivityAfter" -v`
Expected: FAIL — methods don't exist

- [ ] **Step 3: Implement delta methods**

Add to `store.go`:

```go
// MaxSeq returns the current global sequence number.
func (s *Store) MaxSeq() (int64, error) {
	var seq int64
	err := s.db.QueryRow("SELECT value FROM sync_meta WHERE key = 'seq'").Scan(&seq)
	if err != nil {
		return 0, fmt.Errorf("tasks: max seq: %w", err)
	}
	return seq, nil
}

// ListModifiedSince returns all tasks with seq > the given value.
func (s *Store) ListModifiedSince(seq int64) ([]Task, error) {
	rows, err := s.db.Query(
		"SELECT id, title, description, stage, workflow, product, substep, metadata, seq, created_at, updated_at FROM tasks WHERE seq > ? ORDER BY seq ASC",
		seq,
	)
	if err != nil {
		return nil, fmt.Errorf("tasks: list modified since: %w", err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Stage, &t.Workflow, &t.Product, &t.Substep, &t.Metadata, &t.Seq, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("tasks: scan modified: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// ListDeletedSince returns IDs of tasks deleted after the given seq.
func (s *Store) ListDeletedSince(seq int64) ([]int64, error) {
	rows, err := s.db.Query("SELECT id FROM task_tombstones WHERE seq > ?", seq)
	if err != nil {
		return nil, fmt.Errorf("tasks: list deleted since: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("tasks: scan tombstone: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// PruneTombstones removes tombstones older than 24 hours. Returns count removed.
func (s *Store) PruneTombstones() (int64, error) {
	result, err := s.db.Exec("DELETE FROM task_tombstones WHERE deleted_at < datetime('now', '-24 hours')")
	if err != nil {
		return 0, fmt.Errorf("tasks: prune tombstones: %w", err)
	}
	n, _ := result.RowsAffected()
	return n, nil
}

// AllCommentsAfterID returns all comments (any author) for a task with id > lastID, ascending.
func (s *Store) AllCommentsAfterID(taskID, lastID int64) ([]Comment, error) {
	rows, err := s.db.Query(
		"SELECT id, task_id, author, type, body, created_at FROM task_comments WHERE task_id = ? AND id > ? ORDER BY id ASC",
		taskID, lastID,
	)
	if err != nil {
		return nil, fmt.Errorf("tasks: all comments after: %w", err)
	}
	defer rows.Close()

	var comments []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.TaskID, &c.Author, &c.Type, &c.Body, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("tasks: scan comment: %w", err)
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

// ActivityAfterID returns activity entries for a task with id > lastID, ascending.
func (s *Store) ActivityAfterID(taskID, lastID int64) ([]Activity, error) {
	rows, err := s.db.Query(
		"SELECT id, task_id, event_type, data, created_at FROM task_activity WHERE task_id = ? AND id > ? ORDER BY id ASC",
		taskID, lastID,
	)
	if err != nil {
		return nil, fmt.Errorf("tasks: activity after: %w", err)
	}
	defer rows.Close()

	var activities []Activity
	for rows.Next() {
		var a Activity
		if err := rows.Scan(&a.ID, &a.TaskID, &a.EventType, &a.Data, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("tasks: scan activity: %w", err)
		}
		activities = append(activities, a)
	}
	return activities, rows.Err()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/store/ -v -count=1`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tasks/store/store.go internal/tasks/store/store_test.go
git commit -m "feat(tasks): delta query methods — ListModifiedSince, tombstones, AllComments, ActivityAfter"
```

---

## Task 6: Server — Sync Endpoint + Wire OnChange + Delta Params + Pruning

**Files:**
- Modify: `internal/tasks/server/server.go`
- Create: `internal/tasks/server/server_test.go`

- [ ] **Step 1: Write server tests**

```go
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rishav1305/soul-v2/internal/tasks/store"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	srv := New(WithStore(s))
	return srv
}

func TestSyncEndpoint_FullSync(t *testing.T) {
	srv := testServer(t)
	srv.store.Create("task1", "", "")
	srv.store.Create("task2", "", "")

	req := httptest.NewRequest("GET", "/api/tasks/sync", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp struct {
		Tasks    []store.Task `json:"tasks"`
		Deleted  []int64      `json:"deleted"`
		Cursor   string       `json:"cursor"`
		FullSync bool         `json:"fullSync"`
	}
	json.NewDecoder(w.Body).Decode(&resp)

	if !resp.FullSync {
		t.Error("expected fullSync=true for no cursor")
	}
	if len(resp.Tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(resp.Tasks))
	}
	if resp.Cursor == "" {
		t.Error("expected non-empty cursor")
	}
}

func TestSyncEndpoint_DeltaSync(t *testing.T) {
	srv := testServer(t)
	srv.store.Create("task1", "", "")

	// Full sync to get cursor.
	req1 := httptest.NewRequest("GET", "/api/tasks/sync", nil)
	w1 := httptest.NewRecorder()
	srv.ServeHTTP(w1, req1)
	var resp1 struct{ Cursor string `json:"cursor"` }
	json.NewDecoder(w1.Body).Decode(&resp1)

	// Create another task.
	srv.store.Create("task2", "", "")

	// Delta sync.
	req2 := httptest.NewRequest("GET", "/api/tasks/sync?cursor="+resp1.Cursor, nil)
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)

	var resp2 struct {
		Tasks    []store.Task `json:"tasks"`
		FullSync bool         `json:"fullSync"`
	}
	json.NewDecoder(w2.Body).Decode(&resp2)

	if resp2.FullSync {
		t.Error("expected fullSync=false for delta sync")
	}
	if len(resp2.Tasks) != 1 || resp2.Tasks[0].Title != "task2" {
		t.Errorf("expected 1 delta task (task2), got %d", len(resp2.Tasks))
	}
}

func TestSyncEndpoint_StaleCursor(t *testing.T) {
	srv := testServer(t)
	srv.store.Create("task1", "", "")

	// Encode a cursor with ts 25 hours ago.
	stale := store.EncodeCursor(1, 0) // ts=0 (epoch) is definitely >24h ago
	req := httptest.NewRequest("GET", "/api/tasks/sync?cursor="+stale, nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var resp struct{ FullSync bool `json:"fullSync"` }
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.FullSync {
		t.Error("expected fullSync=true for stale cursor")
	}
}

func TestActivityEndpoint_AfterParam(t *testing.T) {
	srv := testServer(t)
	task, _ := srv.store.Create("test", "", "")
	act1, _ := srv.store.AddActivity(task.ID, "evt1", nil)
	srv.store.AddActivity(task.ID, "evt2", nil)

	req := httptest.NewRequest("GET", "/api/tasks/1/activity?after="+strings.Itoa(int(act1.ID)), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var activities []store.Activity
	json.NewDecoder(w.Body).Decode(&activities)
	if len(activities) != 1 || activities[0].EventType != "evt2" {
		t.Errorf("expected 1 activity (evt2), got %d", len(activities))
	}
}

func TestCommentsEndpoint_AfterParam(t *testing.T) {
	srv := testServer(t)
	task, _ := srv.store.Create("test", "", "")
	cmt1, _ := srv.store.InsertComment(task.ID, "user", "feedback", "first")
	srv.store.InsertComment(task.ID, "agent", "response", "second")

	req := httptest.NewRequest("GET", "/api/tasks/1/comments?after="+strings.Itoa(int(cmt1.ID)), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var comments []store.Comment
	json.NewDecoder(w.Body).Decode(&comments)
	if len(comments) != 1 || comments[0].Body != "second" {
		t.Errorf("expected 1 comment (second), got %d", len(comments))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/server/ -run "TestSync|TestActivity|TestComments" -v`
Expected: FAIL — `/api/tasks/sync` route not registered

- [ ] **Step 3: Implement server changes**

Modify `server.go`:

**a) Register sync route** — add after the existing routes in `New()`:
```go
s.mux.HandleFunc("GET /api/tasks/sync", s.handleSync)
```

**b) Wire `OnChange` → `Broadcast`** — add at end of `New()` before middleware chain:
```go
	if s.store != nil {
		s.store.OnChange = func(event string, payload any) {
			data, _ := json.Marshal(payload)
			s.broadcaster.Broadcast(Event{Type: event, Data: string(data)})
		}
	}
```

**c) Add `handleSync` handler:**
```go
func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	cursorParam := r.URL.Query().Get("cursor")
	seq, ts, err := store.DecodeCursor(cursorParam)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid cursor"})
		return
	}

	now := time.Now().Unix()

	// Full sync if no cursor or stale (>24h).
	if cursorParam == "" || (now-ts > 24*3600) {
		tasks, err := s.store.List("", "")
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if tasks == nil {
			tasks = []store.Task{}
		}
		maxSeq, _ := s.store.MaxSeq()
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"tasks":    tasks,
			"deleted":  []int64{},
			"cursor":   store.EncodeCursor(maxSeq, now),
			"fullSync": true,
		})
		return
	}

	// Snapshot current seq before query.
	currentSeq, _ := s.store.MaxSeq()

	tasks, err := s.store.ListModifiedSince(seq)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if tasks == nil {
		tasks = []store.Task{}
	}

	deleted, err := s.store.ListDeletedSince(seq)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if deleted == nil {
		deleted = []int64{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tasks":    tasks,
		"deleted":  deleted,
		"cursor":   store.EncodeCursor(currentSeq, now),
		"fullSync": false,
	})
}
```

**d) Update `handleTaskActivity` to support `?after=` param:**
```go
func (s *Server) handleTaskActivity(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	afterStr := r.URL.Query().Get("after")
	if afterStr != "" {
		afterID, err := strconv.ParseInt(afterStr, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid after param"})
			return
		}
		activities, err := s.store.ActivityAfterID(id, afterID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if activities == nil {
			activities = []store.Activity{}
		}
		writeJSON(w, http.StatusOK, activities)
		return
	}

	activities, err := s.store.ListActivity(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if activities == nil {
		activities = []store.Activity{}
	}
	writeJSON(w, http.StatusOK, activities)
}
```

**e) Update `handleListComments` to support `?after=` param:**
```go
func (s *Server) handleListComments(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	afterStr := r.URL.Query().Get("after")
	if afterStr != "" {
		afterID, err := strconv.ParseInt(afterStr, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid after param"})
			return
		}
		comments, err := s.store.AllCommentsAfterID(id, afterID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if comments == nil {
			comments = []store.Comment{}
		}
		writeJSON(w, http.StatusOK, comments)
		return
	}

	comments, err := s.store.GetComments(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if comments == nil {
		comments = []store.Comment{}
	}
	writeJSON(w, http.StatusOK, comments)
}
```

**f) Remove manual `Broadcast` calls** from `handleCreateTask`, `handleUpdateTask`, `handleStartTask`. The `OnChange` hook now handles broadcasting. In `handleCreateTask`, also update the `AddActivity` call to handle the new `(Activity, error)` return.

**g) Add tombstone pruning goroutine with shutdown** — add a `pruneStop chan struct{}` field to `Server`, initialize in `New()`, and wire into `Start()` and `Shutdown()`:

Add field to Server struct:
```go
pruneStop chan struct{}
```

Initialize in `New()`:
```go
s := &Server{
	// ... existing fields ...
	pruneStop: make(chan struct{}),
}
```

Update `Start()`:
```go
func (s *Server) Start() error {
	if s.store != nil {
		s.store.PruneTombstones()
		go func() {
			ticker := time.NewTicker(1 * time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					s.store.PruneTombstones()
				case <-s.pruneStop:
					return
				}
			}
		}()
	}
	log.Printf("soul-tasks listening on http://%s", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}
```

Update `Shutdown()`:
```go
func (s *Server) Shutdown(ctx context.Context) error {
	close(s.pruneStop)
	return s.httpServer.Shutdown(ctx)
}
```
```

- [ ] **Step 4: Run server tests**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/server/ -v -count=1`
Expected: All PASS

- [ ] **Step 5: Run full backend tests**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/... -v -count=1`
Expected: All PASS

- [ ] **Step 6: Run go vet**

Run: `cd /home/rishav/soul-v2 && go vet ./internal/tasks/...`
Expected: no errors

- [ ] **Step 7: Commit**

```bash
git add internal/tasks/server/server.go internal/tasks/server/server_test.go
git commit -m "feat(tasks): sync endpoint, OnChange→Broadcast wiring, delta params, tombstone pruning"
```

---

## Task 7: Frontend — WS Event Forwarding in useChat.ts

**Files:**
- Modify: `web/src/hooks/useChat.ts`

- [ ] **Step 1: Add task event forwarding**

In `useChat.ts`, find the `handleMessage` callback (line 177). Add the following **before** the `switch (type)` block (before `switch (type) {` at line 178):

```typescript
    // Forward task events to useTaskSync via DOM event.
    // Task events are not in OutboundMessageType — check raw string before typed switch.
    if ((type as string)?.startsWith('task.')) {
      window.dispatchEvent(new CustomEvent('ws:task-event', { detail: { type, data } }));
      return;
    }
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /home/rishav/soul-v2/web && npx tsc --noEmit`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add web/src/hooks/useChat.ts
git commit -m "feat(tasks): forward task.* WS events via CustomEvent for useTaskSync"
```

---

## Task 8: Frontend — Update Task Type + useTaskSync Hook

**Files:**
- Modify: `web/src/lib/types.ts`
- Create: `web/src/hooks/useTaskSync.ts`

This is the largest frontend task. It updates the Task type, then implements the unified hook.

- [ ] **Step 1: Add `seq` and `substep` to the Task interface**

In `web/src/lib/types.ts`, the `Task` interface is in the manual types section (line 424). Add `seq` and `substep`:

```typescript
/** tasks */
export interface Task {
  id: number;
  title: string;
  description: string;
  stage: TaskStage;
  workflow: string;
  product: string;
  substep: string;
  metadata: string;
  seq: number;
  createdAt: string;
  updatedAt: string;
}
```

- [ ] **Step 2: Verify TypeScript compiles with new fields**

Run: `cd /home/rishav/soul-v2/web && npx tsc --noEmit`
Expected: no errors (new optional fields won't break existing usage)

- [ ] **Step 3: Create the useTaskSync hook**

```typescript
import { useState, useEffect, useCallback, useRef } from 'react';
import type { Task, TaskActivity, TaskStage } from '../lib/types';
import { api } from '../lib/api';
import { reportError, reportUsage } from '../lib/telemetry';

export interface CreateTaskInput {
  title: string;
  description?: string;
  product?: string;
}

interface Comment {
  id: number;
  taskId: number;
  author: string;
  type: string;
  body: string;
  createdAt: string;
}

interface SyncResponse {
  tasks: Task[];
  deleted: number[];
  cursor: string;
  fullSync: boolean;
}

interface UseTaskSyncOptions {
  taskId?: number;
  mode?: 'kanban' | 'detail';
}

interface UseTaskSyncReturn {
  tasks: Task[];
  task: Task | null;
  activities: TaskActivity[];
  comments: Comment[];
  loading: boolean;
  error: string | null;
  connected: boolean;
  refresh: () => void;
  createTask: (input: CreateTaskInput) => Promise<Task>;
  updateTask: (id: number, fields: Partial<Task>) => Promise<Task>;
  deleteTask: (id: number) => Promise<void>;
  startTask: (id: number) => Promise<void>;
  stopTask: (id: number) => Promise<void>;
  addComment: (id: number, body: string) => Promise<void>;
}

export function useTaskSync(options?: UseTaskSyncOptions): UseTaskSyncReturn {
  const mode = options?.mode ?? 'kanban';
  const taskId = options?.taskId;

  const [taskMap, setTaskMap] = useState<Map<number, Task>>(new Map());
  const [activities, setActivities] = useState<TaskActivity[]>([]);
  const [comments, setComments] = useState<Comment[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [connected, setConnected] = useState(false);

  const cursorRef = useRef<string>('');
  const lastActivityIdRef = useRef<number>(0);
  const lastCommentIdRef = useRef<number>(0);
  const inflightRef = useRef(false);

  // --- Helpers ---

  const applyTaskUpdate = useCallback((task: Task) => {
    setTaskMap(prev => {
      const existing = prev.get(task.id);
      if (existing && existing.seq >= task.seq) return prev;
      const next = new Map(prev);
      next.set(task.id, task);
      return next;
    });
  }, []);

  const applyTaskDelete = useCallback((id: number) => {
    setTaskMap(prev => {
      if (!prev.has(id)) return prev;
      const next = new Map(prev);
      next.delete(id);
      return next;
    });
  }, []);

  const appendActivity = useCallback((act: TaskActivity) => {
    setActivities(prev => {
      if (prev.some(a => a.id === act.id)) return prev;
      return [...prev, act];
    });
    if (act.id > lastActivityIdRef.current) {
      lastActivityIdRef.current = act.id;
    }
  }, []);

  const appendComment = useCallback((cmt: Comment) => {
    setComments(prev => {
      if (prev.some(c => c.id === cmt.id)) return prev;
      return [...prev, cmt];
    });
    if (cmt.id > lastCommentIdRef.current) {
      lastCommentIdRef.current = cmt.id;
    }
  }, []);

  // --- WS Event Listener ---

  useEffect(() => {
    const handler = (event: Event) => {
      const { type, data } = (event as CustomEvent).detail;
      setConnected(true);

      switch (type) {
        case 'task.created':
        case 'task.updated':
        case 'task.stage_changed':
        case 'task.substep_changed': {
          const task = typeof data === 'string' ? JSON.parse(data) : data;
          applyTaskUpdate(task);
          break;
        }
        case 'task.deleted': {
          const payload = typeof data === 'string' ? JSON.parse(data) : data;
          applyTaskDelete(payload.id);
          break;
        }
        case 'task.activity': {
          const payload = typeof data === 'string' ? JSON.parse(data) : data;
          if (!taskId || payload.taskId === taskId) {
            appendActivity(payload.activity);
          }
          break;
        }
        case 'task.comment': {
          const payload = typeof data === 'string' ? JSON.parse(data) : data;
          if (!taskId || payload.taskId === taskId) {
            appendComment(payload.comment);
          }
          break;
        }
      }
    };

    window.addEventListener('ws:task-event', handler);
    return () => window.removeEventListener('ws:task-event', handler);
  }, [taskId, applyTaskUpdate, applyTaskDelete, appendActivity, appendComment]);

  // --- Initial Fetch ---

  const doFullSync = useCallback(async () => {
    try {
      const resp = await api.get<SyncResponse>('/api/tasks/sync');
      const map = new Map<number, Task>();
      for (const t of resp.tasks) map.set(t.id, t);
      setTaskMap(map);
      cursorRef.current = resp.cursor;
      setError(null);

      if (mode === 'detail' && taskId) {
        const [acts, cmts] = await Promise.all([
          api.get<TaskActivity[]>(`/api/tasks/${taskId}/activity`),
          api.get<Comment[]>(`/api/tasks/${taskId}/comments`),
        ]);
        setActivities(acts);
        setComments(cmts);
        if (acts.length > 0) lastActivityIdRef.current = acts[acts.length - 1]!.id;
        if (cmts.length > 0) lastCommentIdRef.current = cmts[cmts.length - 1]!.id;
      }
    } catch (err) {
      reportError('useTaskSync.fullSync', err);
      setError(err instanceof Error ? err.message : 'Sync failed');
    } finally {
      setLoading(false);
    }
  }, [mode, taskId]);

  useEffect(() => { doFullSync(); }, [doFullSync]);

  // --- Heartbeat ---

  useEffect(() => {
    const interval = mode === 'detail' ? 5000 : 30000;

    const tick = async () => {
      if (inflightRef.current) return;
      inflightRef.current = true;

      try {
        if (mode === 'kanban') {
          const resp = await api.get<SyncResponse>(
            `/api/tasks/sync?cursor=${encodeURIComponent(cursorRef.current)}`
          );
          if (resp.fullSync) {
            const map = new Map<number, Task>();
            for (const t of resp.tasks) map.set(t.id, t);
            setTaskMap(map);
          } else {
            for (const t of resp.tasks) applyTaskUpdate(t);
            for (const id of resp.deleted) applyTaskDelete(id);
          }
          cursorRef.current = resp.cursor;
        } else if (mode === 'detail' && taskId) {
          const [taskResp, acts, cmts] = await Promise.all([
            api.get<Task>(`/api/tasks/${taskId}`),
            api.get<TaskActivity[]>(`/api/tasks/${taskId}/activity?after=${lastActivityIdRef.current}`),
            api.get<Comment[]>(`/api/tasks/${taskId}/comments?after=${lastCommentIdRef.current}`),
          ]);
          applyTaskUpdate(taskResp);
          for (const a of acts) appendActivity(a);
          for (const c of cmts) appendComment(c);
        }
        setError(null);
      } catch (err) {
        reportError('useTaskSync.heartbeat', err);
        setError(err instanceof Error ? err.message : 'Heartbeat failed');
      } finally {
        inflightRef.current = false;
      }
    };

    const id = setInterval(tick, interval);

    // Tab visibility: pause when hidden, sync on focus.
    const onVisibility = () => {
      if (document.visibilityState === 'visible') tick();
    };
    document.addEventListener('visibilitychange', onVisibility);

    return () => {
      clearInterval(id);
      document.removeEventListener('visibilitychange', onVisibility);
    };
  }, [mode, taskId, applyTaskUpdate, applyTaskDelete, appendActivity, appendComment]);

  // --- Actions ---

  const createTask = useCallback(async (input: CreateTaskInput) => {
    const task = await api.post<Task>('/api/tasks', input);
    applyTaskUpdate(task);
    reportUsage('task.create', { taskId: task.id });
    return task;
  }, [applyTaskUpdate]);

  const updateTask = useCallback(async (id: number, fields: Partial<Task>) => {
    const task = await api.patch<Task>(`/api/tasks/${id}`, fields);
    applyTaskUpdate(task);
    reportUsage('task.update', { taskId: id, fields: Object.keys(fields) });
    return task;
  }, [applyTaskUpdate]);

  const deleteTask = useCallback(async (id: number) => {
    applyTaskDelete(id); // optimistic
    try {
      await api.delete(`/api/tasks/${id}`);
      reportUsage('task.delete', { taskId: id });
    } catch (err) {
      // Rollback: re-fetch
      doFullSync();
      throw err;
    }
  }, [applyTaskDelete, doFullSync]);

  const startTask = useCallback(async (id: number) => {
    await api.post(`/api/tasks/${id}/start`);
    reportUsage('task.start', { taskId: id });
    // WS event or next heartbeat will update state.
  }, []);

  const stopTask = useCallback(async (id: number) => {
    await api.post(`/api/tasks/${id}/stop`);
    reportUsage('task.stop', { taskId: id });
  }, []);

  const addComment = useCallback(async (id: number, body: string) => {
    await api.post(`/api/tasks/${id}/comments`, { author: 'user', type: 'feedback', body });
    reportUsage('task.addComment', { taskId: id });
  }, []);

  const refresh = useCallback(() => { doFullSync(); }, [doFullSync]);

  // --- Derived state ---

  const tasks = Array.from(taskMap.values());
  const task = taskId ? taskMap.get(taskId) ?? null : null;

  return {
    tasks,
    task,
    activities,
    comments,
    loading,
    error,
    connected,
    refresh,
    createTask,
    updateTask,
    deleteTask,
    startTask,
    stopTask,
    addComment,
  };
}
```

- [ ] **Step 4: Verify TypeScript compiles**

Run: `cd /home/rishav/soul-v2/web && npx tsc --noEmit`
Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/types.ts web/src/hooks/useTaskSync.ts
git commit -m "feat(tasks): update Task type + useTaskSync hook — WS events + heartbeat + actions"
```

---

## Task 9: Frontend — Migrate Pages + Delete Old Hooks

**Files:**
- Modify: `web/src/pages/TasksPage.tsx`
- Modify: `web/src/pages/TaskDetailPage.tsx`
- Modify: `web/src/pages/DashboardPage.tsx`
- Delete: `web/src/hooks/useTasks.ts`
- Delete: `web/src/hooks/useTaskEvents.ts`

- [ ] **Step 1: Migrate TasksPage**

Replace the `useTasks` import and usage in `TasksPage.tsx`:

Change:
```typescript
import { useTasks } from '../hooks/useTasks';
```
To:
```typescript
import { useTaskSync } from '../hooks/useTaskSync';
```

Change:
```typescript
const { tasks, loading, error, createTask, startTask, stopTask } = useTasks();
```
To:
```typescript
const { tasks, loading, error, createTask, startTask, stopTask } = useTaskSync({ mode: 'kanban' });
```

The `createTask` call in `handleCreate` changes from `createTask(title, desc)` to `createTask({ title, description: desc })`.

- [ ] **Step 2: Migrate TaskDetailPage**

Replace the entire manual fetch logic with `useTaskSync`:

```typescript
import { useEffect } from 'react';
import { useParams, Link } from 'react-router';
import { reportError, reportUsage } from '../lib/telemetry';
import { usePerformance } from '../hooks/usePerformance';
import { useTaskSync } from '../hooks/useTaskSync';
import type { TaskStage } from '../lib/types';
import { ActivityTimeline } from '../components/ActivityTimeline';
```

Replace the state and fetch logic with:
```typescript
const taskId = id ? Number(id) : undefined;
const { task, activities, loading, error, startTask, stopTask } = useTaskSync({
  taskId,
  mode: 'detail',
});
```

Update `handleStart`/`handleStop` to use the hook's actions:
```typescript
const handleStart = async () => {
  if (!taskId) return;
  try {
    await startTask(taskId);
  } catch (err) {
    reportError('TaskDetailPage.start', err);
  }
};

const handleStop = async () => {
  if (!taskId) return;
  try {
    await stopTask(taskId);
  } catch (err) {
    reportError('TaskDetailPage.stop', err);
  }
};
```

- [ ] **Step 3: Migrate DashboardPage**

Change:
```typescript
import { useTasks } from '../hooks/useTasks';
```
To:
```typescript
import { useTaskSync } from '../hooks/useTaskSync';
```

Change:
```typescript
const { tasks, loading, error } = useTasks();
```
To:
```typescript
const { tasks, loading, error } = useTaskSync({ mode: 'kanban' });
```

- [ ] **Step 4: Check for any remaining useTasks/useTaskEvents imports**

Run: `grep -rn 'useTasks\|useTaskEvents' /home/rishav/soul-v2/web/src/ --include='*.ts' --include='*.tsx' | grep -v node_modules | grep -v useTaskSync`

If no remaining imports reference `useTasks` or `useTaskEvents`, delete both files.

- [ ] **Step 5: Delete old hooks**

```bash
rm web/src/hooks/useTasks.ts web/src/hooks/useTaskEvents.ts
```

- [ ] **Step 6: Verify TypeScript compiles**

Run: `cd /home/rishav/soul-v2/web && npx tsc --noEmit`
Expected: no errors

- [ ] **Step 7: Build frontend**

Run: `cd /home/rishav/soul-v2/web && npx vite build`
Expected: build succeeds

- [ ] **Step 8: Commit**

```bash
git add -u web/src/
git commit -m "feat(tasks): migrate pages to useTaskSync, delete useTasks + useTaskEvents"
```

---

## Task 10: Full Verification

**Files:** None (verification only)

- [ ] **Step 1: Run all Go tests**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/... -v -race -count=1`
Expected: All PASS, no race conditions

- [ ] **Step 2: Run go vet**

Run: `cd /home/rishav/soul-v2 && go vet ./...`
Expected: no errors

- [ ] **Step 3: Run frontend type check**

Run: `cd /home/rishav/soul-v2/web && npx tsc --noEmit`
Expected: no errors

- [ ] **Step 4: Build frontend**

Run: `cd /home/rishav/soul-v2/web && npx vite build`
Expected: build succeeds

- [ ] **Step 5: Run make verify-static**

Run: `cd /home/rishav/soul-v2 && make verify-static`
Expected: all checks pass

- [ ] **Step 6: Manual smoke test**

Start the server (`make serve`). Open the UI. Verify:
1. Dashboard shows task counts (live).
2. Tasks page shows Kanban board.
3. Create a task — it appears immediately without page refresh.
4. Navigate to task detail — activity timeline loads.
5. Start a task from detail — stage changes without manual refresh.

---

## Dependency Graph

```
Task 1 (types.go) ──┐
                     ├── Task 3 (migration) ── Task 4 (OnChange + nextSeq) ── Task 5 (delta methods) ── Task 6 (server)
Task 2 (cursor)  ────┘                                                                                       │
                                                                                                              │
Task 7 (useChat.ts forwarding) ── Task 8 (useTaskSync hook) ── Task 9 (migrate pages) ── Task 10 (verification)
```

**Parallelizable:** Tasks 1+2 can run in parallel. Tasks 7 can run in parallel with Tasks 3-6 (backend vs frontend). Task 8 depends on Task 7. Task 9 depends on Task 8. Task 10 depends on everything.

**Note:** Between Task 4 (OnChange wiring) and Task 6 (manual Broadcast removal), events will be double-broadcast. This is harmless — the frontend deduplicates by `seq` — but tests should not assert exact broadcast counts in this window.
