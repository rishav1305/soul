package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/rishav1305/soul/internal/chat/stream"
)

// retryDelay is the pause between the first and second Claude eval attempt.
// Overridable in tests to avoid slow test runs.
var retryDelay = 3 * time.Second

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
		// First attempt — 30s deadline inside evaluateWithClaude.
		result, err := e.evaluateWithClaude(ctx, questionText, referenceAnswer, userAnswer)
		if err == nil {
			return result, nil
		}
		// Retry once after retryDelay — helps recover from transient rate
		// limits (9 agents share 1 API key; a brief pause often clears the
		// per-minute bucket). Skip retry if the parent context is already done.
		select {
		case <-ctx.Done():
			// parent cancelled — fall through to word-overlap immediately
		case <-time.After(retryDelay):
			if result, err = e.evaluateWithClaude(ctx, questionText, referenceAnswer, userAnswer); err == nil {
				return result, nil
			}
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

	// Bound the Claude call to 30s so a slow or rate-limited API response
	// fails fast and falls back to word-overlap rather than hanging the
	// HTTP handler until the client times out (typically 90s).
	evalCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := e.sender.Send(evalCtx, req)
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

// stopWords is a set of common English words that carry little semantic meaning.
// Filtering them prevents long reference answers with many connectives from
// unfairly penalising correct paraphrases.
var stopWords = map[string]bool{
	"a": true, "an": true, "the": true, "is": true, "are": true, "was": true,
	"were": true, "be": true, "been": true, "being": true, "have": true,
	"has": true, "had": true, "do": true, "does": true, "did": true,
	"to": true, "of": true, "in": true, "for": true, "on": true, "with": true,
	"at": true, "by": true, "from": true, "as": true, "it": true, "its": true,
	"that": true, "this": true, "and": true, "or": true, "but": true, "not": true,
	"can": true, "will": true, "would": true, "could": true, "should": true,
	"may": true, "might": true, "must": true, "shall": true, "which": true,
	"who": true, "what": true, "where": true, "when": true, "how": true,
	"all": true, "each": true, "any": true, "so": true, "if": true, "into": true,
	"than": true, "then": true, "they": true, "their": true, "them": true,
	"there": true, "we": true, "our": true, "us": true, "i": true, "my": true,
	"you": true, "your": true, "he": true, "she": true, "his": true, "her": true,
}

// evaluateWordOverlap computes token-F1 score between the reference and user
// answers after filtering stop words. Token-F1 is the harmonic mean of:
//   - precision = shared_tokens / user_tokens (penalises off-topic verbosity)
//   - recall    = shared_tokens / ref_tokens  (penalises missing key concepts)
//
// This is fairer than recall-only because a correct, concise paraphrase still
// scores highly even when it uses fewer words than the reference.
func (e *Evaluator) evaluateWordOverlap(referenceAnswer, userAnswer string) *Result {
	refWords := toSignificantWordSet(strings.ToLower(referenceAnswer))
	userWords := toSignificantWordSet(strings.ToLower(userAnswer))

	if len(refWords) == 0 {
		return &Result{Correct: true, Score: 100, Quality: 5, Feedback: "No reference answer to compare against."}
	}

	// Collect shared tokens; track which ref words were hit/missed for feedback.
	shared := 0
	var keyHit, keyMissed []string
	for w := range refWords {
		if userWords[w] {
			shared++
			keyHit = append(keyHit, w)
		} else {
			keyMissed = append(keyMissed, w)
		}
	}

	// Token-F1: harmonic mean of precision and recall.
	precision := float64(shared) / float64(len(userWords))
	recall := float64(shared) / float64(len(refWords))

	var f1 float64
	if precision+recall > 0 {
		f1 = 2 * precision * recall / (precision + recall)
	}

	score := f1 * 100
	correct := f1 >= 0.5
	quality := scoreToQuality(score)

	feedback := "Evaluated using token-F1 (Claude unavailable)."
	if correct {
		feedback = fmt.Sprintf("%.0f%% token-F1 match. %s", score, feedback)
	} else {
		feedback = fmt.Sprintf("Only %.0f%% token-F1 match. %s", score, feedback)
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

// normalizeToken applies minimal morphological normalization so that common
// inflected forms match their base tokens:
//   - Strips possessive 's ("go's" → "go")
//   - Strips plural/verb 's' suffix for words > 4 chars that don't end in "ss"
//     ("goroutines" → "goroutine", "threads" → "thread")
//
// This is intentionally simple — no full stemmer — to avoid false matches.
func normalizeToken(w string) string {
	// Strip possessive: "go's" → "go"
	if strings.HasSuffix(w, "'s") && len(w) > 2 {
		return w[:len(w)-2]
	}
	// Strip plain plural 's' for words longer than 4 chars (skip "ss" endings).
	if len(w) > 4 && strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss") {
		return w[:len(w)-1]
	}
	return w
}

// toSignificantWordSet builds a word set from s, excluding stop words and
// normalizing common inflected forms (plurals, possessives).
// Used by evaluateWordOverlap to focus on semantically meaningful tokens.
func toSignificantWordSet(s string) map[string]bool {
	set := make(map[string]bool)
	for _, w := range strings.Fields(s) {
		w = strings.Trim(w, ".,;:!?\"'()[]{}/-")
		w = normalizeToken(w)
		if w != "" && !stopWords[w] {
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
