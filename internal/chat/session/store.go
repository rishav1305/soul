// Package session provides SQLite-backed session and message storage
// for the Soul v2 chat interface.
package session

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver
)

// Status represents the lifecycle state of a session.
type Status string

const (
	StatusIdle            Status = "idle"
	StatusRunning         Status = "running"
	StatusCompleted       Status = "completed"
	StatusCompletedUnread Status = "completed_unread"
)

// validTransitions defines allowed state transitions.
var validTransitions = map[Status][]Status{
	StatusIdle:            {StatusRunning},
	StatusRunning:         {StatusCompleted, StatusCompletedUnread},
	StatusCompletedUnread: {StatusIdle},
	StatusCompleted:       {StatusIdle},
}

// Valid returns true if s is a recognized status value.
func (s Status) Valid() bool {
	switch s {
	case StatusIdle, StatusRunning, StatusCompleted, StatusCompletedUnread:
		return true
	}
	return false
}

// String returns the string representation of the status.
func (s Status) String() string {
	return string(s)
}

// CanTransitionTo returns true if transitioning from s to next is allowed.
func (s Status) CanTransitionTo(next Status) bool {
	for _, allowed := range validTransitions[s] {
		if allowed == next {
			return true
		}
	}
	return false
}

// Session represents a chat session.
type Session struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Status       Status    `json:"status"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	MessageCount int       `json:"messageCount"`
	LastMessage  string    `json:"lastMessage"`
	UnreadCount  int       `json:"unreadCount"`
	Product      string    `json:"product"`
}

// Message represents a single message within a session.
type Message struct {
	ID        string    `json:"id"`
	SessionID string    `json:"sessionId"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

// Store provides CRUD operations for sessions and messages backed by SQLite.
type Store struct {
	db     *sql.DB
	dbPath string
}

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

var validRoles = map[string]bool{
	"user":        true,
	"assistant":   true,
	"tool_use":    true,
	"tool_result": true,
}

// newUUID generates a version 4 UUID using crypto/rand.
func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Open opens (or creates) a SQLite database at the given path, enables WAL mode
// and foreign keys, runs migrations, and returns a Store.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("session: open database: %w", err)
	}

	// Enable WAL mode for concurrent reads.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("session: enable WAL: %w", err)
	}

	// Enable foreign key enforcement.
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("session: enable foreign keys: %w", err)
	}

	// Serialize all operations through a single connection.
	// SQLite is single-writer; this avoids BUSY contention under concurrent writes.
	db.SetMaxOpenConns(1)

	s := &Store{db: db, dbPath: path}

	if err := s.Migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("session: migrate: %w", err)
	}

	// Set busy timeout for concurrent access.
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("session: set busy timeout: %w", err)
	}

	return s, nil
}

// Migrate creates the sessions and messages tables if they don't exist.
func (s *Store) Migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL DEFAULT 'New Session',
    status TEXT NOT NULL DEFAULT 'idle',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    message_count INTEGER NOT NULL DEFAULT 0,
    product TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS messages (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_messages_session_created ON messages(session_id, created_at);
`
	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("session: execute schema: %w", err)
	}

	for _, alt := range []string{
		"ALTER TABLE sessions ADD COLUMN last_message TEXT DEFAULT ''",
		"ALTER TABLE sessions ADD COLUMN unread_count INTEGER DEFAULT 0",
		"ALTER TABLE sessions ADD COLUMN product TEXT NOT NULL DEFAULT ''",
	} {
		if _, err := s.db.Exec(alt); err != nil {
			if !strings.Contains(err.Error(), "duplicate column") {
				return fmt.Errorf("session: migrate column: %w", err)
			}
		}
	}

	// Backfill: set last_message from most recent message per session.
	_, _ = s.db.Exec(`
		UPDATE sessions SET last_message = (
			SELECT CASE
				WHEN length(m.content) > 100 THEN substr(m.content, 1, 100) || '...'
				ELSE m.content
			END
			FROM messages m
			WHERE m.session_id = sessions.id
			ORDER BY m.created_at DESC
			LIMIT 1
		)
		WHERE last_message = '' AND EXISTS (
			SELECT 1 FROM messages WHERE session_id = sessions.id
		)
	`)

	// Backfill: set title from first user message for untitled sessions.
	_, _ = s.db.Exec(`
		UPDATE sessions SET title = (
			SELECT CASE
				WHEN length(m.content) > 50 THEN substr(m.content, 1, 50) || '...'
				ELSE m.content
			END
			FROM messages m
			WHERE m.session_id = sessions.id AND m.role = 'user'
			ORDER BY m.created_at ASC
			LIMIT 1
		)
		WHERE title = 'New Session' AND EXISTS (
			SELECT 1 FROM messages WHERE session_id = sessions.id AND role = 'user'
		)
	`)

	return nil
}

// CreateSession creates a new session with the given title. If title is empty,
// it defaults to "New Session".
func (s *Store) CreateSession(title string) (*Session, error) {
	if title == "" {
		title = "New Session"
	}

	now := time.Now().UTC()
	sess := &Session{
		ID:           newUUID(),
		Title:        title,
		Status:       StatusIdle,
		CreatedAt:    now,
		UpdatedAt:    now,
		MessageCount: 0,
	}

	_, err := s.db.Exec(
		"INSERT INTO sessions (id, title, status, created_at, updated_at, message_count, last_message, unread_count, product) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		sess.ID, sess.Title, string(sess.Status),
		sess.CreatedAt.Format(time.RFC3339Nano),
		sess.UpdatedAt.Format(time.RFC3339Nano),
		sess.MessageCount,
		sess.LastMessage,
		sess.UnreadCount,
		sess.Product,
	)
	if err != nil {
		return nil, fmt.Errorf("session: create: %w", err)
	}
	return sess, nil
}

// GetSession retrieves a session by ID. Returns an error if the ID format is
// invalid or the session doesn't exist.
func (s *Store) GetSession(id string) (*Session, error) {
	if !uuidRe.MatchString(id) {
		return nil, fmt.Errorf("session: invalid UUID format: %q", id)
	}

	sess := &Session{}
	var createdAt, updatedAt, status string
	err := s.db.QueryRow(
		"SELECT id, title, status, created_at, updated_at, message_count, last_message, unread_count, product FROM sessions WHERE id = ?",
		id,
	).Scan(&sess.ID, &sess.Title, &status, &createdAt, &updatedAt, &sess.MessageCount, &sess.LastMessage, &sess.UnreadCount, &sess.Product)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("session: not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("session: get: %w", err)
	}

	sess.Status = Status(status)
	sess.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("session: parse created_at: %w", err)
	}
	sess.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("session: parse updated_at: %w", err)
	}

	return sess, nil
}

// ListSessions returns all sessions ordered by updated_at descending.
func (s *Store) ListSessions() ([]*Session, error) {
	rows, err := s.db.Query(
		"SELECT id, title, status, created_at, updated_at, message_count, last_message, unread_count, product FROM sessions ORDER BY updated_at DESC",
	)
	if err != nil {
		return nil, fmt.Errorf("session: list: %w", err)
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		sess := &Session{}
		var createdAt, updatedAt, status string
		if err := rows.Scan(&sess.ID, &sess.Title, &status, &createdAt, &updatedAt, &sess.MessageCount, &sess.LastMessage, &sess.UnreadCount, &sess.Product); err != nil {
			return nil, fmt.Errorf("session: scan row: %w", err)
		}
		sess.Status = Status(status)
		sess.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, fmt.Errorf("session: parse created_at: %w", err)
		}
		sess.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
		if err != nil {
			return nil, fmt.Errorf("session: parse updated_at: %w", err)
		}
		sessions = append(sessions, sess)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("session: iterate rows: %w", err)
	}
	return sessions, nil
}

// UpdateSessionTitle updates the title of a session and returns the updated session.
func (s *Store) UpdateSessionTitle(id, title string) (*Session, error) {
	if !uuidRe.MatchString(id) {
		return nil, fmt.Errorf("session: invalid UUID format: %q", id)
	}

	now := time.Now().UTC()
	result, err := s.db.Exec(
		"UPDATE sessions SET title = ?, updated_at = ? WHERE id = ?",
		title, now.Format(time.RFC3339Nano), id,
	)
	if err != nil {
		return nil, fmt.Errorf("session: update title: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("session: rows affected: %w", err)
	}
	if affected == 0 {
		return nil, fmt.Errorf("session: not found: %s", id)
	}

	return s.GetSession(id)
}

// UpdateSessionStatus transitions a session to a new status atomically.
// The read-check-update is wrapped in a transaction to prevent race conditions.
func (s *Store) UpdateSessionStatus(id string, status Status) error {
	if !uuidRe.MatchString(id) {
		return fmt.Errorf("session: invalid UUID format: %q", id)
	}
	if !status.Valid() {
		return fmt.Errorf("session: invalid status: %q", status)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("session: begin transaction: %w", err)
	}
	defer tx.Rollback()

	var currentStatus string
	err = tx.QueryRow("SELECT status FROM sessions WHERE id = ?", id).Scan(&currentStatus)
	if err == sql.ErrNoRows {
		return fmt.Errorf("session: not found: %s", id)
	}
	if err != nil {
		return fmt.Errorf("session: get status: %w", err)
	}

	current := Status(currentStatus)
	if !current.CanTransitionTo(status) {
		return fmt.Errorf("session: invalid transition from %q to %q", current, status)
	}

	now := time.Now().UTC()
	_, err = tx.Exec(
		"UPDATE sessions SET status = ?, updated_at = ? WHERE id = ?",
		string(status), now.Format(time.RFC3339Nano), id,
	)
	if err != nil {
		return fmt.Errorf("session: update status: %w", err)
	}

	return tx.Commit()
}

// DeleteSession removes a session and its messages (via CASCADE).
func (s *Store) DeleteSession(id string) error {
	if !uuidRe.MatchString(id) {
		return fmt.Errorf("session: invalid UUID format: %q", id)
	}

	result, err := s.db.Exec("DELETE FROM sessions WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("session: delete: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("session: rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("session: not found: %s", id)
	}
	return nil
}

// AddMessage adds a message to a session. Returns an error if the role is
// invalid or the session doesn't exist.
func (s *Store) AddMessage(sessionID, role, content string) (*Message, error) {
	if !validRoles[role] {
		return nil, fmt.Errorf("session: invalid role: %q", role)
	}
	if !uuidRe.MatchString(sessionID) {
		return nil, fmt.Errorf("session: invalid UUID format: %q", sessionID)
	}

	// Verify session exists.
	if _, err := s.GetSession(sessionID); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	msg := &Message{
		ID:        newUUID(),
		SessionID: sessionID,
		Role:      role,
		Content:   content,
		CreatedAt: now,
	}

	_, err := s.db.Exec(
		"INSERT INTO messages (id, session_id, role, content, created_at) VALUES (?, ?, ?, ?, ?)",
		msg.ID, msg.SessionID, msg.Role, msg.Content, msg.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, fmt.Errorf("session: add message: %w", err)
	}

	preview := content
	if len(preview) > 100 {
		preview = preview[:100]
		if i := strings.LastIndex(preview, " "); i > 50 {
			preview = preview[:i]
		}
		preview += "..."
	}

	_, err = s.db.Exec(
		"UPDATE sessions SET message_count = message_count + 1, unread_count = unread_count + 1, last_message = ?, updated_at = ? WHERE id = ?",
		preview, now.Format(time.RFC3339Nano), sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("session: update message count: %w", err)
	}

	return msg, nil
}

// GetMessages returns all messages for a session, ordered by created_at ascending.
func (s *Store) GetMessages(sessionID string) ([]*Message, error) {
	if !uuidRe.MatchString(sessionID) {
		return nil, fmt.Errorf("session: invalid UUID format: %q", sessionID)
	}

	rows, err := s.db.Query(
		"SELECT id, session_id, role, content, created_at FROM messages WHERE session_id = ? ORDER BY created_at ASC",
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("session: get messages: %w", err)
	}
	defer rows.Close()

	var messages []*Message
	for rows.Next() {
		msg := &Message{}
		var createdAt string
		if err := rows.Scan(&msg.ID, &msg.SessionID, &msg.Role, &msg.Content, &createdAt); err != nil {
			return nil, fmt.Errorf("session: scan message: %w", err)
		}
		msg.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, fmt.Errorf("session: parse message created_at: %w", err)
		}
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("session: iterate messages: %w", err)
	}
	return messages, nil
}

// RunInTransaction executes fn inside a SQL transaction. If fn returns an error,
// the transaction is rolled back; otherwise it is committed.
func (s *Store) RunInTransaction(fn func(tx *sql.Tx) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("session: begin transaction: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// AddMessageTx adds a message within an existing transaction.
func (s *Store) AddMessageTx(tx *sql.Tx, sessionID, role, content string) (*Message, error) {
	if !validRoles[role] {
		return nil, fmt.Errorf("session: invalid role: %q", role)
	}
	if !uuidRe.MatchString(sessionID) {
		return nil, fmt.Errorf("session: invalid UUID format: %q", sessionID)
	}

	now := time.Now().UTC()
	msg := &Message{
		ID:        newUUID(),
		SessionID: sessionID,
		Role:      role,
		Content:   content,
		CreatedAt: now,
	}

	_, err := tx.Exec(
		"INSERT INTO messages (id, session_id, role, content, created_at) VALUES (?, ?, ?, ?, ?)",
		msg.ID, msg.SessionID, msg.Role, msg.Content, msg.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, fmt.Errorf("session: add message: %w", err)
	}

	preview := content
	if len(preview) > 100 {
		preview = preview[:100]
		if i := strings.LastIndex(preview, " "); i > 50 {
			preview = preview[:i]
		}
		preview += "..."
	}

	_, err = tx.Exec(
		"UPDATE sessions SET message_count = message_count + 1, unread_count = unread_count + 1, last_message = ?, updated_at = ? WHERE id = ?",
		preview, now.Format(time.RFC3339Nano), sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("session: update message count: %w", err)
	}

	return msg, nil
}

// ResetUnreadCount resets the unread_count for a session to 0.
func (s *Store) ResetUnreadCount(id string) error {
	if !uuidRe.MatchString(id) {
		return fmt.Errorf("session: invalid UUID format: %q", id)
	}
	_, err := s.db.Exec("UPDATE sessions SET unread_count = 0 WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("session: reset unread: %w", err)
	}
	return nil
}

// SetLastMessage updates the last_message preview for a session.
func (s *Store) SetLastMessage(id, content string) error {
	if !uuidRe.MatchString(id) {
		return fmt.Errorf("session: invalid UUID format: %q", id)
	}
	preview := content
	if len(preview) > 100 {
		preview = preview[:100]
		if i := strings.LastIndex(preview, " "); i > 50 {
			preview = preview[:i]
		}
		preview += "..."
	}
	_, err := s.db.Exec("UPDATE sessions SET last_message = ? WHERE id = ?", preview, id)
	if err != nil {
		return fmt.Errorf("session: set last message: %w", err)
	}
	return nil
}

// SetProduct sets the product for a session.
// Valid products are: "" (none), "tasks", "tutor", "projects", "observe".
func (s *Store) SetProduct(sessionID, product string) error {
	valid := map[string]bool{"": true, "tasks": true, "tutor": true, "projects": true, "observe": true}
	if !valid[product] {
		return fmt.Errorf("invalid product: %q", product)
	}
	now := time.Now().UTC()
	result, err := s.db.Exec(
		`UPDATE sessions SET product = ?, updated_at = ? WHERE id = ?`,
		product, now.Format(time.RFC3339Nano), sessionID,
	)
	if err != nil {
		return fmt.Errorf("set product: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	return nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}
