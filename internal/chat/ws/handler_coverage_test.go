package ws

import (
	"encoding/json"
	"testing"

	"github.com/rishav1305/soul/internal/chat/session"
	"github.com/rishav1305/soul/internal/chat/stream"
)

// ============================================================
// buildAPIMessages — cover tool_use, tool_result, bad JSON paths
// ============================================================

func TestBuildAPIMessages_ToolUseAndResult(t *testing.T) {
	toolUseBlocks, _ := json.Marshal([]stream.ContentBlock{
		{Type: "tool_use", ID: "tu_1", Name: "get_weather", Input: json.RawMessage(`{"city":"SF"}`)},
	})

	toolResult, _ := json.Marshal(struct {
		ToolUseID string `json:"tool_use_id"`
		Content   string `json:"content"`
	}{ToolUseID: "tu_1", Content: "72F sunny"})

	msgs := []*session.Message{
		{Role: "user", Content: "What's the weather?"},
		{Role: "tool_use", Content: string(toolUseBlocks)},
		{Role: "tool_result", Content: string(toolResult)},
		{Role: "assistant", Content: "It's 72F and sunny in SF."},
	}

	result := buildAPIMessages(msgs)
	if len(result) == 0 {
		t.Fatal("expected non-empty result")
	}

	// Should have: user, assistant (tool_use), user (tool_result), assistant (text)
	// normalized into merged messages.
	foundToolResult := false
	for _, m := range result {
		for _, cb := range m.Content {
			if cb.Type == "tool_result" {
				foundToolResult = true
				if cb.ToolUseID != "tu_1" {
					t.Errorf("expected tool_use_id=tu_1, got %q", cb.ToolUseID)
				}
			}
		}
	}
	if !foundToolResult {
		t.Error("expected tool_result in API messages")
	}
}

func TestBuildAPIMessages_ToolUse_InvalidJSON(t *testing.T) {
	msgs := []*session.Message{
		{Role: "user", Content: "hi"},
		{Role: "tool_use", Content: "not valid json"},
		{Role: "assistant", Content: "result"},
	}

	result := buildAPIMessages(msgs)
	// The tool_use message with bad JSON should be skipped (logged).
	// Should have user + assistant.
	if len(result) < 1 {
		t.Error("expected at least 1 message")
	}
}

func TestBuildAPIMessages_ToolResult_InvalidJSON(t *testing.T) {
	msgs := []*session.Message{
		{Role: "user", Content: "hi"},
		{Role: "tool_result", Content: "bad json"},
		{Role: "assistant", Content: "result"},
	}

	result := buildAPIMessages(msgs)
	// Bad tool_result JSON should be skipped.
	if len(result) < 1 {
		t.Error("expected at least 1 message")
	}
}

func TestBuildAPIMessages_EmptyMessages(t *testing.T) {
	result := buildAPIMessages(nil)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d messages", len(result))
	}
}

// ============================================================
// fixOrphanedToolUse — synthetic tool_result injection
// ============================================================

func TestFixOrphanedToolUse_AddsResultForMissingID(t *testing.T) {
	messages := []stream.Message{
		{
			Role: "user",
			Content: []stream.ContentBlock{
				{Type: "text", Text: "hi"},
			},
		},
		{
			Role: "assistant",
			Content: []stream.ContentBlock{
				{Type: "tool_use", ID: "tu_orphan", Name: "get_data"},
			},
		},
		// No tool_result follows — orphaned.
	}

	fixed := fixOrphanedToolUse(messages)

	// Should have 3 messages: user, assistant (tool_use), user (synthetic tool_result).
	if len(fixed) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(fixed))
	}

	// Third message should be a user message with tool_result.
	lastMsg := fixed[2]
	if lastMsg.Role != "user" {
		t.Errorf("expected user role for synthetic result, got %q", lastMsg.Role)
	}
	if len(lastMsg.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(lastMsg.Content))
	}
	if lastMsg.Content[0].Type != "tool_result" {
		t.Errorf("expected tool_result type, got %q", lastMsg.Content[0].Type)
	}
	if lastMsg.Content[0].ToolUseID != "tu_orphan" {
		t.Errorf("expected tu_orphan, got %q", lastMsg.Content[0].ToolUseID)
	}
}

func TestFixOrphanedToolUse_NoOrphans(t *testing.T) {
	messages := []stream.Message{
		{Role: "user", Content: []stream.ContentBlock{{Type: "text", Text: "hi"}}},
		{
			Role:    "assistant",
			Content: []stream.ContentBlock{{Type: "tool_use", ID: "tu_1", Name: "search"}},
		},
		{
			Role:    "user",
			Content: []stream.ContentBlock{{Type: "tool_result", ToolUseID: "tu_1", Content: "found"}},
		},
	}

	fixed := fixOrphanedToolUse(messages)
	if len(fixed) != 3 {
		t.Errorf("expected 3 messages (no orphans), got %d", len(fixed))
	}
}

func TestFixOrphanedToolUse_MultipleToolUseBlocks(t *testing.T) {
	messages := []stream.Message{
		{
			Role: "assistant",
			Content: []stream.ContentBlock{
				{Type: "tool_use", ID: "tu_a", Name: "search"},
				{Type: "tool_use", ID: "tu_b", Name: "read"},
			},
		},
		{
			Role: "user",
			Content: []stream.ContentBlock{
				{Type: "tool_result", ToolUseID: "tu_a", Content: "found"},
				// tu_b missing — orphaned.
			},
		},
	}

	fixed := fixOrphanedToolUse(messages)

	// Should add a synthetic tool_result for tu_b.
	foundSynthetic := false
	for _, m := range fixed {
		for _, cb := range m.Content {
			if cb.Type == "tool_result" && cb.ToolUseID == "tu_b" {
				foundSynthetic = true
				if cb.Content != "[tool execution was interrupted]" {
					t.Errorf("unexpected synthetic content: %q", cb.Content)
				}
			}
		}
	}
	if !foundSynthetic {
		t.Error("expected synthetic tool_result for tu_b")
	}
}

func TestFixOrphanedToolUse_NoToolUse(t *testing.T) {
	messages := []stream.Message{
		{Role: "user", Content: []stream.ContentBlock{{Type: "text", Text: "hi"}}},
		{Role: "assistant", Content: []stream.ContentBlock{{Type: "text", Text: "hello"}}},
	}

	fixed := fixOrphanedToolUse(messages)
	if len(fixed) != 2 {
		t.Errorf("expected 2 messages unchanged, got %d", len(fixed))
	}
}

// ============================================================
// normalizeAPIMessages — merge consecutive same-role messages
// ============================================================

func TestNormalizeAPIMessages_Empty(t *testing.T) {
	result := normalizeAPIMessages(nil)
	if len(result) != 0 {
		t.Errorf("expected empty, got %d", len(result))
	}
}

func TestNormalizeAPIMessages_NoMergeNeeded(t *testing.T) {
	messages := []stream.Message{
		{Role: "user", Content: []stream.ContentBlock{{Type: "text", Text: "hi"}}},
		{Role: "assistant", Content: []stream.ContentBlock{{Type: "text", Text: "hello"}}},
	}

	result := normalizeAPIMessages(messages)
	if len(result) != 2 {
		t.Errorf("expected 2, got %d", len(result))
	}
}

func TestNormalizeAPIMessages_MergesConsecutiveUser(t *testing.T) {
	messages := []stream.Message{
		{Role: "user", Content: []stream.ContentBlock{{Type: "text", Text: "part1"}}},
		{Role: "user", Content: []stream.ContentBlock{{Type: "text", Text: "part2"}}},
		{Role: "assistant", Content: []stream.ContentBlock{{Type: "text", Text: "reply"}}},
	}

	result := normalizeAPIMessages(messages)
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
	if len(result[0].Content) != 2 {
		t.Errorf("expected 2 content blocks in merged user, got %d", len(result[0].Content))
	}
}

func TestNormalizeAPIMessages_MergesConsecutiveAssistant(t *testing.T) {
	messages := []stream.Message{
		{Role: "user", Content: []stream.ContentBlock{{Type: "text", Text: "hi"}}},
		{Role: "assistant", Content: []stream.ContentBlock{{Type: "text", Text: "part1"}}},
		{Role: "assistant", Content: []stream.ContentBlock{{Type: "text", Text: "part2"}}},
	}

	result := normalizeAPIMessages(messages)
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
	if len(result[1].Content) != 2 {
		t.Errorf("expected 2 content blocks in merged assistant, got %d", len(result[1].Content))
	}
}

func TestNormalizeAPIMessages_EmptyContent(t *testing.T) {
	messages := []stream.Message{
		{Role: "user", Content: nil},
		{Role: "assistant", Content: []stream.ContentBlock{{Type: "text", Text: "reply"}}},
	}

	result := normalizeAPIMessages(messages)
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
}

// ============================================================
// buildAPIMessages — tool_use and tool_result full round trip
// ============================================================

func TestBuildAPIMessages_FullToolRoundTrip(t *testing.T) {
	toolUseBlocks, _ := json.Marshal([]stream.ContentBlock{
		{Type: "tool_use", ID: "tu_1", Name: "list_tasks"},
		{Type: "tool_use", ID: "tu_2", Name: "get_status"},
	})

	toolResult1, _ := json.Marshal(struct {
		ToolUseID string `json:"tool_use_id"`
		Content   string `json:"content"`
	}{ToolUseID: "tu_1", Content: "3 tasks"})

	toolResult2, _ := json.Marshal(struct {
		ToolUseID string `json:"tool_use_id"`
		Content   string `json:"content"`
	}{ToolUseID: "tu_2", Content: "all green"})

	msgs := []*session.Message{
		{Role: "user", Content: "Show tasks and status"},
		{Role: "tool_use", Content: string(toolUseBlocks)},
		{Role: "tool_result", Content: string(toolResult1)},
		{Role: "tool_result", Content: string(toolResult2)},
		{Role: "assistant", Content: "You have 3 tasks, all green."},
	}

	result := buildAPIMessages(msgs)
	if len(result) == 0 {
		t.Fatal("expected non-empty result")
	}

	// Count distinct roles
	roles := make(map[string]int)
	for _, m := range result {
		roles[m.Role]++
	}
	// Should have user and assistant messages.
	if roles["user"] == 0 {
		t.Error("expected at least one user message")
	}
	if roles["assistant"] == 0 {
		t.Error("expected at least one assistant message")
	}
}

// ============================================================
// buildAPIMessages with orphaned tool_use (interrupted stream)
// ============================================================

func TestBuildAPIMessages_OrphanedToolUseGetsFixedUp(t *testing.T) {
	toolUseBlocks, _ := json.Marshal([]stream.ContentBlock{
		{Type: "tool_use", ID: "tu_orphan", Name: "search"},
	})

	msgs := []*session.Message{
		{Role: "user", Content: "Search for something"},
		{Role: "tool_use", Content: string(toolUseBlocks)},
		// Stream was interrupted — no tool_result, no assistant reply.
	}

	result := buildAPIMessages(msgs)

	// Should have added a synthetic tool_result.
	foundSynthetic := false
	for _, m := range result {
		for _, cb := range m.Content {
			if cb.Type == "tool_result" && cb.ToolUseID == "tu_orphan" {
				foundSynthetic = true
			}
		}
	}
	if !foundSynthetic {
		t.Error("expected synthetic tool_result for orphaned tool_use")
	}
}
