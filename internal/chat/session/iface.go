package session

import "database/sql"

// StoreInterface defines the session store operations used by the application.
// Both Store and TimedStore implement this interface.
type StoreInterface interface {
	CreateSession(title string) (*Session, error)
	GetSession(id string) (*Session, error)
	ListSessions() ([]*Session, error)
	UpdateSessionTitle(id, title string) (*Session, error)
	UpdateSessionStatus(id string, status Status) error
	DeleteSession(id string) error
	AddMessage(sessionID, role, content string) (*Message, error)
	AddMessageTx(tx *sql.Tx, sessionID, role, content string) (*Message, error)
	GetMessages(sessionID string) ([]*Message, error)
	RunInTransaction(fn func(tx *sql.Tx) error) error
	ResetUnreadCount(id string) error
	SetLastMessage(id, content string) error
	Close() error
}
