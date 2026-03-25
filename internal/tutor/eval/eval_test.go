package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rishav1305/soul-v2/internal/chat/stream"
)

type mockSender struct {
	response *stream.Response
	err      error
	gotReq   *stream.Request
}

func (m *mockSender) Send(ctx context.Context, req *stream.Request) (*stream.Response, error) {
	m.gotReq = req
	return m.response, m.err
}

// slowSender blocks until ctx is cancelled — simulates a slow or hung API call.
type slowSender struct {
	done chan struct{}
}

func (s *slowSender) Send(ctx context.Context, req *stream.Request) (*stream.Response, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.done:
		return nil, fmt.Errorf("done channel closed")
	}
}

func TestEvaluateUsesDefaultModel(t *testing.T) {
	// eval.go must NOT hardcode a model so the stream client uses its
	// OAuth-accessible default (Haiku). Sonnet is blocked via OAuth beta.
	expected := Result{Correct: true, Score: 80, Quality: 4, Feedback: "Good."}
	respJSON, _ := json.Marshal(expected)

	sender := &mockSender{
		response: &stream.Response{
			Content: []stream.ContentBlock{{Type: "text", Text: string(respJSON)}},
		},
	}
	e := New(sender)
	_, err := e.Evaluate(context.Background(), "What is a goroutine?", "A goroutine is a lightweight thread.", "A goroutine is a lightweight thread managed by Go runtime.")
	if err != nil {
		t.Fatal(err)
	}
	if sender.gotReq == nil {
		t.Fatal("expected Send to be called")
	}
	if sender.gotReq.Model != "" {
		t.Errorf("eval must not set Model field (got %q) — leave empty for stream client default (Haiku)", sender.gotReq.Model)
	}
}

func TestEvaluateBlankAnswer(t *testing.T) {
	e := New(nil)
	result, err := e.Evaluate(context.Background(), "What is X?", "X is Y", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Quality != 0 {
		t.Errorf("expected quality=0 for blank, got %d", result.Quality)
	}
	if result.Score != 0 {
		t.Errorf("expected score=0 for blank, got %.0f", result.Score)
	}
}

func TestEvaluateSkipAnswer(t *testing.T) {
	e := New(nil)
	for _, skip := range []string{"skip", "idk", "I don't know", "  SKIP  "} {
		result, err := e.Evaluate(context.Background(), "Q?", "A", skip)
		if err != nil {
			t.Fatal(err)
		}
		if result.Quality != 0 {
			t.Errorf("expected quality=0 for %q, got %d", skip, result.Quality)
		}
	}
}

func TestEvaluateWordOverlapFallback(t *testing.T) {
	e := New(nil)
	result, err := e.Evaluate(context.Background(),
		"What is a hash map?",
		"A hash map is a data structure that maps keys to values using a hash function for O(1) average lookup",
		"A hash map uses a hash function to map keys to values with constant time lookup",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Score == 0 {
		t.Error("expected non-zero score for overlapping answer")
	}
	if result.Feedback == "" {
		t.Error("expected feedback")
	}
}

func TestEvaluateWithClaude(t *testing.T) {
	expected := Result{
		Correct:   true,
		Score:     85,
		Quality:   4,
		Feedback:  "Good answer covering key concepts.",
		KeyMissed: []string{"hash function"},
		KeyHit:    []string{"O(1) lookup", "key-value pairs"},
	}
	respJSON, _ := json.Marshal(expected)

	sender := &mockSender{
		response: &stream.Response{
			Content: []stream.ContentBlock{
				{Type: "text", Text: string(respJSON)},
			},
		},
	}

	e := New(sender)
	result, err := e.Evaluate(context.Background(),
		"What is a hash map?",
		"A hash map is a data structure...",
		"A hash map stores key-value pairs...",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Score != 85 {
		t.Errorf("expected score=85, got %.0f", result.Score)
	}
	if result.Quality != 4 {
		t.Errorf("expected quality=4, got %d", result.Quality)
	}
}

func TestEvaluateClaudeFailsFallsBack(t *testing.T) {
	sender := &mockSender{err: fmt.Errorf("network error")}
	e := New(sender)

	result, err := e.Evaluate(context.Background(),
		"What is X?",
		"X is a data structure for efficient lookup",
		"X provides efficient lookup using hashing",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Feedback == "" {
		t.Error("expected fallback feedback")
	}
}

func TestEvaluateClaudeMalformedJSON(t *testing.T) {
	sender := &mockSender{
		response: &stream.Response{
			Content: []stream.ContentBlock{
				{Type: "text", Text: "not valid json at all"},
			},
		},
	}
	e := New(sender)

	result, err := e.Evaluate(context.Background(), "Q?", "A", "my answer")
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Error("expected fallback result, got nil")
	}
}

func TestScoreToQuality(t *testing.T) {
	tests := []struct {
		score   float64
		quality int
	}{
		{100, 5}, {90, 5}, {85, 4}, {70, 4},
		{60, 3}, {50, 3}, {40, 2}, {30, 2},
		{20, 1}, {1, 1}, {0, 0},
	}
	for _, tt := range tests {
		got := scoreToQuality(tt.score)
		if got != tt.quality {
			t.Errorf("scoreToQuality(%.0f) = %d, want %d", tt.score, got, tt.quality)
		}
	}
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain JSON",
			input:    `{"score": 85}`,
			expected: `{"score": 85}`,
		},
		{
			name:     "JSON in code fence",
			input:    "```json\n{\"score\": 85}\n```",
			expected: `{"score": 85}`,
		},
		{
			name:     "JSON in plain code fence",
			input:    "```\n{\"score\": 85}\n```",
			expected: `{"score": 85}`,
		},
		{
			name:     "JSON with surrounding text",
			input:    "Here is the result:\n```json\n{\"score\": 85}\n```\nEnd.",
			expected: `{"score": 85}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSON(tt.input)
			if got != tt.expected {
				t.Errorf("extractJSON() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestTokenF1FairerThanRecall demonstrates the core bug fix:
// the old recall-only metric would score a correct paraphrase as 0% because
// it used different words. The new token-F1 metric scores it correctly.
func TestTokenF1FairerThanRecall(t *testing.T) {
	e := New(nil) // nil sender → word-overlap path
	// Reference is verbose with stop words. User answer is a correct paraphrase.
	result, err := e.Evaluate(context.Background(),
		"What does a goroutine do?",
		"A goroutine is a lightweight thread of execution managed by the Go runtime scheduler.",
		"Goroutines are lightweight threads managed by Go's runtime.",
	)
	if err != nil {
		t.Fatal(err)
	}
	// With the old recall-only metric this would score ~20% (misses "a", "is", "of", "by", "the" etc.)
	// With token F1 + stop-word filtering it should score above 50.
	if result.Score < 50 {
		t.Errorf("expected score >= 50 for a correct paraphrase, got %.1f — check token F1 and stop-word filtering", result.Score)
	}
	if !result.Correct {
		t.Errorf("expected correct=true for a correct paraphrase, got false (score=%.1f)", result.Score)
	}
}

func TestStopWordsNotPenalised(t *testing.T) {
	// A reference packed with stop words should not drag down the score.
	// "The key is that a hash map is a data structure that maps a key to a value."
	// User says "hash map maps keys to values" — highly overlapping on meaningful words.
	e := New(nil)
	result, err := e.Evaluate(context.Background(),
		"What is a hash map?",
		"The key is that a hash map is a data structure that maps a key to a value efficiently.",
		"A hash map is a data structure that maps keys to values.",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Score < 60 {
		t.Errorf("expected score >= 60 when stop words are filtered, got %.1f", result.Score)
	}
}

// TestEvaluateClaudeTimeoutFallsBack verifies that when the Claude API is slow
// (simulated by a blocking sender), the 30s internal timeout fires and the
// evaluator falls back to word-overlap rather than blocking the caller.
// We pass a short context so the test runs in milliseconds, not 30s.
// context.WithTimeout(ctx, 30s) adopts the shorter of the two deadlines.
func TestEvaluateClaudeTimeoutFallsBack(t *testing.T) {
	done := make(chan struct{})
	defer close(done)

	e := New(&slowSender{done: done})

	// 50ms deadline — much shorter than the 30s eval timeout — ensures the
	// blocking sender is cancelled quickly for a fast test.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	result, err := e.Evaluate(ctx,
		"What is a goroutine?",
		"A goroutine is a lightweight concurrent thread managed by the Go runtime.",
		"A goroutine is a lightweight concurrent thread managed by Go.",
	)
	if err != nil {
		t.Fatalf("expected fallback result, not error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result from word-overlap fallback")
	}
	// Word-overlap fallback includes "Claude unavailable" in the feedback.
	if !strings.Contains(result.Feedback, "Claude unavailable") {
		t.Errorf("expected word-overlap fallback feedback, got: %q", result.Feedback)
	}
}

func TestEvaluateNoSenderNilResult(t *testing.T) {
	// With nil sender and a valid (non-skip) answer, must return word-overlap result, never nil.
	e := New(nil)
	result, err := e.Evaluate(context.Background(), "Q?", "reference answer here", "some answer here")
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestEvaluateScoreClamping(t *testing.T) {
	// Claude returns out-of-range values; they should be clamped.
	outOfRange := Result{
		Correct:  true,
		Score:    150,
		Quality:  9,
		Feedback: "Clamping test.",
	}
	respJSON, _ := json.Marshal(outOfRange)

	sender := &mockSender{
		response: &stream.Response{
			Content: []stream.ContentBlock{
				{Type: "text", Text: string(respJSON)},
			},
		},
	}

	e := New(sender)
	result, err := e.Evaluate(context.Background(), "Q?", "ref", "answer")
	if err != nil {
		t.Fatal(err)
	}
	if result.Score != 100 {
		t.Errorf("expected score clamped to 100, got %.0f", result.Score)
	}
	if result.Quality != 5 {
		t.Errorf("expected quality clamped to 5, got %d", result.Quality)
	}
}
