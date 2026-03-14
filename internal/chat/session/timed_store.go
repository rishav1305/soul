package session

import (
	"database/sql"
	"log"
	"time"

	"github.com/rishav1305/soul-v2/internal/chat/metrics"
)

// TimedStore wraps *Store with timing instrumentation. Every public method
// logs a db.query event with its duration. Queries exceeding the slow
// threshold also log a db.slow event.
type TimedStore struct {
	inner  *Store
	logger *metrics.EventLogger
	slowMs int64
}

// NewTimedStore creates a TimedStore that wraps inner, logs events via logger,
// and flags queries taking longer than slowMs milliseconds as slow.
func NewTimedStore(inner *Store, logger *metrics.EventLogger, slowMs int64) *TimedStore {
	return &TimedStore{inner: inner, logger: logger, slowMs: slowMs}
}

// Inner returns the underlying *Store.
func (ts *TimedStore) Inner() *Store { return ts.inner }

// logQuery records a db.query event and, if the query exceeded the slow
// threshold, a db.slow event.
func (ts *TimedStore) logQuery(method string, start time.Time, sessionID string) {
	dur := time.Since(start).Milliseconds()
	data := map[string]interface{}{
		"method":      method,
		"duration_ms": dur,
	}
	if sessionID != "" {
		data["session_id"] = sessionID
	}
	if err := ts.logger.Log(metrics.EventDBQuery, data); err != nil {
		log.Printf("metrics: log db.query: %v", err)
	}
	if dur > ts.slowMs {
		data["threshold_ms"] = ts.slowMs
		if err := ts.logger.Log(metrics.EventDBSlow, data); err != nil {
			log.Printf("metrics: log db.slow: %v", err)
		}
	}
}

// CreateSession creates a new session and logs timing.
func (ts *TimedStore) CreateSession(title string) (*Session, error) {
	start := time.Now()
	result, err := ts.inner.CreateSession(title)
	sessionID := ""
	if result != nil {
		sessionID = result.ID
	}
	ts.logQuery("CreateSession", start, sessionID)
	return result, err
}

// GetSession retrieves a session by ID and logs timing.
func (ts *TimedStore) GetSession(id string) (*Session, error) {
	start := time.Now()
	result, err := ts.inner.GetSession(id)
	ts.logQuery("GetSession", start, id)
	return result, err
}

// ListSessions returns all sessions and logs timing.
func (ts *TimedStore) ListSessions() ([]*Session, error) {
	start := time.Now()
	result, err := ts.inner.ListSessions()
	ts.logQuery("ListSessions", start, "")
	return result, err
}

// UpdateSessionTitle updates a session's title and logs timing.
func (ts *TimedStore) UpdateSessionTitle(id, title string) (*Session, error) {
	start := time.Now()
	result, err := ts.inner.UpdateSessionTitle(id, title)
	ts.logQuery("UpdateSessionTitle", start, id)
	return result, err
}

// UpdateSessionStatus transitions a session to a new status and logs timing.
func (ts *TimedStore) UpdateSessionStatus(id string, status Status) error {
	start := time.Now()
	err := ts.inner.UpdateSessionStatus(id, status)
	ts.logQuery("UpdateSessionStatus", start, id)
	return err
}

// DeleteSession removes a session and logs timing.
func (ts *TimedStore) DeleteSession(id string) error {
	start := time.Now()
	err := ts.inner.DeleteSession(id)
	ts.logQuery("DeleteSession", start, id)
	return err
}

// AddMessage adds a message to a session and logs timing.
func (ts *TimedStore) AddMessage(sessionID, role, content string) (*Message, error) {
	start := time.Now()
	result, err := ts.inner.AddMessage(sessionID, role, content)
	ts.logQuery("AddMessage", start, sessionID)
	return result, err
}

// AddMessageTx adds a message within an existing transaction and logs timing.
func (ts *TimedStore) AddMessageTx(tx *sql.Tx, sessionID, role, content string) (*Message, error) {
	start := time.Now()
	result, err := ts.inner.AddMessageTx(tx, sessionID, role, content)
	ts.logQuery("AddMessageTx", start, sessionID)
	return result, err
}

// GetMessages returns all messages for a session and logs timing.
func (ts *TimedStore) GetMessages(sessionID string) ([]*Message, error) {
	start := time.Now()
	result, err := ts.inner.GetMessages(sessionID)
	ts.logQuery("GetMessages", start, sessionID)
	return result, err
}

// RunInTransaction executes fn inside a SQL transaction and logs timing.
func (ts *TimedStore) RunInTransaction(fn func(tx *sql.Tx) error) error {
	start := time.Now()
	err := ts.inner.RunInTransaction(fn)
	ts.logQuery("RunInTransaction", start, "")
	return err
}

// ResetUnreadCount resets the unread_count for a session and logs timing.
func (ts *TimedStore) ResetUnreadCount(id string) error {
	start := time.Now()
	err := ts.inner.ResetUnreadCount(id)
	ts.logQuery("ResetUnreadCount", start, id)
	return err
}

// SetLastMessage updates the last_message preview for a session and logs timing.
func (ts *TimedStore) SetLastMessage(id, content string) error {
	start := time.Now()
	err := ts.inner.SetLastMessage(id, content)
	ts.logQuery("SetLastMessage", start, id)
	return err
}

// Close closes the underlying database connection without timing instrumentation.
func (ts *TimedStore) Close() error {
	return ts.inner.Close()
}

// Migrate runs database migrations without timing instrumentation.
func (ts *TimedStore) Migrate() error {
	return ts.inner.Migrate()
}
