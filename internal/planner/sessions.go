package planner

import (
	"fmt"
	"time"
)

// ChatSession represents a chat session record.
type ChatSession struct {
	ID           int64  `json:"id"`
	Title        string `json:"title"`
	Summary      string `json:"summary"`
	Model        string `json:"model"`
	Status       string `json:"status"`
	MessageCount int    `json:"message_count"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// ChatMessageRecord represents a single chat message stored in the database.
type ChatMessageRecord struct {
	ID        int64  `json:"id"`
	SessionID int64  `json:"session_id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

// CreateSession inserts a new chat session and returns it with the generated ID.
func (s *Store) CreateSession(title string) (ChatSession, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		`INSERT INTO chat_sessions (title, status, created_at, updated_at) VALUES (?, 'idle', ?, ?)`,
		title, now, now,
	)
	if err != nil {
		return ChatSession{}, fmt.Errorf("insert session: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return ChatSession{}, fmt.Errorf("last insert id: %w", err)
	}
	return ChatSession{
		ID:        id,
		Title:     title,
		Status:    "idle",
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// UpdateSessionSummary updates the title, summary, and model for a session.
func (s *Store) UpdateSessionSummary(id int64, title, summary, model string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		`UPDATE chat_sessions SET title = ?, summary = ?, model = ?, updated_at = ? WHERE id = ?`,
		title, summary, model, now, id,
	)
	if err != nil {
		return fmt.Errorf("update session summary: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListSessions returns the most recent sessions, ordered by updated_at DESC.
func (s *Store) ListSessions(limit int) ([]ChatSession, error) {
	rows, err := s.db.Query(
		`SELECT s.id, s.title, s.summary, s.model, s.status, s.created_at, s.updated_at,
		        (SELECT COUNT(*) FROM chat_messages WHERE session_id = s.id) AS message_count
		 FROM chat_sessions s ORDER BY s.updated_at DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []ChatSession
	for rows.Next() {
		var cs ChatSession
		if err := rows.Scan(&cs.ID, &cs.Title, &cs.Summary, &cs.Model, &cs.Status, &cs.CreatedAt, &cs.UpdatedAt, &cs.MessageCount); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		sessions = append(sessions, cs)
	}
	return sessions, rows.Err()
}

// GetSessionMessages returns all messages for a session, ordered by created_at ASC.
func (s *Store) GetSessionMessages(sessionID int64) ([]ChatMessageRecord, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, role, content, created_at FROM chat_messages WHERE session_id = ? ORDER BY created_at ASC`,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("get session messages: %w", err)
	}
	defer rows.Close()

	var msgs []ChatMessageRecord
	for rows.Next() {
		var m ChatMessageRecord
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// InsertMessage inserts a chat message and returns the new row ID.
func (s *Store) InsertMessage(sessionID int64, role, content string) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		`INSERT INTO chat_messages (session_id, role, content, created_at) VALUES (?, ?, ?, ?)`,
		sessionID, role, content, now,
	)
	if err != nil {
		return 0, fmt.Errorf("insert message: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	// Update session timestamp.
	if role == "user" {
		title := content
		if len(title) > 100 {
			title = title[:100]
		}
		_, _ = s.db.Exec(
			`UPDATE chat_sessions SET updated_at = ?, title = CASE WHEN title = '' THEN ? ELSE title END WHERE id = ?`,
			now, title, sessionID,
		)
	} else {
		_, _ = s.db.Exec(
			`UPDATE chat_sessions SET updated_at = ? WHERE id = ?`,
			now, sessionID,
		)
	}
	return id, nil
}

// UpdateMessageContent updates the content of an existing message by ID.
func (s *Store) UpdateMessageContent(id int64, content string) error {
	_, err := s.db.Exec(`UPDATE chat_messages SET content = ? WHERE id = ?`, content, id)
	if err != nil {
		return fmt.Errorf("update message content: %w", err)
	}
	return nil
}

// AddMessage inserts a chat message and updates the session's updated_at timestamp.
// If the session title is empty and the message role is "user", the title is set
// from the message content (truncated to 100 characters).
func (s *Store) AddMessage(sessionID int64, role, content string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := s.db.Exec(
		`INSERT INTO chat_messages (session_id, role, content, created_at) VALUES (?, ?, ?, ?)`,
		sessionID, role, content, now,
	)
	if err != nil {
		return fmt.Errorf("insert message: %w", err)
	}

	// Update session updated_at; also set title from first user message if empty.
	if role == "user" {
		title := content
		if len(title) > 100 {
			title = title[:100]
		}
		_, err = s.db.Exec(
			`UPDATE chat_sessions SET updated_at = ?, title = CASE WHEN title = '' THEN ? ELSE title END WHERE id = ?`,
			now, title, sessionID,
		)
	} else {
		_, err = s.db.Exec(
			`UPDATE chat_sessions SET updated_at = ? WHERE id = ?`,
			now, sessionID,
		)
	}
	if err != nil {
		return fmt.Errorf("update session timestamp: %w", err)
	}
	return nil
}

// UpdateSessionStatus updates the status field of a chat session.
// Valid statuses: "idle", "running", "completed_unread".
func (s *Store) UpdateSessionStatus(id int64, status string) error {
	switch status {
	case "idle", "running", "completed_unread":
		// valid
	default:
		return fmt.Errorf("invalid session status: %q", status)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		`UPDATE chat_sessions SET status = ?, updated_at = ? WHERE id = ?`,
		status, now, id,
	)
	if err != nil {
		return fmt.Errorf("update session status: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkSessionRead transitions a session from "completed_unread" → "idle".
// Safe to call on already-idle sessions (no-op).
func (s *Store) MarkSessionRead(id int64) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`UPDATE chat_sessions SET status = 'idle', updated_at = ? WHERE id = ? AND status = 'completed_unread'`,
		now, id,
	)
	if err != nil {
		return fmt.Errorf("mark session read: %w", err)
	}
	return nil
}
