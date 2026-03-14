package events

// Logger is the interface for structured event logging.
// Both chat and tasks servers implement this with their own storage.
type Logger interface {
	Log(eventType string, data map[string]interface{}) error
}

// NopLogger is a no-op Logger for testing or when logging is disabled.
type NopLogger struct{}

func (NopLogger) Log(string, map[string]interface{}) error { return nil }

// Auth-related event type constants (used by pkg/auth).
const (
	EventOAuthRefresh = "oauth.refresh"
	EventOAuthReload  = "oauth.reload"
)
