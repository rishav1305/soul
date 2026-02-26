package ai_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rishav1305/soul/internal/ai"
	soulv1 "github.com/rishav1305/soul/proto/soul/v1"
)

func TestBuildToolsFromRegistry(t *testing.T) {
	tools := []*soulv1.Tool{
		{
			Name:            "scan",
			Description:     "Run compliance scan",
			InputSchemaJson: `{"type":"object","properties":{"path":{"type":"string"}}}`,
		},
		{
			Name:            "report",
			Description:     "Generate report",
			InputSchemaJson: `{"type":"object","properties":{"format":{"type":"string"}}}`,
		},
	}

	claudeTools := ai.BuildClaudeTools("compliance", tools)

	if len(claudeTools) != 2 {
		t.Fatalf("expected 2 claude tools, got %d", len(claudeTools))
	}

	// Verify qualified names use product__tool format.
	if claudeTools[0].Name != "compliance__scan" {
		t.Fatalf("expected name 'compliance__scan', got %q", claudeTools[0].Name)
	}
	if claudeTools[1].Name != "compliance__report" {
		t.Fatalf("expected name 'compliance__report', got %q", claudeTools[1].Name)
	}

	// Verify descriptions are passed through.
	if claudeTools[0].Description != "Run compliance scan" {
		t.Fatalf("expected description 'Run compliance scan', got %q", claudeTools[0].Description)
	}
	if claudeTools[1].Description != "Generate report" {
		t.Fatalf("expected description 'Generate report', got %q", claudeTools[1].Description)
	}

	// Verify InputSchema is valid JSON.
	for i, ct := range claudeTools {
		if !json.Valid(ct.InputSchema) {
			t.Fatalf("claude tool %d has invalid InputSchema JSON: %s", i, string(ct.InputSchema))
		}
	}

	// Verify the schema content is correct.
	var schema0 map[string]interface{}
	if err := json.Unmarshal(claudeTools[0].InputSchema, &schema0); err != nil {
		t.Fatalf("failed to unmarshal InputSchema[0]: %v", err)
	}
	if schema0["type"] != "object" {
		t.Fatalf("expected schema type 'object', got %v", schema0["type"])
	}
}

func TestBuildToolsInvalidSchema(t *testing.T) {
	tools := []*soulv1.Tool{
		{
			Name:            "broken",
			Description:     "A tool with broken schema",
			InputSchemaJson: "not valid json {{{",
		},
	}

	claudeTools := ai.BuildClaudeTools("myproduct", tools)

	if len(claudeTools) != 1 {
		t.Fatalf("expected 1 claude tool, got %d", len(claudeTools))
	}

	// Invalid JSON should default to {"type":"object"}.
	if !json.Valid(claudeTools[0].InputSchema) {
		t.Fatalf("expected valid JSON for default schema, got %s", string(claudeTools[0].InputSchema))
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(claudeTools[0].InputSchema, &schema); err != nil {
		t.Fatalf("failed to unmarshal default schema: %v", err)
	}
	if schema["type"] != "object" {
		t.Fatalf("expected default schema type 'object', got %v", schema["type"])
	}
}

func TestBuildToolsEmptySchema(t *testing.T) {
	tools := []*soulv1.Tool{
		{
			Name:            "empty",
			Description:     "A tool with empty schema",
			InputSchemaJson: "",
		},
	}

	claudeTools := ai.BuildClaudeTools("myproduct", tools)

	if len(claudeTools) != 1 {
		t.Fatalf("expected 1 claude tool, got %d", len(claudeTools))
	}

	// Empty schema should also default to {"type":"object"}.
	var schema map[string]interface{}
	if err := json.Unmarshal(claudeTools[0].InputSchema, &schema); err != nil {
		t.Fatalf("failed to unmarshal default schema: %v", err)
	}
	if schema["type"] != "object" {
		t.Fatalf("expected default schema type 'object', got %v", schema["type"])
	}
}

func TestBuildToolsEmptySlice(t *testing.T) {
	claudeTools := ai.BuildClaudeTools("product", nil)
	if len(claudeTools) != 0 {
		t.Fatalf("expected 0 claude tools for nil input, got %d", len(claudeTools))
	}

	claudeTools = ai.BuildClaudeTools("product", []*soulv1.Tool{})
	if len(claudeTools) != 0 {
		t.Fatalf("expected 0 claude tools for empty input, got %d", len(claudeTools))
	}
}

func TestNewClient(t *testing.T) {
	// NewClient should not panic even with empty API key.
	c := ai.NewClient("", "claude-sonnet-4-20250514")
	if c == nil {
		t.Fatal("expected NewClient to return non-nil client")
	}

	// NewClient with a real-looking key.
	c = ai.NewClient("sk-ant-api-test-key", "claude-sonnet-4-20250514")
	if c == nil {
		t.Fatal("expected NewClient to return non-nil client")
	}
}

func TestParseSSEStream(t *testing.T) {
	// Simulate a Claude SSE stream.
	sseData := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_123"}}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
		"",
		"",
	}, "\n")

	reader := bufio.NewReader(bytes.NewReader([]byte(sseData)))
	events := make(chan ai.StreamEvent, 10)
	go ai.ParseSSEStream(reader, events)

	var received []ai.StreamEvent
	for ev := range events {
		received = append(received, ev)
	}

	if len(received) != 5 {
		t.Fatalf("expected 5 events, got %d", len(received))
	}

	// Verify event types.
	expectedTypes := []string{
		"message_start",
		"content_block_start",
		"content_block_delta",
		"content_block_delta",
		"message_stop",
	}
	for i, ev := range received {
		if ev.Type != expectedTypes[i] {
			t.Fatalf("event %d: expected type %q, got %q", i, expectedTypes[i], ev.Type)
		}
		if !json.Valid(ev.Data) {
			t.Fatalf("event %d: data is not valid JSON: %s", i, string(ev.Data))
		}
	}
}

func TestParseSSEStreamEmpty(t *testing.T) {
	reader := bufio.NewReader(bytes.NewReader([]byte("")))
	events := make(chan ai.StreamEvent, 10)
	go ai.ParseSSEStream(reader, events)

	var received []ai.StreamEvent
	for ev := range events {
		received = append(received, ev)
	}

	if len(received) != 0 {
		t.Fatalf("expected 0 events for empty input, got %d", len(received))
	}
}

func TestParseSSEStreamDataOnly(t *testing.T) {
	// SSE lines without event: prefix — type should be empty string.
	sseData := `data: {"type":"ping"}

`
	reader := bufio.NewReader(bytes.NewReader([]byte(sseData)))
	events := make(chan ai.StreamEvent, 10)
	go ai.ParseSSEStream(reader, events)

	var received []ai.StreamEvent
	for ev := range events {
		received = append(received, ev)
	}

	if len(received) != 1 {
		t.Fatalf("expected 1 event, got %d", len(received))
	}
	if received[0].Type != "" {
		t.Fatalf("expected empty type for data-only event, got %q", received[0].Type)
	}
}

func TestRequestMarshaling(t *testing.T) {
	req := ai.Request{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 4096,
		System:    "You are a helpful assistant.",
		Messages: []ai.Message{
			{Role: "user", Content: "Hello"},
		},
		Tools: []ai.ClaudeTool{
			{
				Name:        "compliance__scan",
				Description: "Run compliance scan",
				InputSchema: json.RawMessage(`{"type":"object"}`),
			},
		},
		Stream: true,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	if parsed["model"] != "claude-sonnet-4-20250514" {
		t.Fatalf("expected model 'claude-sonnet-4-20250514', got %v", parsed["model"])
	}
	if parsed["stream"] != true {
		t.Fatalf("expected stream true, got %v", parsed["stream"])
	}
	if parsed["system"] != "You are a helpful assistant." {
		t.Fatalf("expected system prompt, got %v", parsed["system"])
	}
}
