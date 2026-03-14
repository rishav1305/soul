# Tasks Server Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a standalone tasks server (`soul-tasks`, port 3003) with SQLite-backed task CRUD, SSE event broadcasting, and a reverse proxy from the chat server — delivering the foundation for autonomous task execution (Plan 3).

**Architecture:** Separate binary (`cmd/tasks/main.go`) with its own HTTP server, task store (`tasks.db`), and SSE broadcaster. The chat server reverse-proxies `/api/tasks/*` and `/api/products/*` to the tasks server and relays SSE events to WebSocket clients. Both servers share `pkg/auth/` and `pkg/events/`.

**Tech Stack:** Go 1.24, SQLite (modernc.org/sqlite), standard `net/http` with Go 1.22+ routing, SSE via chunked HTTP responses

**Spec:** `docs/superpowers/specs/2026-03-14-autonomous-execution-design.md`

---

## Scope

**In scope (Plan 2):**
- Task store: SQLite CRUD with stages (backlog, active, validation, done, blocked)
- Task activity log: structured event history per task
- REST API: list, create, get, update tasks + activity log
- SSE endpoint: real-time task status and activity events
- Health endpoint with task/worktree counts
- `soul-tasks` binary with graceful shutdown
- Chat-to-tasks reverse proxy (REST + SSE relay)
- Build, deploy, systemd service

**Out of scope (Plan 3):**
- Autonomous executor (agent loop, worktrees, step-verify-fix)
- Progressive verification gates (L1-L7)
- Product manager (gRPC lifecycle, health checks)
- `POST /api/tasks/:id/start` and `/stop` (stubbed as 501)

**Out of scope (Plan 4):**
- Frontend pages (Tasks Kanban, Dashboard, Product pages)
- Client-side routing

---

## File Structure

### New files to create

| File | Purpose |
|------|---------|
| `internal/tasks/store/store.go` | Task + activity SQLite CRUD |
| `internal/tasks/store/store_test.go` | Store unit tests |
| `internal/tasks/server/server.go` | HTTP server, route registration, handlers |
| `internal/tasks/server/middleware.go` | Recovery, request ID, body limit, CSP, rate limit |
| `internal/tasks/server/sse.go` | SSE broadcaster + `/api/stream` handler |
| `internal/tasks/server/server_test.go` | API endpoint tests |
| `internal/tasks/server/sse_test.go` | SSE broadcaster tests |
| `cmd/tasks/main.go` | Tasks server CLI entrypoint |
| `internal/chat/server/proxy.go` | Reverse proxy + SSE relay to tasks server |
| `deploy/soul-v2-tasks.service` | Systemd service for tasks server |

### Files to modify

| File | Change |
|------|--------|
| `internal/chat/server/server.go` | Register proxy routes |
| `Makefile` | Add `build-tasks` target, update `build` and `serve` |
| `CLAUDE.md` | Add tasks server to architecture section |

---

## Task 1: Create task store

SQLite-backed CRUD for tasks and activity log. Follows the same patterns as `internal/chat/session/store.go`.

**Files:**
- Create: `internal/tasks/store/store.go`
- Create: `internal/tasks/store/store_test.go`

- [ ] **Step 1: Write store tests**

Create `internal/tasks/store/store_test.go`:

```go
package store

import (
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tasks_test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpen_CreatesDatabase(t *testing.T) {
	s := newTestStore(t)
	if s == nil {
		t.Fatal("store is nil")
	}
}

func TestCreate_ReturnsTask(t *testing.T) {
	s := newTestStore(t)
	task, err := s.Create("Test task", "Description", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if task.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if task.Title != "Test task" {
		t.Errorf("Title = %q, want %q", task.Title, "Test task")
	}
	if task.Stage != "backlog" {
		t.Errorf("Stage = %q, want %q", task.Stage, "backlog")
	}
}

func TestGet_ReturnsCreatedTask(t *testing.T) {
	s := newTestStore(t)
	created, _ := s.Create("Get test", "desc", "")
	got, err := s.Get(created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "Get test" {
		t.Errorf("Title = %q, want %q", got.Title, "Get test")
	}
}

func TestGet_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.Get(999)
	if err == nil {
		t.Error("expected error for non-existent task")
	}
}

func TestList_FiltersByStage(t *testing.T) {
	s := newTestStore(t)
	s.Create("Task A", "", "")
	s.Create("Task B", "", "")
	task3, _ := s.Create("Task C", "", "")
	s.Update(task3.ID, map[string]interface{}{"stage": "active"})

	all, _ := s.List("", "")
	if len(all) != 3 {
		t.Errorf("List('') = %d tasks, want 3", len(all))
	}

	backlog, _ := s.List("backlog", "")
	if len(backlog) != 2 {
		t.Errorf("List('backlog') = %d tasks, want 2", len(backlog))
	}

	active, _ := s.List("active", "")
	if len(active) != 1 {
		t.Errorf("List('active') = %d tasks, want 1", len(active))
	}
}

func TestList_FiltersByProduct(t *testing.T) {
	s := newTestStore(t)
	s.Create("Core task", "", "")
	s.Create("Scout task", "", "scout")

	scout, _ := s.List("", "scout")
	if len(scout) != 1 {
		t.Errorf("List(product=scout) = %d, want 1", len(scout))
	}
}

func TestUpdate_ChangesFields(t *testing.T) {
	s := newTestStore(t)
	task, _ := s.Create("Original", "desc", "")
	updated, err := s.Update(task.ID, map[string]interface{}{
		"title": "Updated",
		"stage": "active",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Title != "Updated" {
		t.Errorf("Title = %q, want %q", updated.Title, "Updated")
	}
	if updated.Stage != "active" {
		t.Errorf("Stage = %q, want %q", updated.Stage, "active")
	}
}

func TestUpdate_RejectsInvalidStage(t *testing.T) {
	s := newTestStore(t)
	task, _ := s.Create("Task", "", "")
	_, err := s.Update(task.ID, map[string]interface{}{"stage": "invalid"})
	if err == nil {
		t.Error("expected error for invalid stage")
	}
}

func TestUpdate_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.Update(999, map[string]interface{}{"title": "x"})
	if err == nil {
		t.Error("expected error for non-existent task")
	}
}

func TestAddActivity_And_ListActivity(t *testing.T) {
	s := newTestStore(t)
	task, _ := s.Create("Task", "", "")

	err := s.AddActivity(task.ID, "task.created", map[string]interface{}{"by": "user"})
	if err != nil {
		t.Fatalf("AddActivity: %v", err)
	}

	activities, err := s.ListActivity(task.ID)
	if err != nil {
		t.Fatalf("ListActivity: %v", err)
	}
	if len(activities) != 1 {
		t.Fatalf("ListActivity = %d, want 1", len(activities))
	}
	if activities[0].EventType != "task.created" {
		t.Errorf("EventType = %q, want %q", activities[0].EventType, "task.created")
	}
}

func TestDelete_RemovesTaskAndActivity(t *testing.T) {
	s := newTestStore(t)
	task, _ := s.Create("Doomed", "", "")
	s.AddActivity(task.ID, "task.created", nil)

	err := s.Delete(task.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = s.Get(task.ID)
	if err == nil {
		t.Error("expected error after delete")
	}

	activities, _ := s.ListActivity(task.ID)
	if len(activities) != 0 {
		t.Errorf("activities after delete = %d, want 0", len(activities))
	}
}

func TestCountByStage(t *testing.T) {
	s := newTestStore(t)
	s.Create("A", "", "")
	s.Create("B", "", "")
	task3, _ := s.Create("C", "", "")
	s.Update(task3.ID, map[string]interface{}{"stage": "active"})

	counts, err := s.CountByStage()
	if err != nil {
		t.Fatalf("CountByStage: %v", err)
	}
	if counts["backlog"] != 2 {
		t.Errorf("backlog = %d, want 2", counts["backlog"])
	}
	if counts["active"] != 1 {
		t.Errorf("active = %d, want 1", counts["active"])
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/store/ -v`
Expected: FAIL (package doesn't exist)

- [ ] **Step 3: Implement store.go**

Create `internal/tasks/store/store.go`:

```go
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Valid task stages.
var validStages = map[string]bool{
	"backlog":    true,
	"active":     true,
	"validation": true,
	"done":       true,
	"blocked":    true,
}

// Task represents a task in the task store.
type Task struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Stage       string    `json:"stage"`
	Workflow    string    `json:"workflow,omitempty"`
	Product     string    `json:"product,omitempty"`
	Metadata    string    `json:"metadata,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Activity represents a task activity log entry.
type Activity struct {
	ID        int64     `json:"id"`
	TaskID    int64     `json:"taskId"`
	EventType string    `json:"eventType"`
	Data      string    `json:"data"`
	CreatedAt time.Time `json:"createdAt"`
}

// Store provides SQLite-backed task CRUD.
type Store struct {
	db     *sql.DB
	dbPath string
}

// Open creates a new Store with the given database path.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("tasks: open database: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("tasks: enable WAL: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("tasks: enable foreign keys: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("tasks: set busy timeout: %w", err)
	}

	s := &Store{db: db, dbPath: path}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	const schema = `
	CREATE TABLE IF NOT EXISTS tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		stage TEXT NOT NULL DEFAULT 'backlog',
		workflow TEXT NOT NULL DEFAULT '',
		product TEXT NOT NULL DEFAULT '',
		metadata TEXT NOT NULL DEFAULT '{}',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS task_activity (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
		event_type TEXT NOT NULL,
		data TEXT NOT NULL DEFAULT '{}',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_tasks_stage ON tasks(stage);
	CREATE INDEX IF NOT EXISTS idx_tasks_product ON tasks(product);
	CREATE INDEX IF NOT EXISTS idx_task_activity_task_id ON task_activity(task_id);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("tasks: migrate: %w", err)
	}
	return nil
}

// Create inserts a new task with the given title, description, and product.
func (s *Store) Create(title, description, product string) (*Task, error) {
	res, err := s.db.Exec(
		"INSERT INTO tasks (title, description, product) VALUES (?, ?, ?)",
		title, description, product,
	)
	if err != nil {
		return nil, fmt.Errorf("tasks: create: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.Get(id)
}

// Get retrieves a task by ID.
func (s *Store) Get(id int64) (*Task, error) {
	var t Task
	err := s.db.QueryRow(
		"SELECT id, title, description, stage, workflow, product, metadata, created_at, updated_at FROM tasks WHERE id = ?",
		id,
	).Scan(&t.ID, &t.Title, &t.Description, &t.Stage, &t.Workflow, &t.Product, &t.Metadata, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tasks: not found: %d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("tasks: get: %w", err)
	}
	return &t, nil
}

// List returns tasks, optionally filtered by stage and/or product.
func (s *Store) List(stage, product string) ([]Task, error) {
	query := "SELECT id, title, description, stage, workflow, product, metadata, created_at, updated_at FROM tasks"
	var conditions []string
	var args []interface{}

	if stage != "" {
		conditions = append(conditions, "stage = ?")
		args = append(args, stage)
	}
	if product != "" {
		conditions = append(conditions, "product = ?")
		args = append(args, product)
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY updated_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("tasks: list: %w", err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Stage, &t.Workflow, &t.Product, &t.Metadata, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("tasks: scan: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// Update modifies task fields. Allowed keys: title, description, stage, workflow, product, metadata.
func (s *Store) Update(id int64, fields map[string]interface{}) (*Task, error) {
	if stage, ok := fields["stage"]; ok {
		if s, ok := stage.(string); ok && !validStages[s] {
			return nil, fmt.Errorf("tasks: invalid stage: %q", s)
		}
	}

	var setClauses []string
	var args []interface{}
	allowed := map[string]bool{"title": true, "description": true, "stage": true, "workflow": true, "product": true, "metadata": true}

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

	setClauses = append(setClauses, "updated_at = CURRENT_TIMESTAMP")
	args = append(args, id)

	result, err := s.db.Exec(
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
	return s.Get(id)
}

// Delete removes a task and its activity (cascading foreign key).
func (s *Store) Delete(id int64) error {
	result, err := s.db.Exec("DELETE FROM tasks WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("tasks: delete: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("tasks: not found: %d", id)
	}
	return nil
}

// AddActivity appends an activity entry for a task.
func (s *Store) AddActivity(taskID int64, eventType string, data map[string]interface{}) error {
	dataJSON := "{}"
	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("tasks: marshal activity data: %w", err)
		}
		dataJSON = string(b)
	}
	_, err := s.db.Exec(
		"INSERT INTO task_activity (task_id, event_type, data) VALUES (?, ?, ?)",
		taskID, eventType, dataJSON,
	)
	if err != nil {
		return fmt.Errorf("tasks: add activity: %w", err)
	}
	return nil
}

// ListActivity returns activity entries for a task, newest first.
func (s *Store) ListActivity(taskID int64) ([]Activity, error) {
	rows, err := s.db.Query(
		"SELECT id, task_id, event_type, data, created_at FROM task_activity WHERE task_id = ? ORDER BY created_at DESC",
		taskID,
	)
	if err != nil {
		return nil, fmt.Errorf("tasks: list activity: %w", err)
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

// CountByStage returns task counts grouped by stage.
func (s *Store) CountByStage() (map[string]int, error) {
	rows, err := s.db.Query("SELECT stage, COUNT(*) FROM tasks GROUP BY stage")
	if err != nil {
		return nil, fmt.Errorf("tasks: count by stage: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var stage string
		var count int
		if err := rows.Scan(&stage, &count); err != nil {
			return nil, fmt.Errorf("tasks: scan count: %w", err)
		}
		counts[stage] = count
	}
	return counts, rows.Err()
}
```

- [ ] **Step 4: Run tests**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/store/ -v`
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tasks/store/
git commit -m "feat: add tasks store with SQLite CRUD and activity log"
```

---

## Task 2: Create SSE broadcaster

An in-memory pub/sub broadcaster for real-time task events over SSE.

**Files:**
- Create: `internal/tasks/server/sse.go`
- Create: `internal/tasks/server/sse_test.go`

- [ ] **Step 1: Write broadcaster tests**

Create `internal/tasks/server/sse_test.go`:

```go
package server

import (
	"testing"
	"time"
)

func TestBroadcaster_SubscribeReceivesEvents(t *testing.T) {
	b := NewBroadcaster()

	ch, cancel := b.Subscribe()
	defer cancel()

	go b.Broadcast(Event{Type: "task.created", Data: `{"id":1}`})

	select {
	case ev := <-ch:
		if ev.Type != "task.created" {
			t.Errorf("Type = %q, want %q", ev.Type, "task.created")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestBroadcaster_CancelRemovesSubscriber(t *testing.T) {
	b := NewBroadcaster()

	_, cancel := b.Subscribe()
	cancel()

	b.mu.RLock()
	count := len(b.subscribers)
	b.mu.RUnlock()

	if count != 0 {
		t.Errorf("subscribers = %d, want 0 after cancel", count)
	}
}

func TestBroadcaster_MultipleSubscribers(t *testing.T) {
	b := NewBroadcaster()

	ch1, cancel1 := b.Subscribe()
	defer cancel1()
	ch2, cancel2 := b.Subscribe()
	defer cancel2()

	go b.Broadcast(Event{Type: "test", Data: "{}"})

	for _, ch := range []<-chan Event{ch1, ch2} {
		select {
		case ev := <-ch:
			if ev.Type != "test" {
				t.Errorf("Type = %q, want %q", ev.Type, "test")
			}
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for event")
		}
	}
}

func TestBroadcaster_SlowSubscriberDoesNotBlock(t *testing.T) {
	b := NewBroadcaster()

	// Subscribe but never read — channel should fill and broadcast should not block.
	_, cancel := b.Subscribe()
	defer cancel()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 200; i++ {
			b.Broadcast(Event{Type: "flood", Data: "{}"})
		}
		close(done)
	}()

	select {
	case <-done:
		// Broadcast completed without blocking — good.
	case <-time.After(2 * time.Second):
		t.Fatal("broadcast blocked on slow subscriber")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/server/ -run TestBroadcaster -v`
Expected: FAIL (types don't exist)

- [ ] **Step 3: Implement broadcaster**

Create `internal/tasks/server/sse.go`:

```go
package server

import (
	"fmt"
	"net/http"
	"sync"
)

// Event is a server-sent event.
type Event struct {
	Type string
	Data string
}

// Broadcaster is an in-memory pub/sub for SSE events.
type Broadcaster struct {
	mu          sync.RWMutex
	subscribers map[chan Event]struct{}
}

// NewBroadcaster creates a new Broadcaster.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		subscribers: make(map[chan Event]struct{}),
	}
}

// Subscribe returns a channel that receives broadcast events and a cancel function.
func (b *Broadcaster) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 64)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()

	cancel := func() {
		b.mu.Lock()
		delete(b.subscribers, ch)
		b.mu.Unlock()
	}
	return ch, cancel
}

// Broadcast sends an event to all subscribers. Non-blocking — drops if a subscriber's buffer is full.
func (b *Broadcaster) Broadcast(ev Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subscribers {
		select {
		case ch <- ev:
		default:
			// Subscriber too slow — drop event.
		}
	}
}

// handleStream handles the GET /api/stream SSE endpoint.
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch, cancel := s.broadcaster.Subscribe()
	defer cancel()

	// Send initial connected event.
	fmt.Fprintf(w, "event: connected\ndata: {}\n\n")
	flusher.Flush()

	for {
		select {
		case ev := <-ch:
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, ev.Data)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
```

- [ ] **Step 4: Run tests**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/server/ -run TestBroadcaster -v`
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tasks/server/sse.go internal/tasks/server/sse_test.go
git commit -m "feat: add SSE broadcaster for task events"
```

---

## Task 3: Create tasks HTTP server

REST API for task CRUD, activity log, health endpoint, and SSE stream.

**Files:**
- Create: `internal/tasks/server/server.go`
- Create: `internal/tasks/server/middleware.go`
- Create: `internal/tasks/server/server_test.go`

- [ ] **Step 1: Write server tests**

Create `internal/tasks/server/server_test.go`:

```go
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rishav1305/soul-v2/internal/tasks/store"
	"github.com/rishav1305/soul-v2/pkg/events"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return New(WithStore(s), WithLogger(events.NopLogger{}))
}

func TestHealthEndpoint(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("status = %v, want ok", body["status"])
	}
}

func TestCreateTask(t *testing.T) {
	srv := newTestServer(t)
	body := `{"title":"Test task","description":"A description"}`
	req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
	var task store.Task
	json.NewDecoder(rec.Body).Decode(&task)
	if task.Title != "Test task" {
		t.Errorf("Title = %q, want %q", task.Title, "Test task")
	}
}

func TestListTasks(t *testing.T) {
	srv := newTestServer(t)

	// Create two tasks.
	for _, title := range []string{"A", "B"} {
		body := `{"title":"` + title + `"}`
		req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
	}

	req := httptest.NewRequest("GET", "/api/tasks", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var tasks []store.Task
	json.NewDecoder(rec.Body).Decode(&tasks)
	if len(tasks) != 2 {
		t.Errorf("len = %d, want 2", len(tasks))
	}
}

func TestGetTask(t *testing.T) {
	srv := newTestServer(t)

	body := `{"title":"Get me"}`
	req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var created store.Task
	json.NewDecoder(rec.Body).Decode(&created)

	req = httptest.NewRequest("GET", fmt.Sprintf("/api/tasks/%d", created.ID), nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestGetTask_NotFound(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/tasks/999", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestUpdateTask(t *testing.T) {
	srv := newTestServer(t)

	body := `{"title":"Original"}`
	req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var created store.Task
	json.NewDecoder(rec.Body).Decode(&created)

	patchBody := `{"title":"Updated","stage":"active"}`
	req = httptest.NewRequest("PATCH", fmt.Sprintf("/api/tasks/%d", created.ID), strings.NewReader(patchBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var updated store.Task
	json.NewDecoder(rec.Body).Decode(&updated)
	if updated.Title != "Updated" {
		t.Errorf("Title = %q, want %q", updated.Title, "Updated")
	}
}

func TestDeleteTask(t *testing.T) {
	srv := newTestServer(t)

	body := `{"title":"Delete me"}`
	req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var created store.Task
	json.NewDecoder(rec.Body).Decode(&created)

	req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/tasks/%d", created.ID), nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
}

func TestTaskActivity(t *testing.T) {
	srv := newTestServer(t)

	body := `{"title":"With activity"}`
	req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var created store.Task
	json.NewDecoder(rec.Body).Decode(&created)

	req = httptest.NewRequest("GET", fmt.Sprintf("/api/tasks/%d/activity", created.ID), nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	// Should have a task.created activity from the POST handler.
	var activities []store.Activity
	json.NewDecoder(rec.Body).Decode(&activities)
	if len(activities) < 1 {
		t.Error("expected at least 1 activity entry")
	}
}

func TestStartTask_NotImplemented(t *testing.T) {
	srv := newTestServer(t)

	body := `{"title":"Start me"}`
	req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var created store.Task
	json.NewDecoder(rec.Body).Decode(&created)

	req = httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%d/start", created.ID), nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want 501", rec.Code)
	}
}
```

Add `"fmt"` to the imports.

- [ ] **Step 2: Implement middleware.go**

Create `internal/tasks/server/middleware.go`:

```go
package server

import (
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"sync/atomic"
)

var requestCounter atomic.Uint64

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("[panic] %v\n%s", err, debug.Stack())
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := fmt.Sprintf("tasks-%04d", requestCounter.Add(1))
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r)
	})
}

func bodyLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil && (r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH") {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func cspMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 3: Implement server.go**

Create `internal/tasks/server/server.go`:

```go
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rishav1305/soul-v2/internal/tasks/store"
	"github.com/rishav1305/soul-v2/pkg/events"
)

// Server is the tasks HTTP server.
type Server struct {
	mux         *http.ServeMux
	httpServer  *http.Server
	store       *store.Store
	broadcaster *Broadcaster
	logger      events.Logger
	host        string
	port        int
	startTime   time.Time
}

// Option configures the Server.
type Option func(*Server)

func WithStore(s *store.Store) Option       { return func(srv *Server) { srv.store = s } }
func WithLogger(l events.Logger) Option     { return func(srv *Server) { srv.logger = l } }
func WithHost(h string) Option              { return func(srv *Server) { srv.host = h } }
func WithPort(p int) Option                 { return func(srv *Server) { srv.port = p } }

// New creates a new tasks Server.
func New(opts ...Option) *Server {
	s := &Server{
		mux:         http.NewServeMux(),
		broadcaster: NewBroadcaster(),
		logger:      events.NopLogger{},
		host:        "127.0.0.1",
		port:        3003,
		startTime:   time.Now(),
	}
	for _, opt := range opts {
		opt(s)
	}

	// Register routes.
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/tasks", s.handleListTasks)
	s.mux.HandleFunc("POST /api/tasks", s.handleCreateTask)
	s.mux.HandleFunc("GET /api/tasks/{id}", s.handleGetTask)
	s.mux.HandleFunc("PATCH /api/tasks/{id}", s.handleUpdateTask)
	s.mux.HandleFunc("DELETE /api/tasks/{id}", s.handleDeleteTask)
	s.mux.HandleFunc("POST /api/tasks/{id}/start", s.handleStartTask)
	s.mux.HandleFunc("POST /api/tasks/{id}/stop", s.handleStopTask)
	s.mux.HandleFunc("GET /api/tasks/{id}/activity", s.handleTaskActivity)
	s.mux.HandleFunc("GET /api/stream", s.handleStream)

	// Build middleware chain.
	handler := http.Handler(s.mux)
	handler = bodyLimitMiddleware(64 << 10)(handler)
	handler = cspMiddleware(handler)
	handler = requestIDMiddleware(handler)
	handler = recoveryMiddleware(handler)

	s.httpServer = &http.Server{
		Addr:              net.JoinHostPort(s.host, strconv.Itoa(s.port)),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return s
}

// ServeHTTP implements http.Handler for testing.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// Start begins listening.
func (s *Server) Start() error {
	log.Printf("soul-tasks listening on http://%s", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// --- Handlers ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	counts, _ := s.store.CountByStage()
	active := counts["active"]

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":       "ok",
		"uptime":       time.Since(s.startTime).Round(time.Second).String(),
		"active_tasks": active,
	})
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	stage := r.URL.Query().Get("stage")
	product := r.URL.Query().Get("product")

	tasks, err := s.store.List(stage, product)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if tasks == nil {
		tasks = []store.Task{}
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Product     string `json:"product"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if body.Title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title is required"})
		return
	}

	task, err := s.store.Create(body.Title, body.Description, body.Product)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Log activity.
	s.store.AddActivity(task.ID, "task.created", map[string]interface{}{
		"title": task.Title,
	})

	// Broadcast event.
	data, _ := json.Marshal(task)
	s.broadcaster.Broadcast(Event{Type: "task.created", Data: string(data)})

	_ = s.logger.Log("task.created", map[string]interface{}{
		"task_id": task.ID,
		"title":   task.Title,
	})

	writeJSON(w, http.StatusCreated, task)
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	task, err := s.store.Get(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	var fields map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&fields); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	task, err := s.store.Update(id, fields)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
			return
		}
		if strings.Contains(err.Error(), "invalid stage") {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Broadcast update.
	data, _ := json.Marshal(task)
	s.broadcaster.Broadcast(Event{Type: "task.updated", Data: string(data)})

	writeJSON(w, http.StatusOK, task)
}

func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	if err := s.store.Delete(id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleStartTask(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusNotImplemented, map[string]string{
		"error": "autonomous execution not yet implemented — see Plan 3",
	})
}

func (s *Server) handleStopTask(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusNotImplemented, map[string]string{
		"error": "autonomous execution not yet implemented — see Plan 3",
	})
}

func (s *Server) handleTaskActivity(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
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

// --- Helpers ---

func parseID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
```

- [ ] **Step 4: Run tests**

Run: `cd /home/rishav/soul-v2 && go test ./internal/tasks/server/ -v`
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tasks/server/
git commit -m "feat: add tasks HTTP server with REST API and SSE"
```

---

## Task 4: Create cmd/tasks binary and update Makefile

The tasks server binary entrypoint and build infrastructure.

**Files:**
- Create: `cmd/tasks/main.go`
- Modify: `Makefile`

- [ ] **Step 1: Create cmd/tasks/main.go**

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/rishav1305/soul-v2/internal/tasks/server"
	"github.com/rishav1305/soul-v2/internal/tasks/store"
	"github.com/rishav1305/soul-v2/pkg/events"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: soul-tasks <command>")
		fmt.Println("commands: serve")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		runServe()
	default:
		fmt.Printf("unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func runServe() {
	// Data directory.
	dataDir := os.Getenv("SOUL_V2_DATA_DIR")
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".soul-v2")
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}

	// Open task store.
	dbPath := filepath.Join(dataDir, "tasks.db")
	taskStore, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("open task store: %v", err)
	}
	defer taskStore.Close()

	// Recover interrupted tasks on startup.
	recoverInterruptedTasks(taskStore)

	// Server options.
	host := os.Getenv("SOUL_TASKS_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	port := 3003
	if p := os.Getenv("SOUL_TASKS_PORT"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			port = parsed
		}
	}

	opts := []server.Option{
		server.WithStore(taskStore),
		server.WithLogger(events.NopLogger{}),
		server.WithHost(host),
		server.WithPort(port),
	}

	srv := server.New(opts...)

	// Graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("received %s, shutting down...", sig)
		srv.Shutdown(context.Background())
	}()

	if err := srv.Start(); err != nil && err.Error() != "http: Server closed" {
		log.Fatalf("server error: %v", err)
	}
}

// recoverInterruptedTasks marks any active tasks as blocked on startup.
func recoverInterruptedTasks(s *store.Store) {
	tasks, err := s.List("active", "")
	if err != nil {
		log.Printf("warn: could not scan for interrupted tasks: %v", err)
		return
	}
	for _, t := range tasks {
		log.Printf("recovering interrupted task %d: %s", t.ID, t.Title)
		s.Update(t.ID, map[string]interface{}{
			"stage": "blocked",
		})
		s.AddActivity(t.ID, "task.blocked", map[string]interface{}{
			"reason": "server restart — execution interrupted",
		})
	}
}
```

- [ ] **Step 2: Update Makefile**

Add the `build-tasks` target and update `build` and `serve`:

In the Makefile, replace the `build-tasks` placeholder:
```makefile
# Before:
build-tasks:
	@echo "Tasks server not yet implemented"

# After:
build-tasks:
	go build -o soul-tasks ./cmd/tasks
```

Update `build` to include both:
```makefile
build: build-go build-tasks web
```

Update `serve` to run both servers:
```makefile
serve: build
	./soul-chat serve & ./soul-tasks serve & wait
```

Add `soul-tasks` to `clean`:
```makefile
clean:
	rm -f soul-chat soul-tasks
	rm -rf web/dist
```

- [ ] **Step 3: Verify build**

Run: `cd /home/rishav/soul-v2 && go build -o soul-tasks ./cmd/tasks`
Expected: Produces `soul-tasks` binary.

Run: `go build ./...`
Expected: All packages compile.

- [ ] **Step 4: Update .gitignore**

Add `/soul-tasks` to `.gitignore`:
```
/soul-chat
/soul-tasks
```

- [ ] **Step 5: Commit**

```bash
git add cmd/tasks/ Makefile .gitignore
git commit -m "feat: add soul-tasks binary and build infrastructure"
```

---

## Task 5: Chat-to-tasks reverse proxy

Add a reverse proxy to the chat server that forwards `/api/tasks/*` and `/api/products/*` requests to the tasks server, and relays SSE events to WebSocket clients.

**Files:**
- Create: `internal/chat/server/proxy.go`
- Modify: `internal/chat/server/server.go` — register proxy routes

- [ ] **Step 1: Implement proxy.go**

Create `internal/chat/server/proxy.go`:

```go
package server

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// tasksProxy manages the reverse proxy and SSE relay to the tasks server.
type tasksProxy struct {
	targetURL    *url.URL
	reverseProxy *httputil.ReverseProxy
	hub          hubBroadcaster
	mu           sync.Mutex
	connected    bool
}

// hubBroadcaster is the interface the proxy needs to send events to WS clients.
type hubBroadcaster interface {
	BroadcastJSON(msgType string, data interface{})
}

func newTasksProxy(hub hubBroadcaster) *tasksProxy {
	tasksURL := os.Getenv("SOUL_TASKS_URL")
	if tasksURL == "" {
		tasksURL = "http://127.0.0.1:3003"
	}

	target, err := url.Parse(tasksURL)
	if err != nil {
		log.Printf("warn: invalid SOUL_TASKS_URL %q: %v", tasksURL, err)
		return nil
	}

	rp := httputil.NewSingleHostReverseProxy(target)

	// Custom transport with timeout.
	rp.Transport = &http.Transport{
		ResponseHeaderTimeout: 5 * time.Second,
	}

	// Custom error handler — return 503 if tasks server is down.
	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("tasks proxy error: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, `{"error":"tasks server unavailable"}`)
	}

	return &tasksProxy{
		targetURL:    target,
		reverseProxy: rp,
		hub:          hub,
	}
}

// ServeHTTP forwards requests to the tasks server.
func (tp *tasksProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tp.reverseProxy.ServeHTTP(w, r)
}

// StartSSERelay connects to the tasks server SSE stream and relays events to WS clients.
func (tp *tasksProxy) StartSSERelay(ctx context.Context) {
	if tp == nil || tp.hub == nil {
		return
	}

	backoff := []time.Duration{
		1 * time.Second, 2 * time.Second, 4 * time.Second,
		8 * time.Second, 15 * time.Second, 30 * time.Second,
	}
	attempt := 0

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		err := tp.connectSSE(ctx)
		if err != nil && ctx.Err() == nil {
			delay := backoff[attempt]
			if attempt < len(backoff)-1 {
				attempt++
			}
			log.Printf("tasks SSE relay disconnected: %v (retry in %s)", err, delay)

			tp.mu.Lock()
			tp.connected = false
			tp.mu.Unlock()

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return
			}
		} else {
			attempt = 0
		}
	}
}

func (tp *tasksProxy) connectSSE(ctx context.Context) error {
	streamURL := tp.targetURL.String() + "/api/stream"
	req, err := http.NewRequestWithContext(ctx, "GET", streamURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("SSE stream returned %d", resp.StatusCode)
	}

	tp.mu.Lock()
	tp.connected = true
	tp.mu.Unlock()

	log.Printf("tasks SSE relay connected to %s", streamURL)

	scanner := bufio.NewScanner(resp.Body)
	var eventType, eventData string

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			eventData = strings.TrimPrefix(line, "data: ")
		} else if line == "" && eventType != "" {
			// Empty line = end of event.
			if tp.hub != nil && eventType != "connected" {
				tp.hub.BroadcastJSON(eventType, eventData)
			}
			eventType = ""
			eventData = ""
		}
	}

	return scanner.Err()
}

// IsConnected returns whether the SSE relay is connected.
func (tp *tasksProxy) IsConnected() bool {
	if tp == nil {
		return false
	}
	tp.mu.Lock()
	defer tp.mu.Unlock()
	return tp.connected
}
```

- [ ] **Step 2: Register proxy routes in server.go**

In `internal/chat/server/server.go`, add to the route registration block (after existing routes, before the SPA handler):

```go
// Tasks server proxy.
if s.tasksProxy != nil {
    s.mux.Handle("/api/tasks/", s.tasksProxy)
    s.mux.Handle("/api/tasks", s.tasksProxy)
    s.mux.Handle("/api/products/", s.tasksProxy)
    s.mux.Handle("/api/products", s.tasksProxy)
}
```

Add `tasksProxy *tasksProxy` field to the Server struct.

Add option:
```go
func WithTasksProxy(hub hubBroadcaster) Option {
    return func(s *Server) {
        s.tasksProxy = newTasksProxy(hub)
    }
}
```

In `cmd/chat/main.go`, add the option when creating the server and start the SSE relay goroutine:
```go
serverOpts = append(serverOpts, server.WithTasksProxy(hub))
```

After `srv` is created:
```go
// Start tasks server SSE relay.
relayCtx, relayCancel := context.WithCancel(context.Background())
defer relayCancel()
go srv.StartSSERelay(relayCtx)
```

Add a public `StartSSERelay` method to Server that delegates to tasksProxy:
```go
func (s *Server) StartSSERelay(ctx context.Context) {
    if s.tasksProxy != nil {
        s.tasksProxy.StartSSERelay(ctx)
    }
}
```

- [ ] **Step 3: Add BroadcastJSON to Hub**

The WebSocket hub needs a `BroadcastJSON` method. In `internal/chat/ws/hub.go`, add:

```go
// BroadcastJSON broadcasts a JSON message to all connected clients.
func (h *Hub) BroadcastJSON(msgType string, data interface{}) {
    msg := map[string]interface{}{
        "type": msgType,
        "data": data,
    }
    payload, err := json.Marshal(msg)
    if err != nil {
        return
    }
    h.broadcast <- payload
}
```

- [ ] **Step 4: Verify build**

Run: `cd /home/rishav/soul-v2 && go build ./...`
Expected: All packages compile.

- [ ] **Step 5: Commit**

```bash
git add internal/chat/server/proxy.go internal/chat/server/server.go internal/chat/ws/hub.go cmd/chat/main.go
git commit -m "feat: add chat-to-tasks reverse proxy with SSE relay"
```

---

## Task 6: Deploy, docs, and full verification

Create systemd service, update documentation, and verify everything works.

**Files:**
- Create: `deploy/soul-v2-tasks.service`
- Modify: `CLAUDE.md`

- [ ] **Step 1: Create systemd service**

Create `deploy/soul-v2-tasks.service`:

```ini
[Unit]
Description=Soul v2 — Tasks Server
After=network.target

[Service]
Type=simple
User=rishav
Group=rishav
WorkingDirectory=/home/rishav/soul-v2
ExecStart=/home/rishav/soul-v2/soul-tasks serve
Restart=on-failure
RestartSec=5
Environment=SOUL_TASKS_HOST=127.0.0.1
Environment=SOUL_TASKS_PORT=3003

# Hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=read-only
ReadWritePaths=/home/rishav/.soul-v2
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

- [ ] **Step 2: Update CLAUDE.md**

Add tasks server to the architecture section:

```
cmd/chat/main.go              Chat server CLI entrypoint (:3002)
cmd/tasks/main.go             Tasks server CLI entrypoint (:3003)
pkg/
  auth/                       Claude OAuth — shared by both servers
  events/                     Logger interface + Event type
internal/chat/
  server/                     HTTP server + SPA serving + tasks proxy
  session/                    SQLite session CRUD (chat.db)
  stream/                     Claude API streaming — SSE parse
  ws/                         WebSocket hub — session-scoped routing
  metrics/                    Event logging, aggregation, CLI reporting
internal/tasks/
  server/                     HTTP server, REST API, SSE broadcaster
  store/                      SQLite task CRUD (tasks.db)
```

Add environment variables:

```
| `SOUL_TASKS_HOST` | `127.0.0.1` | Tasks server bind address |
| `SOUL_TASKS_PORT` | `3003` | Tasks server port |
| `SOUL_TASKS_URL` | `http://127.0.0.1:3003` | Tasks server URL (for chat proxy) |
```

- [ ] **Step 3: Run full verification**

```bash
cd /home/rishav/soul-v2

# Build both binaries.
go build -o soul-chat ./cmd/chat
go build -o soul-tasks ./cmd/tasks

# Run all tests.
go test ./pkg/... ./internal/tasks/... -v -count=1

# Static analysis.
go vet ./...

# Frontend types (unchanged — should still pass).
cd web && npx tsc --noEmit
```

Expected: All builds pass, all tests pass, no vet issues, TypeScript clean.

- [ ] **Step 4: Commit**

```bash
git add deploy/soul-v2-tasks.service CLAUDE.md
git commit -m "docs: add tasks server deploy + update CLAUDE.md architecture"
```

---

## Post-Plan Verification

After all 6 tasks are complete:

1. `go build ./...` — all packages compile
2. `go test ./internal/tasks/... -v` — all task server tests pass
3. `go test ./pkg/... -v` — all shared package tests pass
4. `make build` — produces both `soul-chat` and `soul-tasks` binaries
5. Start tasks server: `./soul-tasks serve` — listens on :3003
6. `curl http://127.0.0.1:3003/api/health` — returns `{"status":"ok",...}`
7. `curl -X POST http://127.0.0.1:3003/api/tasks -d '{"title":"Test"}' -H 'Content-Type: application/json'` — creates task
8. `curl http://127.0.0.1:3003/api/tasks` — lists tasks
9. Start chat server: `./soul-chat serve` — listens on :3002, proxies to :3003
10. `curl http://127.0.0.1:3002/api/tasks` — proxied through chat server

## Next Plans

After this plan is verified:
- **Plan 3: Autonomous Execution** — executor, progressive gates, product manager, worktree management
- **Plan 4: Frontend** — routing, Dashboard, Tasks Kanban, Product pages
