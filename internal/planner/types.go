package planner

import (
	"time"
)

// Stage represents the lifecycle stage of a task.
type Stage string

const (
	StageBacklog    Stage = "backlog"
	StageBrainstorm Stage = "brainstorm"
	StageActive     Stage = "active"
	StageBlocked    Stage = "blocked"
	StageValidation Stage = "validation"
	StageDone       Stage = "done"
)

// Valid reports whether s is a recognised stage.
func (s Stage) Valid() bool {
	switch s {
	case StageBacklog, StageBrainstorm, StageActive, StageBlocked, StageValidation, StageDone:
		return true
	}
	return false
}

// validTransitions maps each stage to the set of stages it may transition to.
var validTransitions = map[Stage]map[Stage]bool{
	StageBacklog:    {StageBrainstorm: true, StageActive: true},
	StageBrainstorm: {StageActive: true},
	StageActive:     {StageBlocked: true, StageValidation: true},
	StageBlocked:    {StageActive: true},
	StageValidation: {StageDone: true, StageActive: true},
}

// ValidTransition reports whether moving from one stage to another is allowed.
func ValidTransition(from, to Stage) bool {
	targets, ok := validTransitions[from]
	if !ok {
		return false
	}
	return targets[to]
}

// Substep represents the current sub-step within the active stage.
type Substep string

const (
	SubstepTDD            Substep = "tdd"
	SubstepImplementing   Substep = "implementing"
	SubstepReviewing      Substep = "reviewing"
	SubstepQATest         Substep = "qa_test"
	SubstepE2ETest        Substep = "e2e_test"
	SubstepSecurityReview Substep = "security_review"
)

// substepOrder defines the canonical ordering of substeps.
var substepOrder = []Substep{
	SubstepTDD,
	SubstepImplementing,
	SubstepReviewing,
	SubstepQATest,
	SubstepE2ETest,
	SubstepSecurityReview,
}

// Valid reports whether ss is a recognised substep.
func (ss Substep) Valid() bool {
	for _, s := range substepOrder {
		if ss == s {
			return true
		}
	}
	return false
}

// Index returns the 1-based position of the substep in the canonical order.
// It returns 0 if the substep is not valid.
func (ss Substep) Index() int {
	for i, s := range substepOrder {
		if ss == s {
			return i + 1
		}
	}
	return 0
}

// Next returns the next substep in the canonical order and true.
// If ss is the last substep or not valid, it returns "" and false.
func (ss Substep) Next() (Substep, bool) {
	idx := ss.Index()
	if idx == 0 || idx >= len(substepOrder) {
		return "", false
	}
	return substepOrder[idx], true
}

// Task represents a planner task.
type Task struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Acceptance  string `json:"acceptance"`

	Stage   Stage   `json:"stage"`
	Substep Substep `json:"substep"`

	Priority int    `json:"priority"`
	Source   string `json:"source"`
	Blocker  string `json:"blocker"`
	Plan     string `json:"plan"`
	Output   string `json:"output"`
	Error    string `json:"error"`

	AgentID  string `json:"agent_id"`
	Product  string `json:"product"`
	ParentID *int64 `json:"parent_id,omitempty"`

	Metadata string `json:"metadata"`

	RetryCount int `json:"retry_count"`
	MaxRetries int `json:"max_retries"`

	CreatedAt   string `json:"created_at"`
	StartedAt   string `json:"started_at"`
	CompletedAt string `json:"completed_at"`
}

// NewTask creates a task with sensible defaults.
func NewTask(title, description string) Task {
	return Task{
		Title:       title,
		Description: description,
		Stage:       StageBacklog,
		Source:      "manual",
		MaxRetries:  3,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
}

// TaskFilter is used to narrow List results.
type TaskFilter struct {
	Stage   Stage  `json:"stage,omitempty"`
	Product string `json:"product,omitempty"`
}

// TaskUpdate carries optional partial-update fields (nil means "don't change").
type TaskUpdate struct {
	Title       *string  `json:"title,omitempty"`
	Description *string  `json:"description,omitempty"`
	Acceptance  *string  `json:"acceptance,omitempty"`
	Stage       *Stage   `json:"stage,omitempty"`
	Substep     *Substep `json:"substep,omitempty"`
	Priority    *int     `json:"priority,omitempty"`
	Source      *string  `json:"source,omitempty"`
	Blocker     *string  `json:"blocker,omitempty"`
	Plan        *string  `json:"plan,omitempty"`
	Output      *string  `json:"output,omitempty"`
	Error       *string  `json:"error,omitempty"`
	AgentID     *string  `json:"agent_id,omitempty"`
	Product     *string  `json:"product,omitempty"`
	ParentID    *int64   `json:"parent_id,omitempty"`
	Metadata    *string  `json:"metadata,omitempty"`
	RetryCount  *int     `json:"retry_count,omitempty"`
	MaxRetries  *int     `json:"max_retries,omitempty"`
	StartedAt   *string  `json:"started_at,omitempty"`
	CompletedAt *string  `json:"completed_at,omitempty"`
}

// Comment represents a comment on a task.
type Comment struct {
	ID          int64    `json:"id"`
	TaskID      int64    `json:"task_id"`
	Author      string   `json:"author"`      // "user" or "soul"
	Type        string   `json:"type"`         // "feedback", "status", "verification", "error"
	Body        string   `json:"body"`
	Attachments []string `json:"attachments"`  // MinIO keys
	CreatedAt   string   `json:"created_at"`
}
