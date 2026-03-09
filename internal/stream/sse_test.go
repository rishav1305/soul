package stream

import (
	"io"
	"strings"
	"testing"
)

func TestSSEParser_SingleEvent(t *testing.T) {
	input := "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[],\"model\":\"claude-sonnet-4-20250514\",\"stop_reason\":null,\"usage\":{\"input_tokens\":10,\"output_tokens\":1}}}\n\n"
	p := NewSSEParser(strings.NewReader(input))

	evt, err := p.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evt.Type != "message_start" {
		t.Fatalf("expected type message_start, got %q", evt.Type)
	}
	if evt.Message == nil {
		t.Fatal("expected Message to be non-nil")
	}
	if evt.Message.ID != "msg_1" {
		t.Fatalf("expected message ID msg_1, got %q", evt.Message.ID)
	}
	if evt.Message.Role != "assistant" {
		t.Fatalf("expected role assistant, got %q", evt.Message.Role)
	}

	// Next call should return EOF.
	_, err = p.Next()
	if err != io.EOF {
		t.Fatalf("expected io.EOF, got %v", err)
	}
}

func TestSSEParser_ContentBlockDelta(t *testing.T) {
	input := "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n\n"
	p := NewSSEParser(strings.NewReader(input))

	evt, err := p.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evt.Type != "content_block_delta" {
		t.Fatalf("expected content_block_delta, got %q", evt.Type)
	}
	if !evt.IsContent() {
		t.Fatal("expected IsContent() to be true")
	}
	if evt.Delta == nil {
		t.Fatal("expected Delta to be non-nil")
	}
	if evt.Delta.Text != "Hello" {
		t.Fatalf("expected 'Hello', got %q", evt.Delta.Text)
	}
	if evt.Delta.Type != "text_delta" {
		t.Fatalf("expected delta type text_delta, got %q", evt.Delta.Type)
	}
	if evt.Index != 0 {
		t.Fatalf("expected index 0, got %d", evt.Index)
	}
}

func TestSSEParser_MultiLineData(t *testing.T) {
	// Multi-line data: each line has "data:" prefix.
	input := "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\ndata: \"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hi\"}}\n\n"
	p := NewSSEParser(strings.NewReader(input))

	evt, err := p.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evt.Type != "content_block_delta" {
		t.Fatalf("expected content_block_delta, got %q", evt.Type)
	}
	if evt.Delta == nil {
		t.Fatal("expected Delta non-nil")
	}
	if evt.Delta.Text != "Hi" {
		t.Fatalf("expected 'Hi', got %q", evt.Delta.Text)
	}
}

func TestSSEParser_SkipsComments(t *testing.T) {
	input := ": this is a comment\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	p := NewSSEParser(strings.NewReader(input))

	evt, err := p.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evt.Type != "message_stop" {
		t.Fatalf("expected message_stop, got %q", evt.Type)
	}
}

func TestSSEParser_EmptyLinesBetweenEvents(t *testing.T) {
	input := "\n\nevent: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[],\"model\":\"test\",\"stop_reason\":null}}\n\n\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	p := NewSSEParser(strings.NewReader(input))

	evt1, err := p.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evt1.Type != "message_start" {
		t.Fatalf("expected message_start, got %q", evt1.Type)
	}

	evt2, err := p.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evt2.Type != "message_stop" {
		t.Fatalf("expected message_stop, got %q", evt2.Type)
	}
}

func TestSSEParser_SkipsPing(t *testing.T) {
	input := "event: ping\ndata: {\"type\":\"ping\"}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	p := NewSSEParser(strings.NewReader(input))

	evt, err := p.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should skip ping and return message_stop.
	if evt.Type != "message_stop" {
		t.Fatalf("expected message_stop (ping skipped), got %q", evt.Type)
	}
}

func TestSSEParser_IgnoresRetryField(t *testing.T) {
	input := "retry: 5000\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	p := NewSSEParser(strings.NewReader(input))

	evt, err := p.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evt.Type != "message_stop" {
		t.Fatalf("expected message_stop, got %q", evt.Type)
	}
}

func TestSSEParser_ErrorEvent(t *testing.T) {
	input := "event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\",\"message\":\"Overloaded\"}}\n\n"
	p := NewSSEParser(strings.NewReader(input))

	evt, err := p.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evt.Type != "error" {
		t.Fatalf("expected error, got %q", evt.Type)
	}
	if !evt.IsTerminal() {
		t.Fatal("expected IsTerminal() to be true for error")
	}
	if evt.Error == nil {
		t.Fatal("expected Error to be non-nil")
	}
	if evt.Error.Type != "overloaded_error" {
		t.Fatalf("expected overloaded_error, got %q", evt.Error.Type)
	}
	if evt.Error.Message != "Overloaded" {
		t.Fatalf("expected 'Overloaded', got %q", evt.Error.Message)
	}
}

func TestSSEParser_InvalidJSON(t *testing.T) {
	input := "event: message_start\ndata: not json at all\n\n"
	p := NewSSEParser(strings.NewReader(input))

	_, err := p.Next()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	pe, ok := err.(*ParseError)
	if !ok {
		t.Fatalf("expected *ParseError, got %T", err)
	}
	if pe.EventType != "message_start" {
		t.Fatalf("expected event type message_start, got %q", pe.EventType)
	}
	if pe.RawData != "not json at all" {
		t.Fatalf("expected raw data preserved, got %q", pe.RawData)
	}
}

func TestSSEParser_UnknownEventType(t *testing.T) {
	input := "event: custom_event\ndata: {\"foo\":\"bar\"}\n\n"
	p := NewSSEParser(strings.NewReader(input))

	evt, err := p.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evt.Type != "custom_event" {
		t.Fatalf("expected custom_event, got %q", evt.Type)
	}
}

func TestSSEParser_ContentBlockStart(t *testing.T) {
	input := "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n"
	p := NewSSEParser(strings.NewReader(input))

	evt, err := p.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evt.Type != "content_block_start" {
		t.Fatalf("expected content_block_start, got %q", evt.Type)
	}
	if evt.ContentBlock == nil {
		t.Fatal("expected ContentBlock non-nil")
	}
	if evt.ContentBlock.Type != "text" {
		t.Fatalf("expected block type text, got %q", evt.ContentBlock.Type)
	}
}

func TestSSEParser_ContentBlockStop(t *testing.T) {
	input := "event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n"
	p := NewSSEParser(strings.NewReader(input))

	evt, err := p.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evt.Type != "content_block_stop" {
		t.Fatalf("expected content_block_stop, got %q", evt.Type)
	}
	if evt.Index != 0 {
		t.Fatalf("expected index 0, got %d", evt.Index)
	}
}

func TestSSEParser_MessageDelta(t *testing.T) {
	input := "event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":42}}\n\n"
	p := NewSSEParser(strings.NewReader(input))

	evt, err := p.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evt.Type != "message_delta" {
		t.Fatalf("expected message_delta, got %q", evt.Type)
	}
	if evt.Usage == nil {
		t.Fatal("expected Usage non-nil")
	}
	if evt.Usage.OutputTokens != 42 {
		t.Fatalf("expected output_tokens 42, got %d", evt.Usage.OutputTokens)
	}
	if evt.StopReason != "end_turn" {
		t.Fatalf("expected stop_reason end_turn, got %q", evt.StopReason)
	}
}

func TestSSEParser_SplitChunks(t *testing.T) {
	// Simulate data that arrives in two separate reads by using a reader
	// that presents the data as a complete stream. The bufio.Scanner handles
	// the underlying buffering — we test that the parser correctly handles
	// multiple events in sequence.
	chunk1 := "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n\n"
	chunk2 := "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\" world\"}}\n\n"

	p := NewSSEParser(strings.NewReader(chunk1 + chunk2))

	evt1, err := p.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evt1.Delta == nil || evt1.Delta.Text != "Hello" {
		t.Fatalf("expected 'Hello', got %v", evt1.Delta)
	}

	evt2, err := p.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evt2.Delta == nil || evt2.Delta.Text != " world" {
		t.Fatalf("expected ' world', got %v", evt2.Delta)
	}
}

func TestSSEParser_InputJSONDelta(t *testing.T) {
	input := "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"path\\\":\"}}\n\n"
	p := NewSSEParser(strings.NewReader(input))

	evt, err := p.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evt.Delta == nil {
		t.Fatal("expected Delta non-nil")
	}
	if evt.Delta.Type != "input_json_delta" {
		t.Fatalf("expected input_json_delta, got %q", evt.Delta.Type)
	}
	if evt.Delta.PartialJSON != `{"path":` {
		t.Fatalf("expected partial JSON, got %q", evt.Delta.PartialJSON)
	}
}

func TestSSEParser_FullConversation(t *testing.T) {
	// Simulate a realistic Claude streaming conversation.
	input := `event: message_start
data: {"type":"message_start","message":{"id":"msg_123","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-20250514","stop_reason":null,"usage":{"input_tokens":25,"output_tokens":1}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: ping
data: {"type":"ping"}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"!"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":12}}

event: message_stop
data: {"type":"message_stop"}

`
	p := NewSSEParser(strings.NewReader(input))

	expectedTypes := []string{
		"message_start",
		"content_block_start",
		// ping is skipped
		"content_block_delta",
		"content_block_delta",
		"content_block_stop",
		"message_delta",
		"message_stop",
	}

	for i, expected := range expectedTypes {
		evt, err := p.Next()
		if err != nil {
			t.Fatalf("event %d: unexpected error: %v", i, err)
		}
		if evt.Type != expected {
			t.Fatalf("event %d: expected %q, got %q", i, expected, evt.Type)
		}
	}

	_, err := p.Next()
	if err != io.EOF {
		t.Fatalf("expected io.EOF after all events, got %v", err)
	}
}

func TestSSEParser_EmptyInput(t *testing.T) {
	p := NewSSEParser(strings.NewReader(""))
	_, err := p.Next()
	if err != io.EOF {
		t.Fatalf("expected io.EOF for empty input, got %v", err)
	}
}

func TestSSEParser_OnlyComments(t *testing.T) {
	input := ": comment 1\n: comment 2\n"
	p := NewSSEParser(strings.NewReader(input))
	_, err := p.Next()
	if err != io.EOF {
		t.Fatalf("expected io.EOF for only comments, got %v", err)
	}
}

func TestSSEParser_DataWithoutEvent(t *testing.T) {
	// Data line without a preceding event: line — should be skipped.
	input := "data: {\"type\":\"test\"}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	p := NewSSEParser(strings.NewReader(input))

	evt, err := p.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evt.Type != "message_stop" {
		t.Fatalf("expected message_stop, got %q", evt.Type)
	}
}

func TestSSEParser_EventAtEOFWithoutTrailingNewline(t *testing.T) {
	// Event at end of stream without trailing blank line.
	input := "event: message_stop\ndata: {\"type\":\"message_stop\"}"
	p := NewSSEParser(strings.NewReader(input))

	evt, err := p.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evt.Type != "message_stop" {
		t.Fatalf("expected message_stop, got %q", evt.Type)
	}
}

func TestSSEParser_DataSpaceTrimming(t *testing.T) {
	// SSE spec: "data: value" — the space after colon is stripped.
	input := "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	p := NewSSEParser(strings.NewReader(input))

	evt, err := p.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evt.Type != "message_stop" {
		t.Fatalf("expected message_stop, got %q", evt.Type)
	}
}

func TestParseError_Unwrap(t *testing.T) {
	input := "event: message_start\ndata: {broken\n\n"
	p := NewSSEParser(strings.NewReader(input))

	_, err := p.Next()
	if err == nil {
		t.Fatal("expected error")
	}
	pe, ok := err.(*ParseError)
	if !ok {
		t.Fatalf("expected *ParseError, got %T", err)
	}
	if pe.Unwrap() == nil {
		t.Fatal("expected wrapped error")
	}
}
