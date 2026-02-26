package session

import "time"

// Message represents a single chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Session tracks a conversation with its message history.
type Session struct {
	ID        string    `json:"id"`
	Messages  []Message `json:"messages"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// New creates a fresh Session with the given ID.
func New(id string) *Session {
	now := time.Now()
	return &Session{
		ID:        id,
		Messages:  make([]Message, 0),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// AddMessage appends a message to the session and updates the timestamp.
func (s *Session) AddMessage(role, content string) {
	s.Messages = append(s.Messages, Message{
		Role:    role,
		Content: content,
	})
	s.UpdatedAt = time.Now()
}
