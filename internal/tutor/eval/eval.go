package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rishav1305/soul-v2/internal/chat/stream"
)

// Sender sends a non-streaming request to Claude and returns the response.
type Sender interface {
	Send(ctx context.Context, req *stream.Request) (*stream.Response, error)
}

// Result holds the evaluation outcome.
type Result struct {
	Correct   bool     `json:"correct"`
	Score     float64  `json:"score"`
	Quality   int      `json:"quality"`
	Feedback  string   `json:"feedback"`
	KeyMissed []string `json:"keyMissed"`
	KeyHit    []string `json:"keyHit"`
}

// Evaluator evaluates answers using Claude, with word-overlap fallback.
type Evaluator struct {
	sender Sender
}

// New creates a new Evaluator. sender may be nil (uses fallback only).
func New(sender Sender) *Evaluator {
	return &Evaluator{sender: sender}
}

const evalSystemPrompt = `You are an expert technical interviewer evaluating a candidate's answer.
Given the reference answer and candidate's response, evaluate on:
1. Correctness of core concepts
2. Completeness (key points covered)
3. Technical accuracy of details
4. For Python questions: idiomatic Python usage

Return ONLY valid JSON with this exact schema:
{"correct": bool, "score": 0-100, "quality": 0-5, "feedback": "2-3 sentences", "keyMissed": ["concept1"], "keyHit": ["concept1"]}

Quality mapping: 5=perfect, 4=correct with gaps, 3=barely correct, 2=incorrect but close, 1=completely wrong, 0=no attempt/blank.
Score is granular 0-100: partial credit for partial answers.`

// Evaluate assesses a user's answer against a reference answer.
func (e *Evaluator) Evaluate(ctx context.Context, questionText, referenceAnswer, userAnswer string) (*Result, error) {
	trimmed := strings.TrimSpace(userAnswer)
	if trimmed == "" || strings.EqualFold(trimmed, "skip") || strings.EqualFold(trimmed, "idk") || strings.EqualFold(trimmed, "i don't know") {
		return &Result{
			Correct:   false,
			Score:     0,
			Quality:   0,
			Feedback:  "No answer provided.",
			KeyMissed: []string{"entire answer"},
			KeyHit:    []string{},
		}, nil
	}

	if e.sender != nil {
		result, err := e.evaluateWithClaude(ctx, questionText, referenceAnswer, userAnswer)
		if err == nil {
			return result, nil
		}
	}

	return e.evaluateWordOverlap(referenceAnswer, userAnswer), nil
}

func (e *Evaluator) evaluateWithClaude(ctx context.Context, questionText, referenceAnswer, userAnswer string) (*Result, error) {
	userMsg := fmt.Sprintf("Question:\n%s\n\nReference Answer:\n%s\n\nCandidate's Answer:\n%s", questionText, referenceAnswer, userAnswer)

	// Model is left empty so the stream client uses its default (Haiku).
	// Sonnet is not accessible via OAuth beta — leaving Model unset avoids 400s.
	req := &stream.Request{
		MaxTokens: 1024,
		System:    evalSystemPrompt,
		Messages: []stream.Message{
			{Role: "user", Content: []stream.ContentBlock{{Type: "text", Text: userMsg}}},
		},
	}

	resp, err := e.sender.Send(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("eval: claude send: %w", err)
	}

	var text string
	for _, block := range resp.Content {
		if block.Type == "text" {
			text = block.Text
			break
		}
	}
	if text == "" {
		return nil, fmt.Errorf("eval: no text in response")
	}

	text = extractJSON(text)

	var result Result
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, fmt.Errorf("eval: parse response: %w", err)
	}

	if result.Score < 0 {
		result.Score = 0
	}
	if result.Score > 100 {
		result.Score = 100
	}
	if result.Quality < 0 {
		result.Quality = 0
	}
	if result.Quality > 5 {
		result.Quality = 5
	}

	return &result, nil
}

func (e *Evaluator) evaluateWordOverlap(referenceAnswer, userAnswer string) *Result {
	refWords := toWordSet(strings.ToLower(referenceAnswer))
	userWords := toWordSet(strings.ToLower(userAnswer))

	if len(refWords) == 0 {
		return &Result{Correct: true, Score: 100, Quality: 5, Feedback: "No reference answer to compare against."}
	}

	overlap := 0
	var keyHit, keyMissed []string
	for w := range refWords {
		if userWords[w] {
			overlap++
			keyHit = append(keyHit, w)
		} else {
			keyMissed = append(keyMissed, w)
		}
	}

	ratio := float64(overlap) / float64(len(refWords))
	score := ratio * 100
	correct := ratio >= 0.5
	quality := scoreToQuality(score)

	feedback := "Evaluated using word overlap (Claude unavailable)."
	if correct {
		feedback = fmt.Sprintf("%.0f%% keyword match. %s", score, feedback)
	} else {
		feedback = fmt.Sprintf("Only %.0f%% keyword match. %s", score, feedback)
	}

	return &Result{
		Correct:   correct,
		Score:     score,
		Quality:   quality,
		Feedback:  feedback,
		KeyMissed: keyMissed,
		KeyHit:    keyHit,
	}
}

func scoreToQuality(score float64) int {
	switch {
	case score >= 90:
		return 5
	case score >= 70:
		return 4
	case score >= 50:
		return 3
	case score >= 30:
		return 2
	case score > 0:
		return 1
	default:
		return 0
	}
}

func toWordSet(s string) map[string]bool {
	set := make(map[string]bool)
	for _, w := range strings.Fields(s) {
		w = strings.Trim(w, ".,;:!?\"'()[]{}/-")
		if w != "" {
			set[w] = true
		}
	}
	return set
}

// extractJSON tries to extract JSON from a response that may contain markdown code fences.
func extractJSON(text string) string {
	// Try to find JSON in code fences first
	if start := indexOf(text, "```json"); start >= 0 {
		text = text[start+7:]
		if end := indexOf(text, "```"); end >= 0 {
			return strings.TrimSpace(text[:end])
		}
	}
	if start := indexOf(text, "```"); start >= 0 {
		text = text[start+3:]
		if end := indexOf(text, "```"); end >= 0 {
			return strings.TrimSpace(text[:end])
		}
	}
	return strings.TrimSpace(text)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
