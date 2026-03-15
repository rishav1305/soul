package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/rishav1305/soul-v2/internal/chat/stream"
	"github.com/rishav1305/soul-v2/internal/sentinel/store"
)

// Sender abstracts the Claude API for non-streaming requests.
type Sender interface {
	Send(ctx context.Context, req *stream.Request) (*stream.Response, error)
}

// Session tracks in-memory conversation state for a challenge attempt.
type Session struct {
	SystemPrompt string
	History      []stream.Message
	HintsUsed   int
}

// Engine orchestrates CTF challenge interactions via the Claude API.
type Engine struct {
	store    *store.Store
	sender   Sender
	sessions map[string]*Session // keyed by challenge ID
	mu       sync.Mutex
	model    string
}

// New creates a new Engine with the given store and sender.
func New(s *store.Store, sender Sender) *Engine {
	return &Engine{
		store:    s,
		sender:   sender,
		sessions: make(map[string]*Session),
		model:    stream.DefaultModel,
	}
}

// StartSession creates or resets an in-memory session for the given challenge.
// Returns the challenge metadata.
func (e *Engine) StartSession(challengeID string, reset bool) (*store.Challenge, error) {
	challenge, err := e.store.GetChallenge(challengeID)
	if err != nil {
		return nil, err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	existing, ok := e.sessions[challengeID]
	if !ok || reset {
		e.sessions[challengeID] = &Session{
			SystemPrompt: challenge.SystemPrompt,
			History:      nil,
			HintsUsed:    0,
		}
	} else {
		_ = existing // keep existing session
	}

	return challenge, nil
}

// AttackChallenge sends a user payload to the challenge's Claude session.
// Returns (response text, turn count, error).
func (e *Engine) AttackChallenge(challengeID, payload string) (string, int, error) {
	e.mu.Lock()
	sess, ok := e.sessions[challengeID]
	if !ok {
		e.mu.Unlock()
		return "", 0, fmt.Errorf("no active session for challenge %s — call StartSession first", challengeID)
	}

	// Append user message.
	sess.History = append(sess.History, stream.Message{
		Role: "user",
		Content: []stream.ContentBlock{
			{Type: "text", Text: payload},
		},
	})

	// Build request.
	req := &stream.Request{
		MaxTokens: 2048,
		System:    sess.SystemPrompt,
		Messages:  copyMessages(sess.History),
	}
	e.mu.Unlock()

	// Call Claude (outside lock).
	resp, err := e.sender.Send(context.Background(), req)
	if err != nil {
		return "", 0, fmt.Errorf("claude api: %w", err)
	}

	// Extract text response.
	respText := extractText(resp)

	// Update session with assistant response.
	e.mu.Lock()
	sess.History = append(sess.History, stream.Message{
		Role: "assistant",
		Content: []stream.ContentBlock{
			{Type: "text", Text: respText},
		},
	})
	turnCount := len(sess.History) / 2 // each turn = user + assistant
	e.mu.Unlock()

	// Record attempt in store (best effort).
	_, _ = e.store.RecordAttempt(challengeID, payload, respText, false)

	return respText, turnCount, nil
}

// SubmitFlag checks a flag submission against the challenge's correct flag.
// Returns (points earned, correct, error).
// Scoring: base + bonus(if turns <= maxTurns) - hints*5, minimum base/2.
func (e *Engine) SubmitFlag(challengeID, flag string) (int, bool, error) {
	challenge, err := e.store.GetChallenge(challengeID)
	if err != nil {
		return 0, false, err
	}

	if flag != challenge.Flag {
		return 0, false, nil
	}

	// Already completed — idempotent.
	completed, err := e.store.IsCompleted(challengeID)
	if err != nil {
		return 0, false, err
	}
	if completed {
		progress, err := e.store.GetProgress()
		if err != nil {
			return 0, true, nil
		}
		return progress[challengeID], true, nil
	}

	e.mu.Lock()
	sess := e.sessions[challengeID]
	hintsUsed := 0
	turnsUsed := 0
	if sess != nil {
		hintsUsed = sess.HintsUsed
		turnsUsed = len(sess.History) / 2
	}
	e.mu.Unlock()

	// Calculate points.
	base := challenge.Points
	bonus := 0
	if turnsUsed > 0 && turnsUsed <= challenge.MaxTurns/2 {
		bonus = base / 2 // 50% bonus for fast solve
	}
	points := base + bonus - hintsUsed*5
	minPoints := base / 2
	if points < minPoints {
		points = minPoints
	}

	// Record completion.
	if err := e.store.RecordCompletion(challengeID, points, turnsUsed, hintsUsed); err != nil {
		return 0, false, err
	}

	// Record the successful attempt.
	_, _ = e.store.RecordAttempt(challengeID, "flag:"+flag, "correct", true)

	// Clear session.
	e.mu.Lock()
	delete(e.sessions, challengeID)
	e.mu.Unlock()

	return points, true, nil
}

// AttackSandbox sends a payload to the default sandbox configuration.
func (e *Engine) AttackSandbox(payload string) (string, error) {
	cfg, err := e.store.GetDefaultSandboxConfig()
	if err != nil {
		return "", fmt.Errorf("get sandbox config: %w", err)
	}

	var sandboxCfg SandboxConfig
	if cfg != nil {
		sandboxCfg = SandboxConfig{
			Name:          cfg.Name,
			SystemPrompt:  cfg.SystemPrompt,
			Guardrails:    parseGuardrails(cfg.GuardrailsJSON),
			WeaknessLevel: cfg.WeaknessLevel,
		}
	} else {
		sandboxCfg = DefaultSandboxConfig()
	}

	systemPrompt := BuildSandboxPrompt(sandboxCfg)

	req := &stream.Request{
		MaxTokens: 2048,
		System:    systemPrompt,
		Messages: []stream.Message{
			{
				Role: "user",
				Content: []stream.ContentBlock{
					{Type: "text", Text: payload},
				},
			},
		},
	}

	resp, err := e.sender.Send(context.Background(), req)
	if err != nil {
		return "", fmt.Errorf("claude api: %w", err)
	}

	return extractText(resp), nil
}

// UseHint returns the next available hint for a challenge and increments the hint counter.
func (e *Engine) UseHint(challengeID string) (string, error) {
	challenge, err := e.store.GetChallenge(challengeID)
	if err != nil {
		return "", err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	sess, ok := e.sessions[challengeID]
	if !ok {
		return "", fmt.Errorf("no active session for challenge %s — call StartSession first", challengeID)
	}

	if sess.HintsUsed >= len(challenge.Hints) {
		return "No more hints available.", nil
	}

	hint := challenge.Hints[sess.HintsUsed]
	sess.HintsUsed++
	return hint, nil
}

// copyMessages returns a shallow copy of the message slice.
func copyMessages(msgs []stream.Message) []stream.Message {
	cp := make([]stream.Message, len(msgs))
	copy(cp, msgs)
	return cp
}

// extractText concatenates text blocks from a response.
func extractText(resp *stream.Response) string {
	var text string
	for _, b := range resp.Content {
		if b.Type == "text" {
			text += b.Text
		}
	}
	return text
}

// parseGuardrails splits a JSON string array into a Go slice.
func parseGuardrails(jsonStr string) []string {
	if jsonStr == "" || jsonStr == "[]" {
		return nil
	}
	var result []string
	// Simple JSON array parse — guardrails are stored as JSON string arrays.
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return []string{jsonStr}
	}
	return result
}
