package executor

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/rishav1305/soul/internal/chat/stream"
)

// mockSender implements Sender and returns pre-configured responses in order.
// Once all pre-configured responses have been consumed it returns a default
// end_turn response with text "done".
type mockSender struct {
	responses []*stream.Response
	calls     int
}

func (m *mockSender) Send(_ context.Context, _ *stream.Request) (*stream.Response, error) {
	idx := m.calls
	m.calls++

	if idx < len(m.responses) {
		return m.responses[idx], nil
	}

	// Default fallback: end_turn with "done".
	return &stream.Response{
		Role:       "assistant",
		StopReason: "end_turn",
		Content: []stream.ContentBlock{
			{Type: "text", Text: "done"},
		},
		Usage: &stream.Usage{InputTokens: 10, OutputTokens: 5},
	}, nil
}

// newEndTurnResponse builds a simple end_turn response.
func newEndTurnResponse(text string, in, out int) *stream.Response {
	return &stream.Response{
		Role:       "assistant",
		StopReason: "end_turn",
		Content: []stream.ContentBlock{
			{Type: "text", Text: text},
		},
		Usage: &stream.Usage{InputTokens: in, OutputTokens: out},
	}
}

// newToolUseResponse builds a tool_use response for a single bash call.
func newToolUseResponse(toolID, command string, in, out int) *stream.Response {
	inputJSON, _ := json.Marshal(map[string]string{"command": command})
	return &stream.Response{
		Role:       "assistant",
		StopReason: "tool_use",
		Content: []stream.ContentBlock{
			{
				Type:  "tool_use",
				ID:    toolID,
				Name:  "bash",
				Input: json.RawMessage(inputJSON),
			},
		},
		Usage: &stream.Usage{InputTokens: in, OutputTokens: out},
	}
}

// TestAgentLoopSimpleResponse verifies that a single end_turn response
// returns immediately with iterations=1.
func TestAgentLoopSimpleResponse(t *testing.T) {
	sender := &mockSender{
		responses: []*stream.Response{
			newEndTurnResponse("hello world", 100, 20),
		},
	}

	loop := NewAgentLoop(sender, newTestToolSet(t), 1, 10, nil)
	result, err := loop.Run(context.Background(), "system", "say hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Text != "hello world" {
		t.Errorf("expected text %q, got %q", "hello world", result.Text)
	}
	if result.Iterations != 1 {
		t.Errorf("expected iterations=1, got %d", result.Iterations)
	}
	if result.HitLimit {
		t.Error("expected HitLimit=false")
	}
	if len(result.ToolCalls) != 0 {
		t.Errorf("expected no tool calls, got %d", len(result.ToolCalls))
	}
	if result.TotalInputTokens != 100 {
		t.Errorf("expected TotalInputTokens=100, got %d", result.TotalInputTokens)
	}
	if result.TotalOutputTokens != 20 {
		t.Errorf("expected TotalOutputTokens=20, got %d", result.TotalOutputTokens)
	}
}

// TestAgentLoopToolCall verifies that a tool_use response triggers execution
// and the loop then processes the follow-up end_turn response.
func TestAgentLoopToolCall(t *testing.T) {
	sender := &mockSender{
		responses: []*stream.Response{
			newToolUseResponse("tool-1", "echo hello", 50, 30),
			newEndTurnResponse("all done", 60, 10),
		},
	}

	loop := NewAgentLoop(sender, newTestToolSet(t), 1, 10, nil)
	result, err := loop.Run(context.Background(), "system", "run bash")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Iterations != 2 {
		t.Errorf("expected iterations=2, got %d", result.Iterations)
	}
	if result.Text != "all done" {
		t.Errorf("expected text %q, got %q", "all done", result.Text)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}
	if result.ToolCalls[0].Name != "bash" {
		t.Errorf("expected tool name %q, got %q", "bash", result.ToolCalls[0].Name)
	}
	// Verify token accumulation across both iterations.
	if result.TotalInputTokens != 110 {
		t.Errorf("expected TotalInputTokens=110, got %d", result.TotalInputTokens)
	}
	if result.TotalOutputTokens != 40 {
		t.Errorf("expected TotalOutputTokens=40, got %d", result.TotalOutputTokens)
	}
}

// TestAgentLoopHitsIterationLimit verifies that HitLimit is set when the model
// keeps returning tool_use responses past the configured maxIter.
func TestAgentLoopHitsIterationLimit(t *testing.T) {
	// All responses are tool_use — the loop should never get an end_turn.
	responses := make([]*stream.Response, 10)
	for i := range responses {
		responses[i] = newToolUseResponse("tool-inf", "echo loop", 10, 5)
	}

	sender := &mockSender{responses: responses}
	loop := NewAgentLoop(sender, newTestToolSet(t), 1, 3, nil)
	result, err := loop.Run(context.Background(), "system", "loop forever")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Iterations != 3 {
		t.Errorf("expected iterations=3, got %d", result.Iterations)
	}
	if !result.HitLimit {
		t.Error("expected HitLimit=true")
	}
}

// TestAgentLoopContextCancellation verifies that a cancelled context causes
// Run to return an error immediately.
func TestAgentLoopContextCancellation(t *testing.T) {
	// No pre-configured responses needed — context is cancelled before the first call.
	sender := &mockSender{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	loop := NewAgentLoop(sender, newTestToolSet(t), 1, 10, nil)
	_, err := loop.Run(ctx, "system", "do something")
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
}
