package stream

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRequestValidate_EmptyMessages(t *testing.T) {
	r := &Request{MaxTokens: 100, Messages: nil}
	err := r.Validate()
	if err == nil {
		t.Fatal("expected error for empty messages")
	}
	if !strings.Contains(err.Error(), "messages must not be empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRequestValidate_MaxTokensZero(t *testing.T) {
	r := &Request{
		MaxTokens: 0,
		Messages:  []Message{{Role: "user", Content: []ContentBlock{{Type: "text", Text: "hi"}}}},
	}
	err := r.Validate()
	if err == nil {
		t.Fatal("expected error for zero max_tokens")
	}
	if !strings.Contains(err.Error(), "max_tokens must be greater than 0") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRequestValidate_NegativeMaxTokens(t *testing.T) {
	r := &Request{
		MaxTokens: -1,
		Messages:  []Message{{Role: "user", Content: []ContentBlock{{Type: "text", Text: "hi"}}}},
	}
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for negative max_tokens")
	}
}

func TestRequestValidate_InvalidRole(t *testing.T) {
	r := &Request{
		MaxTokens: 100,
		Messages: []Message{
			{Role: "system", Content: []ContentBlock{{Type: "text", Text: "hi"}}},
		},
	}
	err := r.Validate()
	if err == nil {
		t.Fatal("expected error for invalid role")
	}
	if !strings.Contains(err.Error(), "first message must have role \"user\"") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRequestValidate_InvalidRoleNotUserOrAssistant(t *testing.T) {
	r := &Request{
		MaxTokens: 100,
		Messages: []Message{
			{Role: "user", Content: []ContentBlock{{Type: "text", Text: "hi"}}},
			{Role: "bot", Content: []ContentBlock{{Type: "text", Text: "hello"}}},
		},
	}
	err := r.Validate()
	if err == nil {
		t.Fatal("expected error for invalid role 'bot'")
	}
	if !strings.Contains(err.Error(), "invalid role") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRequestValidate_NonAlternatingRoles(t *testing.T) {
	r := &Request{
		MaxTokens: 100,
		Messages: []Message{
			{Role: "user", Content: []ContentBlock{{Type: "text", Text: "hi"}}},
			{Role: "user", Content: []ContentBlock{{Type: "text", Text: "hello"}}},
		},
	}
	err := r.Validate()
	if err == nil {
		t.Fatal("expected error for non-alternating roles")
	}
	if !strings.Contains(err.Error(), "must alternate") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRequestValidate_FirstMessageMustBeUser(t *testing.T) {
	r := &Request{
		MaxTokens: 100,
		Messages: []Message{
			{Role: "assistant", Content: []ContentBlock{{Type: "text", Text: "hi"}}},
		},
	}
	err := r.Validate()
	if err == nil {
		t.Fatal("expected error when first message is not user")
	}
	if !strings.Contains(err.Error(), "first message must have role \"user\"") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRequestValidate_ValidRequest(t *testing.T) {
	r := &Request{
		MaxTokens: 100,
		Messages: []Message{
			{Role: "user", Content: []ContentBlock{{Type: "text", Text: "hi"}}},
			{Role: "assistant", Content: []ContentBlock{{Type: "text", Text: "hello"}}},
			{Role: "user", Content: []ContentBlock{{Type: "text", Text: "how are you?"}}},
		},
	}
	if err := r.Validate(); err != nil {
		t.Fatalf("expected valid request, got error: %v", err)
	}
}

func TestRequestValidate_InvalidToolSchema(t *testing.T) {
	r := &Request{
		MaxTokens: 100,
		Messages: []Message{
			{Role: "user", Content: []ContentBlock{{Type: "text", Text: "hi"}}},
		},
		Tools: []Tool{
			{Name: "test", Description: "test tool", InputSchema: json.RawMessage(`not json`)},
		},
	}
	err := r.Validate()
	if err == nil {
		t.Fatal("expected error for invalid tool schema")
	}
	if !strings.Contains(err.Error(), "not valid JSON") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestToolValidate_EmptyName(t *testing.T) {
	tool := &Tool{Name: "", Description: "test"}
	err := tool.Validate()
	if err == nil {
		t.Fatal("expected error for empty name")
	}
	if !strings.Contains(err.Error(), "name must not be empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestToolValidate_InvalidSchema(t *testing.T) {
	tool := &Tool{
		Name:        "test",
		Description: "test tool",
		InputSchema: json.RawMessage(`{invalid}`),
	}
	err := tool.Validate()
	if err == nil {
		t.Fatal("expected error for invalid JSON schema")
	}
	if !strings.Contains(err.Error(), "not valid JSON") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestToolValidate_ValidTool(t *testing.T) {
	tool := &Tool{
		Name:        "read_file",
		Description: "Reads a file",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
	}
	if err := tool.Validate(); err != nil {
		t.Fatalf("expected valid tool, got error: %v", err)
	}
}

func TestToolValidate_EmptySchema(t *testing.T) {
	tool := &Tool{Name: "test", Description: "test tool"}
	if err := tool.Validate(); err != nil {
		t.Fatalf("expected valid tool with no schema, got error: %v", err)
	}
}

func TestMessageTextContent(t *testing.T) {
	m := &Message{
		Role: "assistant",
		Content: []ContentBlock{
			{Type: "text", Text: "Hello "},
			{Type: "text", Text: "world"},
		},
	}
	got := m.TextContent()
	if got != "Hello world" {
		t.Fatalf("expected 'Hello world', got %q", got)
	}
}

func TestMessageTextContent_SkipsNonText(t *testing.T) {
	m := &Message{
		Role: "assistant",
		Content: []ContentBlock{
			{Type: "text", Text: "Hello"},
			{Type: "tool_use", Name: "read_file"},
			{Type: "text", Text: " world"},
		},
	}
	got := m.TextContent()
	if got != "Hello world" {
		t.Fatalf("expected 'Hello world', got %q", got)
	}
}

func TestMessageTextContent_Empty(t *testing.T) {
	m := &Message{Role: "user", Content: nil}
	if got := m.TextContent(); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestSSEEvent_IsTerminal(t *testing.T) {
	tests := []struct {
		eventType string
		want      bool
	}{
		{"message_stop", true},
		{"error", true},
		{"message_start", false},
		{"content_block_delta", false},
		{"ping", false},
		{"", false},
	}
	for _, tt := range tests {
		evt := &SSEEvent{Type: tt.eventType}
		if got := evt.IsTerminal(); got != tt.want {
			t.Errorf("SSEEvent{Type:%q}.IsTerminal() = %v, want %v", tt.eventType, got, tt.want)
		}
	}
}

func TestSSEEvent_IsContent(t *testing.T) {
	tests := []struct {
		eventType string
		want      bool
	}{
		{"content_block_delta", true},
		{"message_start", false},
		{"message_stop", false},
		{"error", false},
		{"", false},
	}
	for _, tt := range tests {
		evt := &SSEEvent{Type: tt.eventType}
		if got := evt.IsContent(); got != tt.want {
			t.Errorf("SSEEvent{Type:%q}.IsContent() = %v, want %v", tt.eventType, got, tt.want)
		}
	}
}

func TestAPIError_Error(t *testing.T) {
	e := &APIError{Type: "invalid_request_error", Message: "max_tokens is required"}
	got := e.Error()
	if !strings.Contains(got, "invalid_request_error") {
		t.Fatalf("expected error type in message, got %q", got)
	}
	if !strings.Contains(got, "max_tokens is required") {
		t.Fatalf("expected error message in output, got %q", got)
	}
}

func TestAPIError_ErrorWithStatusCode(t *testing.T) {
	e := &APIError{Type: "rate_limit_error", Message: "too many requests", StatusCode: 429}
	got := e.Error()
	if !strings.Contains(got, "429") {
		t.Fatalf("expected status code in message, got %q", got)
	}
	if !strings.Contains(got, "rate_limit_error") {
		t.Fatalf("expected error type in message, got %q", got)
	}
}

func TestUsageDefaults(t *testing.T) {
	// Usage fields should default to zero when JSON has no such fields.
	data := `{}`
	var u Usage
	if err := json.Unmarshal([]byte(data), &u); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if u.InputTokens != 0 || u.OutputTokens != 0 || u.CacheCreationInputTokens != 0 || u.CacheReadInputTokens != 0 {
		t.Fatalf("expected all zero, got %+v", u)
	}
}

func TestUsagePartialJSON(t *testing.T) {
	data := `{"input_tokens":100,"output_tokens":50}`
	var u Usage
	if err := json.Unmarshal([]byte(data), &u); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if u.InputTokens != 100 || u.OutputTokens != 50 {
		t.Fatalf("unexpected values: %+v", u)
	}
	if u.CacheCreationInputTokens != 0 || u.CacheReadInputTokens != 0 {
		t.Fatalf("expected cache fields zero, got %+v", u)
	}
}

func TestRequestJSON(t *testing.T) {
	r := &Request{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 1024,
		System:    "You are a helpful assistant.",
		Messages: []Message{
			{Role: "user", Content: []ContentBlock{{Type: "text", Text: "Hello"}}},
		},
		Stream: true,
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded Request
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if decoded.Model != r.Model || decoded.MaxTokens != r.MaxTokens || decoded.System != r.System || decoded.Stream != r.Stream {
		t.Fatalf("round-trip mismatch: %+v vs %+v", r, decoded)
	}
	if len(decoded.Messages) != 1 || decoded.Messages[0].Role != "user" {
		t.Fatalf("messages mismatch: %+v", decoded.Messages)
	}
}

func TestContentBlockTextExtraction(t *testing.T) {
	blocks := []ContentBlock{
		{Type: "text", Text: "Part 1. "},
		{Type: "tool_use", Name: "search", ID: "tu_1"},
		{Type: "text", Text: "Part 2."},
	}
	m := &Message{Role: "assistant", Content: blocks}
	want := "Part 1. Part 2."
	if got := m.TextContent(); got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

// --- Error type .Error() method coverage ---

func TestAuthError_Error(t *testing.T) {
	inner := errors.New("bad token")
	e := &AuthError{Err: inner}
	got := e.Error()
	if !strings.Contains(got, "authentication failed") {
		t.Errorf("expected 'authentication failed', got %q", got)
	}
	if !strings.Contains(got, "bad token") {
		t.Errorf("expected inner error message, got %q", got)
	}
}

func TestRateLimitError_Error(t *testing.T) {
	inner := errors.New("too many requests")
	e := &RateLimitError{RetryAfter: 30 * time.Second, Err: inner}
	got := e.Error()
	if !strings.Contains(got, "rate limited") {
		t.Errorf("expected 'rate limited', got %q", got)
	}
	if !strings.Contains(got, "30s") {
		t.Errorf("expected retry duration in message, got %q", got)
	}
	if !strings.Contains(got, "too many requests") {
		t.Errorf("expected inner error, got %q", got)
	}
}

func TestServerError_Error(t *testing.T) {
	inner := errors.New("internal failure")
	e := &ServerError{StatusCode: 503, Err: inner}
	got := e.Error()
	if !strings.Contains(got, "server error") {
		t.Errorf("expected 'server error', got %q", got)
	}
	if !strings.Contains(got, "503") {
		t.Errorf("expected status code, got %q", got)
	}
	if !strings.Contains(got, "internal failure") {
		t.Errorf("expected inner error, got %q", got)
	}
}

func TestIncompleteStreamError_WithLastEvent(t *testing.T) {
	e := &IncompleteStreamError{LastEvent: &SSEEvent{Type: "content_block_delta"}}
	got := e.Error()
	if !strings.Contains(got, "stream ended without message_stop") {
		t.Errorf("expected base message, got %q", got)
	}
	if !strings.Contains(got, "content_block_delta") {
		t.Errorf("expected last event type, got %q", got)
	}
}

func TestIncompleteStreamError_NilLastEvent(t *testing.T) {
	e := &IncompleteStreamError{}
	got := e.Error()
	if got != "stream ended without message_stop" {
		t.Errorf("expected plain message, got %q", got)
	}
}

func TestTruncateBody_Short(t *testing.T) {
	got := truncateBody([]byte("short"), 100)
	if got != "short" {
		t.Errorf("expected 'short', got %q", got)
	}
}

func TestTruncateBody_ExactLen(t *testing.T) {
	data := []byte("12345")
	got := truncateBody(data, 5)
	if got != "12345" {
		t.Errorf("expected '12345', got %q", got)
	}
}

func TestTruncateBody_Long(t *testing.T) {
	data := []byte("abcdefghij") // 10 bytes
	got := truncateBody(data, 5)
	if got != "abcde...(truncated)" {
		t.Errorf("expected truncated, got %q", got)
	}
}

func TestTruncateBody_Empty(t *testing.T) {
	got := truncateBody(nil, 100)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestRequestValidate_SkipValidation(t *testing.T) {
	// With SkipValidation, non-alternating roles and invalid roles (after user) should be OK.
	r := &Request{
		MaxTokens:      100,
		SkipValidation: true,
		Messages: []Message{
			{Role: "user", Content: []ContentBlock{{Type: "text", Text: "hi"}}},
			{Role: "user", Content: []ContentBlock{{Type: "text", Text: "tool result"}}},
		},
	}
	if err := r.Validate(); err != nil {
		t.Fatalf("expected skip to allow non-alternating, got: %v", err)
	}
}

func TestThinkingParam_JSON(t *testing.T) {
	tp := &ThinkingParam{Type: "enabled", BudgetTokens: 1000}
	data, err := json.Marshal(tp)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ThinkingParam
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Type != "enabled" || decoded.BudgetTokens != 1000 {
		t.Errorf("round-trip mismatch: %+v", decoded)
	}
}

func TestImageSource_JSON(t *testing.T) {
	src := &ImageSource{Type: "base64", MediaType: "image/png", Data: "abc123"}
	data, err := json.Marshal(src)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ImageSource
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Type != "base64" || decoded.MediaType != "image/png" || decoded.Data != "abc123" {
		t.Errorf("round-trip mismatch: %+v", decoded)
	}
}

func TestContentBlock_ToolUseJSON(t *testing.T) {
	cb := ContentBlock{
		Type:      "tool_use",
		ID:        "tu_123",
		Name:      "read_file",
		Input:     json.RawMessage(`{"path":"test.go"}`),
	}
	data, err := json.Marshal(cb)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "tool_use") {
		t.Error("expected tool_use in JSON")
	}
	if !strings.Contains(string(data), "tu_123") {
		t.Error("expected tool use ID in JSON")
	}
}

func TestContentBlock_ToolResultJSON(t *testing.T) {
	cb := ContentBlock{
		Type:      "tool_result",
		ToolUseID: "tu_123",
		Content:   "file contents here",
	}
	data, err := json.Marshal(cb)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ContentBlock
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ToolUseID != "tu_123" || decoded.Content != "file contents here" {
		t.Errorf("round-trip mismatch: %+v", decoded)
	}
}
