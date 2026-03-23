package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

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
