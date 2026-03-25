package modules

import (
	"github.com/rishav1305/soul/internal/tutor/eval"
	"github.com/rishav1305/soul/internal/tutor/store"
)

// ToolResult is the standard return type for all module methods.
type ToolResult struct {
	Summary string      `json:"summary"`
	Data    interface{} `json:"data"`
}

// Registry holds references to all tutor modules.
type Registry struct {
	Store        *store.Store
	ContentDir   string
	DSA          *DSAModule
	AI           *AIModule
	Behavioral   *BehavioralModule
	Mock         *MockModule
	Planner      *PlannerModule
	Progress     *ProgressModule
	SystemDesign *SystemDesignModule
}

// NewRegistry creates a Registry with all modules initialized.
// evaluator may be nil — modules will fall back to word-overlap scoring.
func NewRegistry(s *store.Store, contentDir string, evaluator *eval.Evaluator) *Registry {
	return &Registry{
		Store:        s,
		ContentDir:   contentDir,
		DSA:          &DSAModule{store: s, contentDir: contentDir, evaluator: evaluator},
		AI:           &AIModule{store: s, contentDir: contentDir, evaluator: evaluator},
		Behavioral:   &BehavioralModule{store: s},
		Mock:         &MockModule{store: s},
		Planner:      &PlannerModule{store: s},
		Progress:     &ProgressModule{store: s},
		SystemDesign: &SystemDesignModule{store: s, evaluator: evaluator},
	}
}
