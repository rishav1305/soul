package phases

// PhaseConfig holds model selections for each phase of task execution.
type PhaseConfig struct {
	PlanModel   string
	ImplModel   string
	ReviewModel string
	FixModel    string
}

// DefaultConfig returns the default phase configuration.
func DefaultConfig() PhaseConfig {
	return PhaseConfig{
		PlanModel:   "claude-opus-4-6",
		ImplModel:   "claude-sonnet-4-6",
		ReviewModel: "claude-opus-4-6",
		FixModel:    "claude-opus-4-6",
	}
}

// MaxIterations returns the maximum agent loop iterations for a given workflow.
func MaxIterations(workflow string) int {
	switch workflow {
	case "micro":
		return 15
	case "full":
		return 40
	default: // "quick" and anything else
		return 30
	}
}
