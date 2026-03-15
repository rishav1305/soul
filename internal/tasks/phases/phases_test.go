package phases

import (
	"context"
	"testing"

	"github.com/rishav1305/soul-v2/internal/chat/stream"
)

func TestPhaseConfig_Defaults(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.PlanModel != "claude-opus-4-6" {
		t.Errorf("PlanModel = %q, want %q", cfg.PlanModel, "claude-opus-4-6")
	}
	if cfg.ImplModel != "claude-sonnet-4-6" {
		t.Errorf("ImplModel = %q, want %q", cfg.ImplModel, "claude-sonnet-4-6")
	}
	if cfg.ReviewModel != "claude-opus-4-6" {
		t.Errorf("ReviewModel = %q, want %q", cfg.ReviewModel, "claude-opus-4-6")
	}
	if cfg.FixModel != "claude-opus-4-6" {
		t.Errorf("FixModel = %q, want %q", cfg.FixModel, "claude-opus-4-6")
	}
}

func TestMaxIterations(t *testing.T) {
	tests := []struct {
		workflow string
		want     int
	}{
		{"micro", 15},
		{"quick", 30},
		{"full", 40},
		{"", 30},
	}
	for _, tt := range tests {
		if got := MaxIterations(tt.workflow); got != tt.want {
			t.Errorf("MaxIterations(%q) = %d, want %d", tt.workflow, got, tt.want)
		}
	}
}

// mockSender records calls and returns pre-configured responses.
type mockSender struct {
	responses []*stream.Response
	callCount int
}

func (m *mockSender) Send(_ context.Context, _ *stream.Request) (*stream.Response, error) {
	idx := m.callCount
	if idx >= len(m.responses) {
		idx = len(m.responses) - 1
	}
	m.callCount++
	return m.responses[idx], nil
}

func textResponse(text, stopReason string, input, output int) *stream.Response {
	return &stream.Response{
		Content: []stream.ContentBlock{
			{Type: "text", Text: text},
		},
		StopReason: stopReason,
		Usage:      &stream.Usage{InputTokens: input, OutputTokens: output},
	}
}

func TestPhaseRunner_Micro(t *testing.T) {
	sender := &mockSender{
		responses: []*stream.Response{
			textResponse("task completed", "end_turn", 100, 50),
		},
	}

	pr := NewPhaseRunner(sender, DefaultConfig(), "/tmp/test")
	result, err := pr.RunTask(context.Background(), "micro", "do something", "you are helpful")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sender.callCount != 1 {
		t.Errorf("expected 1 call, got %d", sender.callCount)
	}
	if result.Text != "task completed" {
		t.Errorf("expected text 'task completed', got %q", result.Text)
	}
	if result.Iterations != 1 {
		t.Errorf("expected 1 iteration, got %d", result.Iterations)
	}
	if result.TotalInputTokens != 100 {
		t.Errorf("expected 100 input tokens, got %d", result.TotalInputTokens)
	}
	if result.TotalOutputTokens != 50 {
		t.Errorf("expected 50 output tokens, got %d", result.TotalOutputTokens)
	}
}

func TestPhaseRunner_Full_LGTM(t *testing.T) {
	sender := &mockSender{
		responses: []*stream.Response{
			// Phase 1: implementation
			textResponse("implemented feature", "end_turn", 200, 100),
			// Phase 2: review returns LGTM
			textResponse("LGTM", "end_turn", 150, 10),
		},
	}

	pr := NewPhaseRunner(sender, DefaultConfig(), "/tmp/test")
	result, err := pr.RunTask(context.Background(), "full", "implement feature", "you are helpful")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sender.callCount != 2 {
		t.Errorf("expected 2 calls, got %d", sender.callCount)
	}
	if result.Text != "LGTM" {
		t.Errorf("expected text 'LGTM', got %q", result.Text)
	}
	// 1 iteration from impl + 1 from review
	if result.Iterations != 2 {
		t.Errorf("expected 2 iterations, got %d", result.Iterations)
	}
	if result.TotalInputTokens != 350 {
		t.Errorf("expected 350 input tokens, got %d", result.TotalInputTokens)
	}
	if result.TotalOutputTokens != 110 {
		t.Errorf("expected 110 output tokens, got %d", result.TotalOutputTokens)
	}
}

func TestPhaseRunner_Full_WithFix(t *testing.T) {
	sender := &mockSender{
		responses: []*stream.Response{
			// Phase 1: implementation
			textResponse("implemented feature", "end_turn", 200, 100),
			// Phase 2: review finds issues
			textResponse("Issue: missing error handling in line 42", "end_turn", 150, 30),
			// Phase 3: fix
			textResponse("fixed error handling", "end_turn", 180, 80),
		},
	}

	pr := NewPhaseRunner(sender, DefaultConfig(), "/tmp/test")
	result, err := pr.RunTask(context.Background(), "full", "implement feature", "you are helpful")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sender.callCount != 3 {
		t.Errorf("expected 3 calls, got %d", sender.callCount)
	}
	if result.Text != "fixed error handling" {
		t.Errorf("expected text 'fixed error handling', got %q", result.Text)
	}
	// 1 impl + 1 review + 1 fix
	if result.Iterations != 3 {
		t.Errorf("expected 3 iterations, got %d", result.Iterations)
	}
	if result.TotalInputTokens != 530 {
		t.Errorf("expected 530 input tokens, got %d", result.TotalInputTokens)
	}
	if result.TotalOutputTokens != 210 {
		t.Errorf("expected 210 output tokens, got %d", result.TotalOutputTokens)
	}
}
