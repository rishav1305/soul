package stream

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Request represents a Claude API request.
type Request struct {
	Model     string    `json:"model,omitempty"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system,omitempty"`
	Messages  []Message `json:"messages"`
	Tools     []Tool    `json:"tools,omitempty"`
	Stream    bool      `json:"stream"`
}

// Validate checks required fields and message alternation.
func (r *Request) Validate() error {
	if r.MaxTokens <= 0 {
		return errors.New("max_tokens must be greater than 0")
	}
	if len(r.Messages) == 0 {
		return errors.New("messages must not be empty")
	}

	// First message must be from user.
	if r.Messages[0].Role != "user" {
		return fmt.Errorf("first message must have role \"user\", got %q", r.Messages[0].Role)
	}

	// Validate roles and alternation.
	for i, m := range r.Messages {
		if m.Role != "user" && m.Role != "assistant" {
			return fmt.Errorf("message %d has invalid role %q (must be \"user\" or \"assistant\")", i, m.Role)
		}
		if i > 0 && m.Role == r.Messages[i-1].Role {
			return fmt.Errorf("messages must alternate roles: message %d and %d both have role %q", i-1, i, m.Role)
		}
	}

	// Validate tools if present.
	for i := range r.Tools {
		if err := r.Tools[i].Validate(); err != nil {
			return fmt.Errorf("tool %d: %w", i, err)
		}
	}

	return nil
}

// Message represents a conversation message.
type Message struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

// TextContent extracts concatenated text from content blocks.
func (m *Message) TextContent() string {
	var text string
	for _, b := range m.Content {
		if b.Type == "text" {
			text += b.Text
		}
	}
	return text
}

// ContentBlock represents a piece of content within a message.
type ContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
}

// Tool represents a tool definition.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// Validate checks name non-empty and schema is valid JSON.
func (t *Tool) Validate() error {
	if t.Name == "" {
		return errors.New("tool name must not be empty")
	}
	if len(t.InputSchema) > 0 {
		var v interface{}
		if err := json.Unmarshal(t.InputSchema, &v); err != nil {
			return fmt.Errorf("input_schema is not valid JSON: %w", err)
		}
	}
	return nil
}

// SSEEvent represents a parsed Server-Sent Event from the Claude API.
type SSEEvent struct {
	Type         string             `json:"type"`
	Index        int                `json:"index,omitempty"`
	Delta        *ContentBlockDelta `json:"delta,omitempty"`
	Message      *Response          `json:"message,omitempty"`
	ContentBlock *ContentBlock      `json:"content_block,omitempty"`
	Usage        *Usage             `json:"usage,omitempty"`
	Error        *APIError          `json:"error,omitempty"`
	StopReason   string             `json:"stop_reason,omitempty"`
}

// IsTerminal returns true for message_stop or error events.
func (e *SSEEvent) IsTerminal() bool {
	return e.Type == "message_stop" || e.Type == "error"
}

// IsContent returns true for content_block_delta events.
func (e *SSEEvent) IsContent() bool {
	return e.Type == "content_block_delta"
}

// ContentBlockDelta represents incremental content in an SSE event.
type ContentBlockDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
}

// Usage represents token usage information from the Claude API.
type Usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// APIError represents an error returned by the Claude API.
type APIError struct {
	Type       string `json:"type"`
	Message    string `json:"message"`
	StatusCode int    `json:"-"`
}

// Error implements the error interface.
func (e *APIError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("claude api error (%d): %s: %s", e.StatusCode, e.Type, e.Message)
	}
	return fmt.Sprintf("claude api error: %s: %s", e.Type, e.Message)
}

// Response represents a complete Claude API response.
type Response struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Role       string         `json:"role"`
	Content    []ContentBlock `json:"content"`
	Model      string         `json:"model"`
	StopReason string         `json:"stop_reason"`
	Usage      *Usage         `json:"usage,omitempty"`
}

// AuthError is returned when authentication fails (401).
type AuthError struct {
	Err error
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("authentication failed: %v", e.Err)
}

func (e *AuthError) Unwrap() error {
	return e.Err
}

// RateLimitError is returned when the API rate limit is hit (429).
type RateLimitError struct {
	RetryAfter time.Duration
	Err        error
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limited (retry after %v): %v", e.RetryAfter, e.Err)
}

func (e *RateLimitError) Unwrap() error {
	return e.Err
}

// ServerError is returned for retryable server errors (500/502/503).
type ServerError struct {
	StatusCode int
	Err        error
}

func (e *ServerError) Error() string {
	return fmt.Sprintf("server error (%d): %v", e.StatusCode, e.Err)
}

func (e *ServerError) Unwrap() error {
	return e.Err
}

// IncompleteStreamError is returned when the stream ends without message_stop.
type IncompleteStreamError struct {
	LastEvent *SSEEvent
}

func (e *IncompleteStreamError) Error() string {
	if e.LastEvent != nil {
		return fmt.Sprintf("stream ended without message_stop (last event: %s)", e.LastEvent.Type)
	}
	return "stream ended without message_stop"
}
