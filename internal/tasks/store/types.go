package store

// TaskDeleted is the OnChange payload for task.deleted events.
type TaskDeleted struct {
	ID int64 `json:"id"`
}

// TaskActivity is the OnChange payload for task.activity events.
type TaskActivity struct {
	TaskID   int64    `json:"taskId"`
	Activity Activity `json:"activity"`
}

// TaskComment is the OnChange payload for task.comment events.
type TaskComment struct {
	TaskID  int64   `json:"taskId"`
	Comment Comment `json:"comment"`
}

// Substep represents a task substep within the active stage.
type Substep string

const (
	SubstepTDD            Substep = "tdd"
	SubstepImplementing   Substep = "implementing"
	SubstepReviewing      Substep = "reviewing"
	SubstepQATest         Substep = "qa_test"
	SubstepE2ETest        Substep = "e2e_test"
	SubstepSecurityReview Substep = "security_review"
)

var substepOrder = []Substep{
	SubstepTDD,
	SubstepImplementing,
	SubstepReviewing,
	SubstepQATest,
	SubstepE2ETest,
	SubstepSecurityReview,
}

// SubstepOrder returns a copy of the ordered substep list.
func SubstepOrder() []Substep {
	out := make([]Substep, len(substepOrder))
	copy(out, substepOrder)
	return out
}

// Next returns the next substep in order, or false if at the end.
func (ss Substep) Next() (Substep, bool) {
	for i, s := range substepOrder {
		if s == ss && i+1 < len(substepOrder) {
			return substepOrder[i+1], true
		}
	}
	return "", false
}

// Valid returns true for valid substeps and the empty string.
func (ss Substep) Valid() bool {
	if ss == "" {
		return true
	}
	for _, s := range substepOrder {
		if s == ss {
			return true
		}
	}
	return false
}
