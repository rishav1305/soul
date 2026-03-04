# Soul Planner Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a Kanban-based task manager as Soul's core orchestration layer with 6 stages, 6 substeps, SQLite persistence, REST API, WebSocket real-time updates, and a two-panel UI.

**Architecture:** Core Go package `internal/planner/` with SQLite storage (`modernc.org/sqlite`, pure Go, no CGo). REST endpoints on the existing `http.ServeMux`. WebSocket events broadcast through existing `WSClient`. React frontend with Kanban board replacing the compliance side panel.

**Tech Stack:** Go 1.24, `modernc.org/sqlite`, React 19, TailwindCSS 4, existing WebSocket infrastructure

---

## Task 1: Add SQLite dependency

**Files:**
- Modify: `go.mod`

**Step 1: Add the pure-Go SQLite driver**

```bash
cd /home/rishav/soul && go get modernc.org/sqlite
```

This adds a CGo-free SQLite implementation suitable for cross-compilation (arm64 Pi).

**Step 2: Verify it resolves**

```bash
cd /home/rishav/soul && go mod tidy
```

Expected: clean exit, `go.sum` updated.

**Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "deps: add modernc.org/sqlite for planner storage"
```

---

## Task 2: Planner types (`internal/planner/types.go`)

**Files:**
- Create: `internal/planner/types.go`
- Test: `internal/planner/types_test.go`

**Step 1: Write the failing test**

Create `internal/planner/types_test.go`:

```go
package planner

import (
	"testing"
	"time"
)

func TestStageConstants(t *testing.T) {
	stages := []Stage{StageBacklog, StageBrainstorm, StageActive, StageBlocked, StageValidation, StageDone}
	if len(stages) != 6 {
		t.Fatalf("expected 6 stages, got %d", len(stages))
	}
	for _, s := range stages {
		if !s.Valid() {
			t.Errorf("stage %q should be valid", s)
		}
	}
	if Stage("invalid").Valid() {
		t.Error("invalid stage should not be valid")
	}
}

func TestSubstepConstants(t *testing.T) {
	substeps := []Substep{SubstepTDD, SubstepImplementing, SubstepReviewing, SubstepQATest, SubstepE2ETest, SubstepSecurityReview}
	if len(substeps) != 6 {
		t.Fatalf("expected 6 substeps, got %d", len(substeps))
	}
	for _, s := range substeps {
		if !s.Valid() {
			t.Errorf("substep %q should be valid", s)
		}
	}
}

func TestStageTransitionValid(t *testing.T) {
	tests := []struct {
		from, to Stage
		ok       bool
	}{
		{StageBacklog, StageBrainstorm, true},
		{StageBacklog, StageActive, true},
		{StageBrainstorm, StageActive, true},
		{StageActive, StageBlocked, true},
		{StageActive, StageValidation, true},
		{StageBlocked, StageActive, true},
		{StageValidation, StageDone, true},
		{StageValidation, StageActive, true},
		// Invalid transitions
		{StageBacklog, StageDone, false},
		{StageDone, StageBacklog, false},
		{StageBrainstorm, StageValidation, false},
	}
	for _, tt := range tests {
		got := ValidTransition(tt.from, tt.to)
		if got != tt.ok {
			t.Errorf("ValidTransition(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.ok)
		}
	}
}

func TestNewTask(t *testing.T) {
	task := NewTask("Test task", "Description here")
	if task.Title != "Test task" {
		t.Errorf("title = %q, want %q", task.Title, "Test task")
	}
	if task.Stage != StageBacklog {
		t.Errorf("stage = %q, want %q", task.Stage, StageBacklog)
	}
	if task.Source != "manual" {
		t.Errorf("source = %q, want %q", task.Source, "manual")
	}
	if task.CreatedAt.IsZero() {
		t.Error("created_at should not be zero")
	}
}

func TestSubstepNext(t *testing.T) {
	next, ok := SubstepTDD.Next()
	if !ok || next != SubstepImplementing {
		t.Errorf("Next(tdd) = (%q, %v), want (%q, true)", next, ok, SubstepImplementing)
	}
	_, ok = SubstepSecurityReview.Next()
	if ok {
		t.Error("Next(security_review) should return false")
	}
}

func TestSubstepIndex(t *testing.T) {
	if SubstepTDD.Index() != 1 {
		t.Errorf("tdd index = %d, want 1", SubstepTDD.Index())
	}
	if SubstepSecurityReview.Index() != 6 {
		t.Errorf("security_review index = %d, want 6", SubstepSecurityReview.Index())
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd /home/rishav/soul && go test ./internal/planner/ -v -run TestStage
```

Expected: FAIL — package doesn't exist yet.

**Step 3: Write the implementation**

Create `internal/planner/types.go`:

```go
package planner

import "time"

// Stage represents a task's position on the Kanban board.
type Stage string

const (
	StageBacklog    Stage = "backlog"
	StageBrainstorm Stage = "brainstorm"
	StageActive     Stage = "active"
	StageBlocked    Stage = "blocked"
	StageValidation Stage = "validation"
	StageDone       Stage = "done"
)

var validStages = map[Stage]bool{
	StageBacklog: true, StageBrainstorm: true, StageActive: true,
	StageBlocked: true, StageValidation: true, StageDone: true,
}

func (s Stage) Valid() bool { return validStages[s] }

// validTransitions defines allowed stage transitions.
var validTransitions = map[Stage][]Stage{
	StageBacklog:    {StageBrainstorm, StageActive},
	StageBrainstorm: {StageActive},
	StageActive:     {StageBlocked, StageValidation},
	StageBlocked:    {StageActive},
	StageValidation: {StageDone, StageActive},
}

func ValidTransition(from, to Stage) bool {
	for _, allowed := range validTransitions[from] {
		if allowed == to {
			return true
		}
	}
	return false
}

// Substep represents the TDD pipeline step within the ACTIVE stage.
type Substep string

const (
	SubstepTDD            Substep = "tdd"
	SubstepImplementing   Substep = "implementing"
	SubstepReviewing      Substep = "reviewing"
	SubstepQATest         Substep = "qa_test"
	SubstepE2ETest        Substep = "e2e_test"
	SubstepSecurityReview Substep = "security_review"
)

var substepOrder = []Substep{
	SubstepTDD, SubstepImplementing, SubstepReviewing,
	SubstepQATest, SubstepE2ETest, SubstepSecurityReview,
}

var validSubsteps = map[Substep]bool{
	SubstepTDD: true, SubstepImplementing: true, SubstepReviewing: true,
	SubstepQATest: true, SubstepE2ETest: true, SubstepSecurityReview: true,
}

func (s Substep) Valid() bool { return validSubsteps[s] }

// Index returns the 1-based position in the pipeline (1-6).
func (s Substep) Index() int {
	for i, ss := range substepOrder {
		if ss == s {
			return i + 1
		}
	}
	return 0
}

// Next returns the next substep in the pipeline, or false if at the end.
func (s Substep) Next() (Substep, bool) {
	idx := s.Index()
	if idx == 0 || idx >= len(substepOrder) {
		return "", false
	}
	return substepOrder[idx], true
}

// Task is the core data model for a planner task.
type Task struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Acceptance  string    `json:"acceptance,omitempty"`
	Stage       Stage     `json:"stage"`
	Substep     Substep   `json:"substep,omitempty"`
	Priority    int       `json:"priority"`
	Source      string    `json:"source"`
	Blocker     string    `json:"blocker,omitempty"`
	Plan        string    `json:"plan,omitempty"`
	Output      string    `json:"output,omitempty"`
	Error       string    `json:"error,omitempty"`
	AgentID     string    `json:"agent_id,omitempty"`
	Product     string    `json:"product,omitempty"`
	ParentID    *int64    `json:"parent_id,omitempty"`
	Metadata    string    `json:"metadata,omitempty"`
	RetryCount  int       `json:"retry_count"`
	MaxRetries  int       `json:"max_retries"`
	CreatedAt   time.Time `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// NewTask creates a task with sensible defaults.
func NewTask(title, description string) Task {
	return Task{
		Title:       title,
		Description: description,
		Stage:       StageBacklog,
		Source:      "manual",
		MaxRetries:  3,
		CreatedAt:   time.Now(),
	}
}

// TaskFilter defines query filters for listing tasks.
type TaskFilter struct {
	Stage   Stage  `json:"stage,omitempty"`
	Product string `json:"product,omitempty"`
}

// TaskUpdate holds partial update fields. Nil pointers mean "don't update".
type TaskUpdate struct {
	Title       *string  `json:"title,omitempty"`
	Description *string  `json:"description,omitempty"`
	Acceptance  *string  `json:"acceptance,omitempty"`
	Stage       *Stage   `json:"stage,omitempty"`
	Substep     *Substep `json:"substep,omitempty"`
	Priority    *int     `json:"priority,omitempty"`
	Blocker     *string  `json:"blocker,omitempty"`
	Plan        *string  `json:"plan,omitempty"`
	Output      *string  `json:"output,omitempty"`
	Error       *string  `json:"error,omitempty"`
	AgentID     *string  `json:"agent_id,omitempty"`
	Product     *string  `json:"product,omitempty"`
	Metadata    *string  `json:"metadata,omitempty"`
}
```

**Step 4: Run tests to verify they pass**

```bash
cd /home/rishav/soul && go test ./internal/planner/ -v
```

Expected: all tests PASS.

**Step 5: Commit**

```bash
git add internal/planner/
git commit -m "feat(planner): add types — Stage, Substep, Task, transitions"
```

---

## Task 3: Planner store (`internal/planner/store.go`)

**Files:**
- Create: `internal/planner/store.go`
- Test: `internal/planner/store_test.go`

**Step 1: Write the failing test**

Create `internal/planner/store_test.go`:

```go
package planner

import (
	"os"
	"path/filepath"
	"testing"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := OpenStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpenStore(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	s, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	s.Close()
	if _, err := os.Stat(dbPath); err != nil {
		t.Errorf("db file should exist: %v", err)
	}
}

func TestCreateAndGet(t *testing.T) {
	s := tempStore(t)
	task := NewTask("Test task", "Do something")
	task.Priority = 2
	task.Product = "compliance"

	id, err := s.Create(task)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive ID, got %d", id)
	}

	got, err := s.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "Test task" {
		t.Errorf("title = %q, want %q", got.Title, "Test task")
	}
	if got.Stage != StageBacklog {
		t.Errorf("stage = %q, want %q", got.Stage, StageBacklog)
	}
	if got.Priority != 2 {
		t.Errorf("priority = %d, want 2", got.Priority)
	}
	if got.Product != "compliance" {
		t.Errorf("product = %q, want %q", got.Product, "compliance")
	}
}

func TestGetNotFound(t *testing.T) {
	s := tempStore(t)
	_, err := s.Get(999)
	if err == nil {
		t.Fatal("expected error for non-existent task")
	}
}

func TestList(t *testing.T) {
	s := tempStore(t)
	s.Create(NewTask("Task 1", ""))
	s.Create(NewTask("Task 2", ""))
	task3 := NewTask("Task 3", "")
	task3.Stage = StageActive
	task3.Product = "compliance"
	// Create then update stage (since Create always uses backlog)
	id3, _ := s.Create(task3)
	stage := StageActive
	s.Update(id3, TaskUpdate{Stage: &stage})

	// List all
	all, err := s.List(TaskFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(all))
	}

	// Filter by stage
	backlog, err := s.List(TaskFilter{Stage: StageBacklog})
	if err != nil {
		t.Fatalf("List backlog: %v", err)
	}
	if len(backlog) != 2 {
		t.Errorf("expected 2 backlog tasks, got %d", len(backlog))
	}

	// Filter by product
	compliance, err := s.List(TaskFilter{Product: "compliance"})
	if err != nil {
		t.Fatalf("List compliance: %v", err)
	}
	if len(compliance) != 1 {
		t.Errorf("expected 1 compliance task, got %d", len(compliance))
	}
}

func TestUpdate(t *testing.T) {
	s := tempStore(t)
	id, _ := s.Create(NewTask("Original", ""))

	newTitle := "Updated"
	newStage := StageActive
	err := s.Update(id, TaskUpdate{Title: &newTitle, Stage: &newStage})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, _ := s.Get(id)
	if got.Title != "Updated" {
		t.Errorf("title = %q, want %q", got.Title, "Updated")
	}
	if got.Stage != StageActive {
		t.Errorf("stage = %q, want %q", got.Stage, StageActive)
	}
}

func TestDelete(t *testing.T) {
	s := tempStore(t)
	id, _ := s.Create(NewTask("To delete", ""))

	err := s.Delete(id)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = s.Get(id)
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestNextReady(t *testing.T) {
	s := tempStore(t)

	// Empty store
	_, err := s.NextReady()
	if err == nil {
		t.Fatal("expected error on empty store")
	}

	// Add tasks with different priorities
	t1 := NewTask("Low priority", "")
	t1.Priority = 1
	s.Create(t1)

	t2 := NewTask("High priority", "")
	t2.Priority = 5
	s.Create(t2)

	got, err := s.NextReady()
	if err != nil {
		t.Fatalf("NextReady: %v", err)
	}
	if got.Title != "High priority" {
		t.Errorf("NextReady should return highest priority, got %q", got.Title)
	}
}

func TestDependencies(t *testing.T) {
	s := tempStore(t)
	id1, _ := s.Create(NewTask("Task 1", ""))
	id2, _ := s.Create(NewTask("Task 2 (depends on 1)", ""))

	err := s.AddDependency(id2, id1)
	if err != nil {
		t.Fatalf("AddDependency: %v", err)
	}

	// Task 2 should NOT appear in NextReady because Task 1 is not done
	got, err := s.NextReady()
	if err != nil {
		t.Fatalf("NextReady: %v", err)
	}
	if got.ID == id2 {
		t.Error("NextReady should skip task with unresolved dependency")
	}

	// Mark Task 1 as done
	stage := StageDone
	s.Update(id1, TaskUpdate{Stage: &stage})

	// Now Task 2 should be available
	got, err = s.NextReady()
	if err != nil {
		t.Fatalf("NextReady after dep resolved: %v", err)
	}
	if got.ID != id2 {
		t.Errorf("NextReady should return task 2 now, got ID=%d", got.ID)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd /home/rishav/soul && go test ./internal/planner/ -v -run TestOpenStore
```

Expected: FAIL — `OpenStore` not defined.

**Step 3: Write the implementation**

Create `internal/planner/store.go`:

```go
package planner

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS tasks (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    title         TEXT NOT NULL,
    description   TEXT DEFAULT '',
    acceptance    TEXT DEFAULT '',
    stage         TEXT DEFAULT 'backlog',
    substep       TEXT DEFAULT '',
    priority      INTEGER DEFAULT 0,
    source        TEXT DEFAULT 'manual',
    blocker       TEXT DEFAULT '',
    plan          TEXT DEFAULT '',
    output        TEXT DEFAULT '',
    error         TEXT DEFAULT '',
    agent_id      TEXT DEFAULT '',
    product       TEXT DEFAULT '',
    parent_id     INTEGER,
    metadata      TEXT DEFAULT '',
    retry_count   INTEGER DEFAULT 0,
    max_retries   INTEGER DEFAULT 3,
    created_at    TEXT NOT NULL,
    started_at    TEXT DEFAULT '',
    completed_at  TEXT DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_tasks_stage ON tasks(stage);
CREATE INDEX IF NOT EXISTS idx_tasks_priority ON tasks(priority DESC);

CREATE TABLE IF NOT EXISTS task_dependencies (
    task_id    INTEGER REFERENCES tasks(id) ON DELETE CASCADE,
    depends_on INTEGER REFERENCES tasks(id) ON DELETE CASCADE,
    PRIMARY KEY (task_id, depends_on)
);
`

// Store provides SQLite-backed CRUD for planner tasks.
type Store struct {
	db *sql.DB
}

// OpenStore opens (or creates) a SQLite database and runs migrations.
func OpenStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(wal)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// Create inserts a new task and returns its ID.
func (s *Store) Create(t Task) (int64, error) {
	res, err := s.db.Exec(`
		INSERT INTO tasks (title, description, acceptance, stage, substep, priority, source,
			blocker, plan, output, error, agent_id, product, parent_id, metadata,
			retry_count, max_retries, created_at, started_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.Title, t.Description, t.Acceptance, string(t.Stage), string(t.Substep),
		t.Priority, t.Source, t.Blocker, t.Plan, t.Output, t.Error,
		t.AgentID, t.Product, t.ParentID, t.Metadata,
		t.RetryCount, t.MaxRetries, t.CreatedAt.Format(time.RFC3339),
		formatTimePtr(t.StartedAt), formatTimePtr(t.CompletedAt),
	)
	if err != nil {
		return 0, fmt.Errorf("insert task: %w", err)
	}
	return res.LastInsertId()
}

// Get fetches a single task by ID.
func (s *Store) Get(id int64) (Task, error) {
	row := s.db.QueryRow(`SELECT id, title, description, acceptance, stage, substep,
		priority, source, blocker, plan, output, error, agent_id, product,
		parent_id, metadata, retry_count, max_retries, created_at, started_at, completed_at
		FROM tasks WHERE id = ?`, id)
	return scanTask(row)
}

// List returns tasks matching the given filter, ordered by priority DESC then created_at ASC.
func (s *Store) List(f TaskFilter) ([]Task, error) {
	query := `SELECT id, title, description, acceptance, stage, substep,
		priority, source, blocker, plan, output, error, agent_id, product,
		parent_id, metadata, retry_count, max_retries, created_at, started_at, completed_at
		FROM tasks WHERE 1=1`
	var args []any

	if f.Stage != "" {
		query += " AND stage = ?"
		args = append(args, string(f.Stage))
	}
	if f.Product != "" {
		query += " AND product = ?"
		args = append(args, f.Product)
	}

	query += " ORDER BY priority DESC, created_at ASC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		t, err := scanTaskRows(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// Update applies partial updates to a task.
func (s *Store) Update(id int64, u TaskUpdate) error {
	// Build dynamic SET clause.
	var sets []string
	var args []any

	if u.Title != nil {
		sets = append(sets, "title = ?")
		args = append(args, *u.Title)
	}
	if u.Description != nil {
		sets = append(sets, "description = ?")
		args = append(args, *u.Description)
	}
	if u.Acceptance != nil {
		sets = append(sets, "acceptance = ?")
		args = append(args, *u.Acceptance)
	}
	if u.Stage != nil {
		sets = append(sets, "stage = ?")
		args = append(args, string(*u.Stage))
	}
	if u.Substep != nil {
		sets = append(sets, "substep = ?")
		args = append(args, string(*u.Substep))
	}
	if u.Priority != nil {
		sets = append(sets, "priority = ?")
		args = append(args, *u.Priority)
	}
	if u.Blocker != nil {
		sets = append(sets, "blocker = ?")
		args = append(args, *u.Blocker)
	}
	if u.Plan != nil {
		sets = append(sets, "plan = ?")
		args = append(args, *u.Plan)
	}
	if u.Output != nil {
		sets = append(sets, "output = ?")
		args = append(args, *u.Output)
	}
	if u.Error != nil {
		sets = append(sets, "error = ?")
		args = append(args, *u.Error)
	}
	if u.AgentID != nil {
		sets = append(sets, "agent_id = ?")
		args = append(args, *u.AgentID)
	}
	if u.Product != nil {
		sets = append(sets, "product = ?")
		args = append(args, *u.Product)
	}
	if u.Metadata != nil {
		sets = append(sets, "metadata = ?")
		args = append(args, *u.Metadata)
	}

	if len(sets) == 0 {
		return nil
	}

	query := "UPDATE tasks SET "
	for i, set := range sets {
		if i > 0 {
			query += ", "
		}
		query += set
	}
	query += " WHERE id = ?"
	args = append(args, id)

	_, err := s.db.Exec(query, args...)
	return err
}

// Delete removes a task by ID.
func (s *Store) Delete(id int64) error {
	_, err := s.db.Exec("DELETE FROM tasks WHERE id = ?", id)
	return err
}

// NextReady returns the highest-priority task in BACKLOG with no unresolved dependencies.
func (s *Store) NextReady() (Task, error) {
	row := s.db.QueryRow(`
		SELECT id, title, description, acceptance, stage, substep,
			priority, source, blocker, plan, output, error, agent_id, product,
			parent_id, metadata, retry_count, max_retries, created_at, started_at, completed_at
		FROM tasks
		WHERE stage = 'backlog'
		AND id NOT IN (
			SELECT td.task_id FROM task_dependencies td
			JOIN tasks dep ON dep.id = td.depends_on
			WHERE dep.stage != 'done'
		)
		ORDER BY priority DESC, created_at ASC
		LIMIT 1`)
	return scanTask(row)
}

// AddDependency records that taskID depends on dependsOn.
func (s *Store) AddDependency(taskID, dependsOn int64) error {
	_, err := s.db.Exec("INSERT OR IGNORE INTO task_dependencies (task_id, depends_on) VALUES (?, ?)", taskID, dependsOn)
	return err
}

// RemoveDependency removes a dependency.
func (s *Store) RemoveDependency(taskID, dependsOn int64) error {
	_, err := s.db.Exec("DELETE FROM task_dependencies WHERE task_id = ? AND depends_on = ?", taskID, dependsOn)
	return err
}

// scanner is an interface satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanTask(row *sql.Row) (Task, error) {
	var t Task
	var stage, substep, source, createdAt, startedAt, completedAt string
	var parentID sql.NullInt64
	err := row.Scan(&t.ID, &t.Title, &t.Description, &t.Acceptance,
		&stage, &substep, &t.Priority, &source, &t.Blocker, &t.Plan,
		&t.Output, &t.Error, &t.AgentID, &t.Product, &parentID,
		&t.Metadata, &t.RetryCount, &t.MaxRetries, &createdAt, &startedAt, &completedAt)
	if err != nil {
		return Task{}, err
	}
	t.Stage = Stage(stage)
	t.Substep = Substep(substep)
	t.Source = source
	t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	t.StartedAt = parseTimePtr(startedAt)
	t.CompletedAt = parseTimePtr(completedAt)
	if parentID.Valid {
		t.ParentID = &parentID.Int64
	}
	return t, nil
}

func scanTaskRows(rows *sql.Rows) (Task, error) {
	var t Task
	var stage, substep, source, createdAt, startedAt, completedAt string
	var parentID sql.NullInt64
	err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Acceptance,
		&stage, &substep, &t.Priority, &source, &t.Blocker, &t.Plan,
		&t.Output, &t.Error, &t.AgentID, &t.Product, &parentID,
		&t.Metadata, &t.RetryCount, &t.MaxRetries, &createdAt, &startedAt, &completedAt)
	if err != nil {
		return Task{}, err
	}
	t.Stage = Stage(stage)
	t.Substep = Substep(substep)
	t.Source = source
	t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	t.StartedAt = parseTimePtr(startedAt)
	t.CompletedAt = parseTimePtr(completedAt)
	if parentID.Valid {
		t.ParentID = &parentID.Int64
	}
	return t, nil
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

func parseTimePtr(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}
```

**Step 4: Run tests to verify they pass**

```bash
cd /home/rishav/soul && go test ./internal/planner/ -v
```

Expected: all tests PASS.

**Step 5: Commit**

```bash
git add internal/planner/
git commit -m "feat(planner): add SQLite store — CRUD, NextReady, dependencies"
```

---

## Task 4: REST API endpoints (`internal/server/tasks.go`)

**Files:**
- Create: `internal/server/tasks.go`
- Modify: `internal/server/server.go` — add `planner *planner.Store` field
- Modify: `internal/server/routes.go` — register task routes
- Modify: `cmd/soul/main.go` — open planner store and pass to server

**Step 1: Write the handler file**

Create `internal/server/tasks.go`:

```go
package server

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/rishav1305/soul/internal/planner"
)

// handleTaskCreate handles POST /api/tasks.
func (s *Server) handleTaskCreate(w http.ResponseWriter, r *http.Request) {
	if s.planner == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "planner not configured"})
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
		return
	}

	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Priority    int    `json:"priority"`
		Product     string `json:"product"`
		Acceptance  string `json:"acceptance"`
		Source      string `json:"source"`
		ParentID    *int64 `json:"parent_id"`
		Metadata    string `json:"metadata"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title is required"})
		return
	}

	task := planner.NewTask(req.Title, req.Description)
	task.Priority = req.Priority
	task.Product = req.Product
	task.Acceptance = req.Acceptance
	task.ParentID = req.ParentID
	task.Metadata = req.Metadata
	if req.Source != "" {
		task.Source = req.Source
	}

	id, err := s.planner.Create(task)
	if err != nil {
		log.Printf("[planner] create error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create task"})
		return
	}

	created, err := s.planner.Get(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to fetch created task"})
		return
	}

	// Broadcast to WebSocket clients.
	s.broadcastTaskEvent("task.created", created)

	writeJSON(w, http.StatusCreated, created)
}

// handleTaskList handles GET /api/tasks.
func (s *Server) handleTaskList(w http.ResponseWriter, r *http.Request) {
	if s.planner == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "planner not configured"})
		return
	}

	filter := planner.TaskFilter{
		Stage:   planner.Stage(r.URL.Query().Get("stage")),
		Product: r.URL.Query().Get("product"),
	}

	tasks, err := s.planner.List(filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list tasks"})
		return
	}
	if tasks == nil {
		tasks = []planner.Task{}
	}

	writeJSON(w, http.StatusOK, tasks)
}

// handleTaskGet handles GET /api/tasks/{id}.
func (s *Server) handleTaskGet(w http.ResponseWriter, r *http.Request) {
	if s.planner == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "planner not configured"})
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid task ID"})
		return
	}

	task, err := s.planner.Get(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
		return
	}

	writeJSON(w, http.StatusOK, task)
}

// handleTaskUpdate handles PATCH /api/tasks/{id}.
func (s *Server) handleTaskUpdate(w http.ResponseWriter, r *http.Request) {
	if s.planner == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "planner not configured"})
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid task ID"})
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
		return
	}

	var update planner.TaskUpdate
	if err := json.Unmarshal(body, &update); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if err := s.planner.Update(id, update); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update task"})
		return
	}

	updated, err := s.planner.Get(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to fetch updated task"})
		return
	}

	s.broadcastTaskEvent("task.updated", updated)

	writeJSON(w, http.StatusOK, updated)
}

// handleTaskDelete handles DELETE /api/tasks/{id}.
func (s *Server) handleTaskDelete(w http.ResponseWriter, r *http.Request) {
	if s.planner == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "planner not configured"})
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid task ID"})
		return
	}

	if err := s.planner.Delete(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete task"})
		return
	}

	s.broadcastTaskEvent("task.deleted", map[string]int64{"id": id})

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleTaskMove handles POST /api/tasks/{id}/move.
func (s *Server) handleTaskMove(w http.ResponseWriter, r *http.Request) {
	if s.planner == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "planner not configured"})
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid task ID"})
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
		return
	}

	var req struct {
		Stage   planner.Stage `json:"stage"`
		Comment string        `json:"comment"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if !req.Stage.Valid() {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid stage"})
		return
	}

	// Validate transition.
	task, err := s.planner.Get(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
		return
	}

	if !planner.ValidTransition(task.Stage, req.Stage) {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid transition from " + string(task.Stage) + " to " + string(req.Stage),
		})
		return
	}

	stage := req.Stage
	update := planner.TaskUpdate{Stage: &stage}
	if err := s.planner.Update(id, update); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to move task"})
		return
	}

	updated, _ := s.planner.Get(id)
	s.broadcastTaskEvent("task.updated", updated)

	writeJSON(w, http.StatusOK, updated)
}

// broadcastTaskEvent sends a task event to all connected WebSocket clients.
func (s *Server) broadcastTaskEvent(eventType string, data any) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Printf("[planner] failed to marshal event: %v", err)
		return
	}
	s.broadcast(WSMessage{
		Type: eventType,
		Data: json.RawMessage(jsonData),
	})
}
```

**Step 2: Add planner field to Server and broadcast method**

Modify `internal/server/server.go`:
- Add `planner *planner.Store` field to the `Server` struct
- Add a `broadcast` method stub (sends to all WS clients)
- Accept planner in constructors

Modify `internal/server/routes.go`:
- Register the 6 task endpoints in `registerRoutes()`

Modify `internal/server/ws.go`:
- Add a client registry so `broadcast()` can send to all connected clients

Modify `cmd/soul/main.go`:
- Open planner store at `~/.soul/planner.db`
- Pass to server constructor

**Step 3: Register routes in `routes.go`**

Add to `registerRoutes()`:

```go
// Planner task endpoints.
s.mux.HandleFunc("POST /api/tasks", s.handleTaskCreate)
s.mux.HandleFunc("GET /api/tasks", s.handleTaskList)
s.mux.HandleFunc("GET /api/tasks/{id}", s.handleTaskGet)
s.mux.HandleFunc("PATCH /api/tasks/{id}", s.handleTaskUpdate)
s.mux.HandleFunc("DELETE /api/tasks/{id}", s.handleTaskDelete)
s.mux.HandleFunc("POST /api/tasks/{id}/move", s.handleTaskMove)
```

**Step 4: Build and verify**

```bash
cd /home/rishav/soul && go build ./cmd/soul/
```

Expected: clean build.

**Step 5: Commit**

```bash
git add internal/server/ cmd/soul/main.go
git commit -m "feat(planner): add REST API — CRUD + move endpoints with WS broadcast"
```

---

## Task 5: Frontend types and planner hook (`web/src/`)

**Files:**
- Modify: `web/src/lib/types.ts` — add Task types
- Create: `web/src/hooks/usePlanner.ts` — REST + WebSocket hook

**Step 1: Add Task types to `types.ts`**

Append to `web/src/lib/types.ts`:

```typescript
export type TaskStage = 'backlog' | 'brainstorm' | 'active' | 'blocked' | 'validation' | 'done';
export type TaskSubstep = 'tdd' | 'implementing' | 'reviewing' | 'qa_test' | 'e2e_test' | 'security_review';

export interface PlannerTask {
  id: number;
  title: string;
  description: string;
  acceptance: string;
  stage: TaskStage;
  substep: TaskSubstep | '';
  priority: number;
  source: string;
  blocker: string;
  plan: string;
  output: string;
  error: string;
  agent_id: string;
  product: string;
  parent_id: number | null;
  metadata: string;
  retry_count: number;
  max_retries: number;
  created_at: string;
  started_at: string | null;
  completed_at: string | null;
}
```

**Step 2: Create `usePlanner.ts`**

```typescript
import { useState, useEffect, useCallback } from 'react';
import { useWebSocket } from './useWebSocket.ts';
import type { PlannerTask, TaskStage, WSMessage } from '../lib/types.ts';

const API_BASE = '/api/tasks';

export function usePlanner() {
  const { onMessage } = useWebSocket();
  const [tasks, setTasks] = useState<PlannerTask[]>([]);
  const [loading, setLoading] = useState(true);

  // Fetch all tasks on mount.
  useEffect(() => {
    fetch(API_BASE)
      .then((r) => r.json())
      .then((data: PlannerTask[]) => {
        setTasks(data);
        setLoading(false);
      })
      .catch(() => setLoading(false));
  }, []);

  // Listen for real-time updates via WebSocket.
  useEffect(() => {
    const unsubscribe = onMessage((msg: WSMessage) => {
      switch (msg.type) {
        case 'task.created': {
          const task = msg.data as PlannerTask;
          setTasks((prev) => [...prev, task]);
          break;
        }
        case 'task.updated': {
          const task = msg.data as PlannerTask;
          setTasks((prev) => prev.map((t) => (t.id === task.id ? task : t)));
          break;
        }
        case 'task.deleted': {
          const { id } = msg.data as { id: number };
          setTasks((prev) => prev.filter((t) => t.id !== id));
          break;
        }
      }
    });
    return unsubscribe;
  }, [onMessage]);

  const createTask = useCallback(async (title: string, description = '', priority = 0, product = '') => {
    const res = await fetch(API_BASE, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title, description, priority, product }),
    });
    return res.json() as Promise<PlannerTask>;
  }, []);

  const updateTask = useCallback(async (id: number, updates: Partial<PlannerTask>) => {
    const res = await fetch(`${API_BASE}/${id}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(updates),
    });
    return res.json() as Promise<PlannerTask>;
  }, []);

  const deleteTask = useCallback(async (id: number) => {
    await fetch(`${API_BASE}/${id}`, { method: 'DELETE' });
  }, []);

  const moveTask = useCallback(async (id: number, stage: TaskStage, comment = '') => {
    const res = await fetch(`${API_BASE}/${id}/move`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ stage, comment }),
    });
    return res.json() as Promise<PlannerTask>;
  }, []);

  // Group tasks by stage for Kanban view.
  const tasksByStage = tasks.reduce(
    (acc, task) => {
      acc[task.stage] = acc[task.stage] || [];
      acc[task.stage].push(task);
      return acc;
    },
    {} as Record<TaskStage, PlannerTask[]>,
  );

  return { tasks, tasksByStage, loading, createTask, updateTask, deleteTask, moveTask };
}
```

**Step 3: Verify build**

```bash
cd /home/rishav/soul/web && npx vite build
```

Expected: clean build.

**Step 4: Commit**

```bash
git add web/src/lib/types.ts web/src/hooks/usePlanner.ts
git commit -m "feat(planner): add frontend types and usePlanner hook"
```

---

## Task 6: Kanban board UI components

**Files:**
- Create: `web/src/components/planner/TaskCard.tsx`
- Create: `web/src/components/planner/StageColumn.tsx`
- Create: `web/src/components/planner/KanbanBoard.tsx`
- Create: `web/src/components/planner/NewTaskForm.tsx`
- Create: `web/src/components/planner/TaskDetail.tsx`

**Step 1: Create TaskCard.tsx**

The embeddable widget/block — shows title, priority, stage badge, substep, product icon. The core visual building block of the planner.

```tsx
import type { PlannerTask } from '../../lib/types.ts';

const priorityColors: Record<number, string> = {
  0: 'border-l-zinc-600',
  1: 'border-l-sky-500',
  2: 'border-l-amber-500',
  3: 'border-l-red-500',
};

const substepLabels: Record<string, string> = {
  tdd: 'TDD [1/6]',
  implementing: 'Implementing [2/6]',
  reviewing: 'Reviewing [3/6]',
  qa_test: 'QA Test [4/6]',
  e2e_test: 'E2E Test [5/6]',
  security_review: 'Security [6/6]',
};

interface TaskCardProps {
  task: PlannerTask;
  onClick: (task: PlannerTask) => void;
}

export default function TaskCard({ task, onClick }: TaskCardProps) {
  const borderColor = priorityColors[Math.min(task.priority, 3)] ?? 'border-l-zinc-600';

  return (
    <button
      type="button"
      onClick={() => onClick(task)}
      className={`w-full text-left bg-zinc-800 rounded-lg border-l-4 ${borderColor} p-3 hover:bg-zinc-750 transition-colors cursor-pointer`}
    >
      <div className="flex items-start justify-between gap-2">
        <span className="text-sm font-medium text-zinc-100 line-clamp-2">{task.title}</span>
        <span className="text-xs text-zinc-500 shrink-0">#{task.id}</span>
      </div>
      {task.description && (
        <p className="text-xs text-zinc-400 mt-1 line-clamp-1">{task.description}</p>
      )}
      <div className="flex items-center gap-2 mt-2 flex-wrap">
        {task.product && (
          <span className="text-[10px] bg-zinc-700 text-zinc-300 px-1.5 py-0.5 rounded">{task.product}</span>
        )}
        {task.substep && (
          <span className="text-[10px] bg-sky-900 text-sky-300 px-1.5 py-0.5 rounded">
            {substepLabels[task.substep] ?? task.substep}
          </span>
        )}
        {task.blocker && (
          <span className="text-[10px] bg-red-900 text-red-300 px-1.5 py-0.5 rounded">Blocked</span>
        )}
      </div>
    </button>
  );
}
```

**Step 2: Create StageColumn.tsx**

```tsx
import type { PlannerTask, TaskStage } from '../../lib/types.ts';
import TaskCard from './TaskCard.tsx';

const stageLabels: Record<TaskStage, string> = {
  backlog: 'Backlog',
  brainstorm: 'Brainstorm',
  active: 'Active',
  blocked: 'Blocked',
  validation: 'Validation',
  done: 'Done',
};

const stageColors: Record<TaskStage, string> = {
  backlog: 'text-zinc-400',
  brainstorm: 'text-purple-400',
  active: 'text-sky-400',
  blocked: 'text-red-400',
  validation: 'text-amber-400',
  done: 'text-green-400',
};

interface StageColumnProps {
  stage: TaskStage;
  tasks: PlannerTask[];
  onTaskClick: (task: PlannerTask) => void;
}

export default function StageColumn({ stage, tasks, onTaskClick }: StageColumnProps) {
  return (
    <div className="flex flex-col min-w-[200px] flex-1">
      <div className="flex items-center gap-2 px-2 py-2 border-b border-zinc-800">
        <span className={`text-xs font-semibold uppercase ${stageColors[stage]}`}>
          {stageLabels[stage]}
        </span>
        <span className="text-xs text-zinc-500">{tasks.length}</span>
      </div>
      <div className="flex-1 overflow-y-auto p-2 space-y-2">
        {tasks.map((task) => (
          <TaskCard key={task.id} task={task} onClick={onTaskClick} />
        ))}
      </div>
    </div>
  );
}
```

**Step 3: Create KanbanBoard.tsx**

```tsx
import { useState } from 'react';
import type { PlannerTask, TaskStage } from '../../lib/types.ts';
import { usePlanner } from '../../hooks/usePlanner.ts';
import StageColumn from './StageColumn.tsx';
import NewTaskForm from './NewTaskForm.tsx';
import TaskDetail from './TaskDetail.tsx';

const stages: TaskStage[] = ['backlog', 'brainstorm', 'active', 'blocked', 'validation', 'done'];

export default function KanbanBoard() {
  const { tasksByStage, loading, createTask, moveTask, deleteTask } = usePlanner();
  const [selectedTask, setSelectedTask] = useState<PlannerTask | null>(null);
  const [showNewForm, setShowNewForm] = useState(false);

  if (loading) {
    return <div className="flex items-center justify-center h-full text-zinc-500">Loading tasks...</div>;
  }

  return (
    <div className="flex flex-col h-full">
      <div className="flex items-center justify-between px-4 py-2 border-b border-zinc-800">
        <div className="flex items-center gap-3">
          <span className="text-sm font-semibold text-zinc-200">Task Manager</span>
          <button
            type="button"
            className="text-xs text-zinc-400 bg-zinc-800 hover:bg-zinc-700 px-2 py-1 rounded"
          >
            Kanban
          </button>
        </div>
        <button
          type="button"
          onClick={() => setShowNewForm(true)}
          className="text-xs bg-zinc-700 hover:bg-zinc-600 text-zinc-200 px-3 py-1 rounded"
        >
          + New Task
        </button>
      </div>

      <div className="flex-1 flex overflow-x-auto overflow-y-hidden">
        {stages.map((stage) => (
          <StageColumn
            key={stage}
            stage={stage}
            tasks={tasksByStage[stage] ?? []}
            onTaskClick={setSelectedTask}
          />
        ))}
      </div>

      {showNewForm && (
        <NewTaskForm
          onSubmit={async (title, description, priority, product) => {
            await createTask(title, description, priority, product);
            setShowNewForm(false);
          }}
          onClose={() => setShowNewForm(false)}
        />
      )}

      {selectedTask && (
        <TaskDetail
          task={selectedTask}
          onClose={() => setSelectedTask(null)}
          onMove={async (stage) => {
            await moveTask(selectedTask.id, stage);
            setSelectedTask(null);
          }}
          onDelete={async () => {
            await deleteTask(selectedTask.id);
            setSelectedTask(null);
          }}
        />
      )}
    </div>
  );
}
```

**Step 4: Create NewTaskForm.tsx**

```tsx
import { useState } from 'react';

interface NewTaskFormProps {
  onSubmit: (title: string, description: string, priority: number, product: string) => void;
  onClose: () => void;
}

export default function NewTaskForm({ onSubmit, onClose }: NewTaskFormProps) {
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [priority, setPriority] = useState(0);
  const [product, setProduct] = useState('');

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={onClose}>
      <div className="bg-zinc-900 rounded-lg border border-zinc-700 p-6 w-full max-w-md" onClick={(e) => e.stopPropagation()}>
        <h2 className="text-lg font-semibold text-zinc-100 mb-4">New Task</h2>
        <div className="space-y-3">
          <input
            type="text"
            placeholder="Task title"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            className="w-full bg-zinc-800 text-zinc-100 rounded px-3 py-2 text-sm border border-zinc-700 focus:border-zinc-500 outline-none"
            autoFocus
          />
          <textarea
            placeholder="Description (optional)"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            className="w-full bg-zinc-800 text-zinc-100 rounded px-3 py-2 text-sm border border-zinc-700 focus:border-zinc-500 outline-none resize-none h-20"
          />
          <div className="flex gap-3">
            <select
              value={priority}
              onChange={(e) => setPriority(Number(e.target.value))}
              className="bg-zinc-800 text-zinc-100 rounded px-3 py-2 text-sm border border-zinc-700"
            >
              <option value={0}>Low</option>
              <option value={1}>Normal</option>
              <option value={2}>High</option>
              <option value={3}>Critical</option>
            </select>
            <input
              type="text"
              placeholder="Product (optional)"
              value={product}
              onChange={(e) => setProduct(e.target.value)}
              className="flex-1 bg-zinc-800 text-zinc-100 rounded px-3 py-2 text-sm border border-zinc-700 focus:border-zinc-500 outline-none"
            />
          </div>
        </div>
        <div className="flex justify-end gap-2 mt-4">
          <button type="button" onClick={onClose} className="text-sm text-zinc-400 px-3 py-1.5 hover:text-zinc-200">Cancel</button>
          <button
            type="button"
            onClick={() => title && onSubmit(title, description, priority, product)}
            className="text-sm bg-zinc-700 hover:bg-zinc-600 text-zinc-100 px-4 py-1.5 rounded"
          >
            Create
          </button>
        </div>
      </div>
    </div>
  );
}
```

**Step 5: Create TaskDetail.tsx**

```tsx
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import type { PlannerTask, TaskStage } from '../../lib/types.ts';
import { ValidTransitions } from './transitions.ts';

interface TaskDetailProps {
  task: PlannerTask;
  onClose: () => void;
  onMove: (stage: TaskStage) => void;
  onDelete: () => void;
}

export default function TaskDetail({ task, onClose, onMove, onDelete }: TaskDetailProps) {
  const nextStages = ValidTransitions[task.stage] ?? [];

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={onClose}>
      <div className="bg-zinc-900 rounded-lg border border-zinc-700 p-6 w-full max-w-2xl max-h-[80vh] overflow-y-auto" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-start justify-between mb-4">
          <div>
            <h2 className="text-lg font-semibold text-zinc-100">{task.title}</h2>
            <span className="text-xs text-zinc-500">#{task.id} &middot; {task.stage} &middot; Priority {task.priority}</span>
          </div>
          <button type="button" onClick={onClose} className="text-zinc-500 hover:text-zinc-300 text-lg">&times;</button>
        </div>

        {task.description && (
          <div className="mb-4">
            <h3 className="text-xs font-semibold text-zinc-400 uppercase mb-1">Description</h3>
            <p className="text-sm text-zinc-300">{task.description}</p>
          </div>
        )}

        {task.acceptance && (
          <div className="mb-4">
            <h3 className="text-xs font-semibold text-zinc-400 uppercase mb-1">Acceptance Criteria</h3>
            <p className="text-sm text-zinc-300">{task.acceptance}</p>
          </div>
        )}

        {task.plan && (
          <div className="mb-4">
            <h3 className="text-xs font-semibold text-zinc-400 uppercase mb-1">Plan</h3>
            <div className="prose prose-invert prose-sm max-w-none">
              <ReactMarkdown remarkPlugins={[remarkGfm]}>{task.plan}</ReactMarkdown>
            </div>
          </div>
        )}

        {task.output && (
          <div className="mb-4">
            <h3 className="text-xs font-semibold text-zinc-400 uppercase mb-1">Output</h3>
            <div className="prose prose-invert prose-sm max-w-none">
              <ReactMarkdown remarkPlugins={[remarkGfm]}>{task.output}</ReactMarkdown>
            </div>
          </div>
        )}

        {task.error && (
          <div className="mb-4">
            <h3 className="text-xs font-semibold text-zinc-400 uppercase mb-1">Error</h3>
            <p className="text-sm text-red-400">{task.error}</p>
          </div>
        )}

        {task.blocker && (
          <div className="mb-4">
            <h3 className="text-xs font-semibold text-zinc-400 uppercase mb-1">Blocker</h3>
            <p className="text-sm text-red-400">{task.blocker}</p>
          </div>
        )}

        <div className="flex items-center gap-2 mt-6 pt-4 border-t border-zinc-800">
          {nextStages.map((stage) => (
            <button
              key={stage}
              type="button"
              onClick={() => onMove(stage)}
              className="text-xs bg-zinc-700 hover:bg-zinc-600 text-zinc-200 px-3 py-1.5 rounded capitalize"
            >
              Move to {stage}
            </button>
          ))}
          <button
            type="button"
            onClick={onDelete}
            className="text-xs text-red-400 hover:text-red-300 px-3 py-1.5 ml-auto"
          >
            Delete
          </button>
        </div>
      </div>
    </div>
  );
}
```

**Step 6: Create transitions helper**

Create `web/src/components/planner/transitions.ts`:

```typescript
import type { TaskStage } from '../../lib/types.ts';

export const ValidTransitions: Record<TaskStage, TaskStage[]> = {
  backlog: ['brainstorm', 'active'],
  brainstorm: ['active'],
  active: ['blocked', 'validation'],
  blocked: ['active'],
  validation: ['done', 'active'],
  done: [],
};
```

**Step 7: Verify build**

```bash
cd /home/rishav/soul/web && npx vite build
```

Expected: clean build.

**Step 8: Commit**

```bash
git add web/src/components/planner/
git commit -m "feat(planner): add Kanban UI — TaskCard, StageColumn, Board, NewTask, Detail"
```

---

## Task 7: Two-panel layout and App.tsx wiring

**Files:**
- Modify: `web/src/App.tsx` — replace compliance panel with Kanban board in two-panel layout

**Step 1: Update App.tsx**

Replace the current layout with a two-panel vertical split: chat left, task manager right.

```tsx
import ChatView from './components/chat/ChatView.tsx';
import KanbanBoard from './components/planner/KanbanBoard.tsx';
import { WebSocketContext, useWebSocketProvider } from './hooks/useWebSocket.ts';

function AppContent() {
  return (
    <div className="h-screen bg-zinc-950 text-zinc-100 flex flex-col">
      <header className="h-12 border-b border-zinc-800 flex items-center px-4 shrink-0">
        <span className="text-lg font-bold">&#9670; Soul</span>
      </header>
      <main className="flex-1 flex overflow-hidden">
        <div className="w-1/2 min-w-[400px] border-r border-zinc-800">
          <ChatView />
        </div>
        <div className="w-1/2 min-w-[400px]">
          <KanbanBoard />
        </div>
      </main>
    </div>
  );
}

export default function App() {
  const ws = useWebSocketProvider();

  return (
    <WebSocketContext.Provider value={ws}>
      <AppContent />
    </WebSocketContext.Provider>
  );
}
```

Note: The `CompliancePanel`, `PanelContainer`, and `useScanResult` imports are removed. Compliance findings will be shown as task output in a future iteration.

**Step 2: Build and verify**

```bash
cd /home/rishav/soul/web && npx vite build
cd /home/rishav/soul && go install ./cmd/soul/
```

**Step 3: Commit**

```bash
git add web/src/App.tsx
git commit -m "feat(planner): wire two-panel layout — chat left, kanban right"
```

---

## Task 8: WebSocket broadcast infrastructure

**Files:**
- Modify: `internal/server/ws.go` — add client registry and broadcast

Currently `ws.go` handles a single WebSocket connection. We need to track all connected clients so `broadcastTaskEvent` can send to all of them.

**Step 1: Add client registry to ws.go**

Add a `clients` map (`sync.Map` or mutex-guarded map) to the Server. On each WebSocket connection, register the client. On disconnect, remove. The `broadcast` method iterates all clients and sends.

```go
// In ws.go — add to Server via a new wsClients field or similar.
// broadcast sends a message to all connected WebSocket clients.
func (s *Server) broadcast(msg WSMessage) {
    // iterate over all connected clients and send msg
}
```

**Step 2: Verify build**

```bash
cd /home/rishav/soul && go build ./cmd/soul/
```

**Step 3: Commit**

```bash
git add internal/server/ws.go
git commit -m "feat(planner): add WebSocket broadcast to all connected clients"
```

---

## Task 9: Build, deploy, and test end-to-end

**Step 1: Build everything**

```bash
cd /home/rishav/soul/web && npx vite build
cd /home/rishav/soul && go install ./cmd/soul/
```

**Step 2: Restart server**

```bash
pkill -f "soul serve" 2>/dev/null; sleep 1
(unset ANTHROPIC_API_KEY; SOUL_HOST=0.0.0.0 /home/rishav/go/bin/soul serve --compliance-bin /home/rishav/.soul/products/compliance-go > /tmp/soul-server.log 2>&1) &
```

**Step 3: Browser test**

Open `http://192.168.0.128:3000/` and verify:
- Two-panel layout: chat left, task manager right
- Kanban board shows 6 columns
- Click "+ New Task" — form appears, create a task
- Task appears in Backlog column
- Click task card — detail overlay opens
- Move task between stages
- Delete task

**Step 4: Test REST API directly**

```bash
# Create task
curl -X POST http://localhost:3000/api/tasks -H 'Content-Type: application/json' -d '{"title":"Test task","priority":2}'

# List tasks
curl http://localhost:3000/api/tasks

# Move task
curl -X POST http://localhost:3000/api/tasks/1/move -H 'Content-Type: application/json' -d '{"stage":"active"}'
```

**Step 5: Commit final state**

```bash
git add -A
git commit -m "feat(planner): end-to-end Kanban board with SQLite, REST, WebSocket"
```

---

## Task 10: Autonomous processor (future — Phase 2)

This task is deferred to a follow-up plan. It includes:
- `internal/planner/processor.go` — background goroutine
- Auto-process toggle in UI
- AI agent integration (create tasks from chat, process via products)
- Brainstorm stage AI planning
- 6-substep pipeline execution

These are documented in the design but not implemented in this first pass. The priority is getting the Kanban board and REST API working end-to-end.
