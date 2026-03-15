package phases

import "testing"

func TestPhaseConfig_Defaults(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.PlanModel != "claude-opus-4-6" {
		t.Errorf("PlanModel = %q, want %q", cfg.PlanModel, "claude-opus-4-6")
	}
	if cfg.ImplModel != "claude-sonnet-4-6" {
		t.Errorf("ImplModel = %q, want %q", cfg.ImplModel, "claude-sonnet-4-6")
	}
	if cfg.ReviewModel != "claude-opus-4-6" {
		t.Errorf("ReviewModel = %q, want %q", cfg.ReviewModel, "claude-opus-4-6")
	}
	if cfg.FixModel != "claude-opus-4-6" {
		t.Errorf("FixModel = %q, want %q", cfg.FixModel, "claude-opus-4-6")
	}
}

func TestMaxIterations(t *testing.T) {
	tests := []struct {
		workflow string
		want     int
	}{
		{"micro", 15},
		{"quick", 30},
		{"full", 40},
		{"", 30},
	}
	for _, tt := range tests {
		if got := MaxIterations(tt.workflow); got != tt.want {
			t.Errorf("MaxIterations(%q) = %d, want %d", tt.workflow, got, tt.want)
		}
	}
}
