package planner

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// Store is a SQLite-backed task store.
type Store struct {
	db *sql.DB
}

// ErrNotFound is returned when a requested task does not exist.
var ErrNotFound = errors.New("task not found")

const schema = `
CREATE TABLE IF NOT EXISTS tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    description TEXT DEFAULT '',
    acceptance TEXT DEFAULT '',
    stage TEXT DEFAULT 'backlog',
    substep TEXT DEFAULT '',
    priority INTEGER DEFAULT 0,
    source TEXT DEFAULT 'manual',
    blocker TEXT DEFAULT '',
    plan TEXT DEFAULT '',
    output TEXT DEFAULT '',
    error TEXT DEFAULT '',
    agent_id TEXT DEFAULT '',
    product TEXT DEFAULT '',
    parent_id INTEGER,
    metadata TEXT DEFAULT '',
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER DEFAULT 3,
    created_at TEXT NOT NULL,
    started_at TEXT DEFAULT '',
    completed_at TEXT DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_tasks_stage ON tasks(stage);
CREATE INDEX IF NOT EXISTS idx_tasks_priority ON tasks(priority DESC);
CREATE TABLE IF NOT EXISTS task_dependencies (
    task_id INTEGER REFERENCES tasks(id) ON DELETE CASCADE,
    depends_on INTEGER REFERENCES tasks(id) ON DELETE CASCADE,
    PRIMARY KEY (task_id, depends_on)
);
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
`

// OpenStore opens (or creates) a SQLite database at dbPath, applies the schema,
// and enables WAL mode and foreign keys.
func OpenStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// Enable WAL mode and foreign keys.
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("exec %q: %w", pragma, err)
		}
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// Create inserts a new task and returns its auto-generated ID.
func (s *Store) Create(t Task) (int64, error) {
	var parentID sql.NullInt64
	if t.ParentID != nil {
		parentID = sql.NullInt64{Int64: *t.ParentID, Valid: true}
	}

	res, err := s.db.Exec(`
		INSERT INTO tasks (
			title, description, acceptance,
			stage, substep, priority, source,
			blocker, plan, output, error,
			agent_id, product, parent_id, metadata,
			retry_count, max_retries,
			created_at, started_at, completed_at
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		t.Title, t.Description, t.Acceptance,
		string(t.Stage), string(t.Substep), t.Priority, t.Source,
		t.Blocker, t.Plan, t.Output, t.Error,
		t.AgentID, t.Product, parentID, t.Metadata,
		t.RetryCount, t.MaxRetries,
		t.CreatedAt, t.StartedAt, t.CompletedAt,
	)
	if err != nil {
		return 0, fmt.Errorf("insert task: %w", err)
	}
	return res.LastInsertId()
}

// scanTask scans a single row into a Task.
func scanTask(row interface{ Scan(dest ...any) error }) (Task, error) {
	var t Task
	var parentID sql.NullInt64
	var stage, substep string

	err := row.Scan(
		&t.ID,
		&t.Title, &t.Description, &t.Acceptance,
		&stage, &substep,
		&t.Priority, &t.Source,
		&t.Blocker, &t.Plan, &t.Output, &t.Error,
		&t.AgentID, &t.Product, &parentID, &t.Metadata,
		&t.RetryCount, &t.MaxRetries,
		&t.CreatedAt, &t.StartedAt, &t.CompletedAt,
	)
	if err != nil {
		return Task{}, err
	}

	t.Stage = Stage(stage)
	t.Substep = Substep(substep)
	if parentID.Valid {
		pid := parentID.Int64
		t.ParentID = &pid
	}
	return t, nil
}

const selectCols = `id, title, description, acceptance, stage, substep,
	priority, source, blocker, plan, output, error,
	agent_id, product, parent_id, metadata,
	retry_count, max_retries,
	created_at, started_at, completed_at`

// Get fetches a task by its ID.
func (s *Store) Get(id int64) (Task, error) {
	row := s.db.QueryRow("SELECT "+selectCols+" FROM tasks WHERE id = ?", id)
	t, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, fmt.Errorf("get task %d: %w", id, err)
	}
	return t, nil
}

// List returns tasks matching the given filter, ordered by priority DESC then
// created_at ASC.
func (s *Store) List(f TaskFilter) ([]Task, error) {
	query := "SELECT " + selectCols + " FROM tasks"
	var clauses []string
	var args []any

	if f.Stage != "" {
		clauses = append(clauses, "stage = ?")
		args = append(args, string(f.Stage))
	}
	if f.Product != "" {
		clauses = append(clauses, "product = ?")
		args = append(args, f.Product)
	}

	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY priority DESC, created_at ASC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// Update applies a partial update to the task identified by id.
// Only non-nil fields in TaskUpdate are modified.
func (s *Store) Update(id int64, u TaskUpdate) error {
	var setClauses []string
	var args []any

	if u.Title != nil {
		setClauses = append(setClauses, "title = ?")
		args = append(args, *u.Title)
	}
	if u.Description != nil {
		setClauses = append(setClauses, "description = ?")
		args = append(args, *u.Description)
	}
	if u.Acceptance != nil {
		setClauses = append(setClauses, "acceptance = ?")
		args = append(args, *u.Acceptance)
	}
	if u.Stage != nil {
		setClauses = append(setClauses, "stage = ?")
		args = append(args, string(*u.Stage))
	}
	if u.Substep != nil {
		setClauses = append(setClauses, "substep = ?")
		args = append(args, string(*u.Substep))
	}
	if u.Priority != nil {
		setClauses = append(setClauses, "priority = ?")
		args = append(args, *u.Priority)
	}
	if u.Source != nil {
		setClauses = append(setClauses, "source = ?")
		args = append(args, *u.Source)
	}
	if u.Blocker != nil {
		setClauses = append(setClauses, "blocker = ?")
		args = append(args, *u.Blocker)
	}
	if u.Plan != nil {
		setClauses = append(setClauses, "plan = ?")
		args = append(args, *u.Plan)
	}
	if u.Output != nil {
		setClauses = append(setClauses, "output = ?")
		args = append(args, *u.Output)
	}
	if u.Error != nil {
		setClauses = append(setClauses, "error = ?")
		args = append(args, *u.Error)
	}
	if u.AgentID != nil {
		setClauses = append(setClauses, "agent_id = ?")
		args = append(args, *u.AgentID)
	}
	if u.Product != nil {
		setClauses = append(setClauses, "product = ?")
		args = append(args, *u.Product)
	}
	if u.ParentID != nil {
		setClauses = append(setClauses, "parent_id = ?")
		args = append(args, *u.ParentID)
	}
	if u.Metadata != nil {
		setClauses = append(setClauses, "metadata = ?")
		args = append(args, *u.Metadata)
	}
	if u.RetryCount != nil {
		setClauses = append(setClauses, "retry_count = ?")
		args = append(args, *u.RetryCount)
	}
	if u.MaxRetries != nil {
		setClauses = append(setClauses, "max_retries = ?")
		args = append(args, *u.MaxRetries)
	}
	if u.StartedAt != nil {
		setClauses = append(setClauses, "started_at = ?")
		args = append(args, *u.StartedAt)
	}
	if u.CompletedAt != nil {
		setClauses = append(setClauses, "completed_at = ?")
		args = append(args, *u.CompletedAt)
	}

	if len(setClauses) == 0 {
		return nil // nothing to update
	}

	args = append(args, id)
	query := "UPDATE tasks SET " + strings.Join(setClauses, ", ") + " WHERE id = ?"

	res, err := s.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("update task %d: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a task by its ID.
func (s *Store) Delete(id int64) error {
	res, err := s.db.Exec("DELETE FROM tasks WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete task %d: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// NextReady returns the highest-priority backlog task that has no unresolved
// dependencies (i.e. all dependencies are in the 'done' stage).
// Returns ErrNotFound if no eligible task exists.
func (s *Store) NextReady() (Task, error) {
	row := s.db.QueryRow(`
		SELECT `+selectCols+`
		FROM tasks t
		WHERE t.stage = 'backlog'
		  AND NOT EXISTS (
		      SELECT 1
		      FROM task_dependencies td
		      JOIN tasks dep ON dep.id = td.depends_on
		      WHERE td.task_id = t.id
		        AND dep.stage != 'done'
		  )
		ORDER BY t.priority DESC, t.created_at ASC
		LIMIT 1`)

	t, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, fmt.Errorf("next ready: %w", err)
	}
	return t, nil
}

// AddDependency records that taskID depends on dependsOn.
func (s *Store) AddDependency(taskID, dependsOn int64) error {
	_, err := s.db.Exec(
		"INSERT INTO task_dependencies (task_id, depends_on) VALUES (?, ?)",
		taskID, dependsOn,
	)
	if err != nil {
		return fmt.Errorf("add dependency %d→%d: %w", taskID, dependsOn, err)
	}
	return nil
}

// RemoveDependency deletes a dependency record.
func (s *Store) RemoveDependency(taskID, dependsOn int64) error {
	_, err := s.db.Exec(
		"DELETE FROM task_dependencies WHERE task_id = ? AND depends_on = ?",
		taskID, dependsOn,
	)
	if err != nil {
		return fmt.Errorf("remove dependency %d→%d: %w", taskID, dependsOn, err)
	}
	return nil
}
