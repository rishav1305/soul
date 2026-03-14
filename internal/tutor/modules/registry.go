package modules

import "github.com/rishav1305/soul-v2/internal/tutor/store"

// ToolResult is the standard return type for all module methods.
type ToolResult struct {
	Summary string      `json:"summary"`
	Data    interface{} `json:"data"`
}

// Registry holds references to all tutor modules.
type Registry struct {
	Store      *store.Store
	ContentDir string
	DSA        *DSAModule
	AI         *AIModule
	Behavioral *BehavioralModule
	Mock       *MockModule
	Planner    *PlannerModule
	Progress   *ProgressModule
}

// NewRegistry creates a Registry with all modules initialized.
func NewRegistry(s *store.Store, contentDir string) *Registry {
	return &Registry{
		Store:      s,
		ContentDir: contentDir,
		DSA:        &DSAModule{store: s, contentDir: contentDir},
		AI:         &AIModule{store: s, contentDir: contentDir},
		Behavioral: &BehavioralModule{store: s},
		Mock:       &MockModule{store: s},
		Planner:    &PlannerModule{store: s},
		Progress:   &ProgressModule{store: s},
	}
}
