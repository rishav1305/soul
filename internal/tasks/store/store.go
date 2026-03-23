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
	"brainstorm": true,
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
	Substep     string    `json:"substep,omitempty"`
	Metadata    string    `json:"metadata,omitempty"`
	Seq         int64     `json:"seq"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Comment represents a task comment.
type Comment struct {
	ID        int64  `json:"id"`
	TaskID    int64  `json:"taskId"`
	Author    string `json:"author"`
	Type      string `json:"type"`
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt"`
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
	db       *sql.DB
	dbPath   string
	OnChange func(event string, payload any)
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
	CREATE TABLE IF NOT EXISTS task_dependencies (
		task_id INTEGER REFERENCES tasks(id) ON DELETE CASCADE,
		depends_on INTEGER REFERENCES tasks(id) ON DELETE CASCADE,
		PRIMARY KEY (task_id, depends_on)
	);
	CREATE INDEX IF NOT EXISTS idx_tasks_stage ON tasks(stage);
	CREATE INDEX IF NOT EXISTS idx_tasks_product ON tasks(product);
	CREATE INDEX IF NOT EXISTS idx_task_activity_task_id ON task_activity(task_id);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("tasks: migrate: %w", err)
	}

	// Add substep column if it doesn't exist (ignore duplicate column error).
	_, err := s.db.Exec("ALTER TABLE tasks ADD COLUMN substep TEXT NOT NULL DEFAULT ''")
	if err != nil && !strings.Contains(err.Error(), "duplicate column") {
		return fmt.Errorf("tasks: migrate substep column: %w", err)
	}

	// Add task_comments table.
	const commentsSchema = `
	CREATE TABLE IF NOT EXISTS task_comments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id INTEGER REFERENCES tasks(id) ON DELETE CASCADE,
		author TEXT NOT NULL,
		type TEXT NOT NULL,
		body TEXT NOT NULL,
		created_at TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_task_comments_task_id ON task_comments(task_id);
	`
	if _, err := s.db.Exec(commentsSchema); err != nil {
		return fmt.Errorf("tasks: migrate comments: %w", err)
	}

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

	return nil
}

// nextSeqTx atomically increments and returns the global sequence counter within a transaction.
func (s *Store) nextSeqTx(tx *sql.Tx) (int64, error) {
	var seq int64
	err := tx.QueryRow("UPDATE sync_meta SET value = value + 1 WHERE key = 'seq' RETURNING value").Scan(&seq)
	if err != nil {
		return 0, fmt.Errorf("tasks: next seq: %w", err)
	}
	return seq, nil
}

// fireOnChange calls the OnChange hook if set.
func (s *Store) fireOnChange(event string, payload any) {
	if s.OnChange != nil {
		s.OnChange(event, payload)
	}
}

// Create inserts a new task with the given title, description, and product.
func (s *Store) Create(title, description, product string) (*Task, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("tasks: create begin tx: %w", err)
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

	s.fireOnChange("task.created", task)
	return task, nil
}

// Get retrieves a task by ID.
func (s *Store) Get(id int64) (*Task, error) {
	var t Task
	err := s.db.QueryRow(
		"SELECT id, title, description, stage, workflow, product, substep, metadata, seq, created_at, updated_at FROM tasks WHERE id = ?",
		id,
	).Scan(&t.ID, &t.Title, &t.Description, &t.Stage, &t.Workflow, &t.Product, &t.Substep, &t.Metadata, &t.Seq, &t.CreatedAt, &t.UpdatedAt)
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
	query := "SELECT id, title, description, stage, workflow, product, substep, metadata, seq, created_at, updated_at FROM tasks"
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
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Stage, &t.Workflow, &t.Product, &t.Substep, &t.Metadata, &t.Seq, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("tasks: scan: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// Update modifies task fields. Allowed keys: title, description, stage, workflow, product, substep, metadata.
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
		return nil, fmt.Errorf("tasks: update begin tx: %w", err)
	}
	defer tx.Rollback()

	seq, err := s.nextSeqTx(tx)
	if err != nil {
		return nil, err
	}

	setClauses = append(setClauses, "seq = ?")
	args = append(args, seq)
	setClauses = append(setClauses, "updated_at = CURRENT_TIMESTAMP")
	args = append(args, id)

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

	// Classify event by priority: stage_changed > substep_changed > updated.
	var eventType string
	if _, ok := fields["stage"]; ok {
		eventType = "task.stage_changed"
	} else if _, ok := fields["substep"]; ok {
		eventType = "task.substep_changed"
	} else {
		eventType = "task.updated"
	}

	s.fireOnChange(eventType, task)
	return task, nil
}

// Delete removes a task and its activity (cascading foreign key).
func (s *Store) Delete(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("tasks: delete begin tx: %w", err)
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
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("tasks: not found: %d", id)
	}

	if _, err := tx.Exec(
		"INSERT INTO task_tombstones (id, seq) VALUES (?, ?)",
		id, seq,
	); err != nil {
		return fmt.Errorf("tasks: insert tombstone: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("tasks: delete commit: %w", err)
	}

	s.fireOnChange("task.deleted", TaskDeleted{ID: id})
	return nil
}

// AddActivity appends an activity entry for a task and returns the created Activity.
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
	actID, _ := res.LastInsertId()

	var act Activity
	err = s.db.QueryRow(
		"SELECT id, task_id, event_type, data, created_at FROM task_activity WHERE id = ?",
		actID,
	).Scan(&act.ID, &act.TaskID, &act.EventType, &act.Data, &act.CreatedAt)
	if err != nil {
		return Activity{}, fmt.Errorf("tasks: get activity: %w", err)
	}

	s.fireOnChange("task.activity", TaskActivity{TaskID: taskID, Activity: act})
	return act, nil
}

// InsertComment adds a comment to a task and returns the created Comment.
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

	s.fireOnChange("task.comment", TaskComment{TaskID: taskID, Comment: cmt})
	return cmt, nil
}

// GetComments returns all comments for a task, ordered by created_at ASC.
func (s *Store) GetComments(taskID int64) ([]Comment, error) {
	rows, err := s.db.Query(
		"SELECT id, task_id, author, type, body, created_at FROM task_comments WHERE task_id = ? ORDER BY created_at ASC",
		taskID,
	)
	if err != nil {
		return nil, fmt.Errorf("tasks: get comments: %w", err)
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

// CommentsAfter returns user comments with id > lastID, ordered by id ASC.
func (s *Store) CommentsAfter(lastID int64) ([]Comment, error) {
	rows, err := s.db.Query(
		"SELECT id, task_id, author, type, body, created_at FROM task_comments WHERE id > ? AND author = 'user' ORDER BY id ASC",
		lastID,
	)
	if err != nil {
		return nil, fmt.Errorf("tasks: comments after: %w", err)
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

// AddDependency records that taskID depends on dependsOn. Idempotent.
func (s *Store) AddDependency(taskID, dependsOn int64) error {
	_, err := s.db.Exec(
		"INSERT OR IGNORE INTO task_dependencies (task_id, depends_on) VALUES (?, ?)",
		taskID, dependsOn,
	)
	if err != nil {
		return fmt.Errorf("tasks: add dependency: %w", err)
	}
	return nil
}

// RemoveDependency removes a dependency between two tasks.
func (s *Store) RemoveDependency(taskID, dependsOn int64) error {
	_, err := s.db.Exec(
		"DELETE FROM task_dependencies WHERE task_id = ? AND depends_on = ?",
		taskID, dependsOn,
	)
	if err != nil {
		return fmt.Errorf("tasks: remove dependency: %w", err)
	}
	return nil
}

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

// NextReady returns the oldest backlog task whose dependencies are all done.
func (s *Store) NextReady() (*Task, error) {
	var t Task
	err := s.db.QueryRow(`
		SELECT id, title, description, stage, workflow, product, substep, metadata, seq, created_at, updated_at
		FROM tasks t
		WHERE t.stage = 'backlog'
		AND NOT EXISTS (
			SELECT 1 FROM task_dependencies td
			JOIN tasks dep ON dep.id = td.depends_on
			WHERE td.task_id = t.id AND dep.stage != 'done'
		)
		ORDER BY t.created_at ASC
		LIMIT 1
	`).Scan(&t.ID, &t.Title, &t.Description, &t.Stage, &t.Workflow, &t.Product, &t.Substep, &t.Metadata, &t.Seq, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tasks: no ready task found")
	}
	if err != nil {
		return nil, fmt.Errorf("tasks: next ready: %w", err)
	}
	return &t, nil
}
