package pipelines

import "fmt"

// Pipeline defines a lead pipeline with ordered stages and terminal states.
type Pipeline struct {
	Stages   []string
	Terminal []string
}

// Pipelines defines the 5 supported pipeline types.
var Pipelines = map[string]Pipeline{
	"job":         {Stages: []string{"discovered", "applied", "screening", "interview", "offer", "joined"}, Terminal: []string{"joined", "rejected", "withdrawn"}},
	"freelance":   {Stages: []string{"found", "proposal-sent", "shortlisted", "awarded", "delivering", "completed"}, Terminal: []string{"completed", "lost", "withdrawn"}},
	"contract":    {Stages: []string{"discovered", "applied", "screening", "interview", "offer", "engaged", "completed"}, Terminal: []string{"completed", "rejected", "withdrawn"}},
	"consulting":  {Stages: []string{"lead", "discovery-call", "proposal-sent", "negotiating", "engaged", "delivered"}, Terminal: []string{"delivered", "lost", "declined"}},
	"product-dev": {Stages: []string{"lead", "scoping", "proposal-sent", "negotiating", "building", "delivered"}, Terminal: []string{"delivered", "lost", "declined"}},
}

// ValidateTransition checks whether moving from fromStage to toStage is valid
// within the given pipeline type. A valid transition moves forward in the stage
// list, or moves to a terminal stage from any position.
func ValidateTransition(pipelineType, fromStage, toStage string) error {
	p, ok := Pipelines[pipelineType]
	if !ok {
		return fmt.Errorf("pipelines: unknown pipeline type: %q", pipelineType)
	}

	// Terminal stages are always valid targets.
	for _, ts := range p.Terminal {
		if toStage == ts {
			return nil
		}
	}

	// Find indices in the stage list.
	fromIdx := -1
	toIdx := -1
	for i, s := range p.Stages {
		if s == fromStage {
			fromIdx = i
		}
		if s == toStage {
			toIdx = i
		}
	}

	if fromIdx == -1 {
		return fmt.Errorf("pipelines: unknown from stage %q in pipeline %q", fromStage, pipelineType)
	}
	if toIdx == -1 {
		return fmt.Errorf("pipelines: unknown to stage %q in pipeline %q", toStage, pipelineType)
	}
	if toIdx <= fromIdx {
		return fmt.Errorf("pipelines: cannot move backward from %q to %q in pipeline %q", fromStage, toStage, pipelineType)
	}

	return nil
}

// DefaultStage returns the first stage for the given pipeline type.
func DefaultStage(pipelineType string) string {
	p, ok := Pipelines[pipelineType]
	if !ok {
		return ""
	}
	return p.Stages[0]
}

// IsTerminal reports whether the given stage is a terminal stage for the pipeline.
func IsTerminal(pipelineType, stage string) bool {
	p, ok := Pipelines[pipelineType]
	if !ok {
		return false
	}
	for _, ts := range p.Terminal {
		if stage == ts {
			return true
		}
	}
	return false
}
