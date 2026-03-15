package pipelines

import "testing"

func TestValidateTransition_Valid(t *testing.T) {
	tests := []struct {
		pipeline string
		from     string
		to       string
	}{
		{"job", "discovered", "applied"},
		{"job", "discovered", "interview"},
		{"job", "applied", "screening"},
		{"job", "interview", "offer"},
		{"freelance", "found", "proposal-sent"},
		{"freelance", "awarded", "delivering"},
		{"contract", "discovered", "applied"},
		{"consulting", "lead", "discovery-call"},
		{"product-dev", "lead", "scoping"},
	}
	for _, tt := range tests {
		err := ValidateTransition(tt.pipeline, tt.from, tt.to)
		if err != nil {
			t.Errorf("ValidateTransition(%q, %q, %q) = %v, want nil", tt.pipeline, tt.from, tt.to, err)
		}
	}
}

func TestValidateTransition_Terminal(t *testing.T) {
	// Terminal stages should be valid from any stage.
	tests := []struct {
		pipeline string
		from     string
		to       string
	}{
		{"job", "discovered", "rejected"},
		{"job", "interview", "withdrawn"},
		{"freelance", "found", "lost"},
		{"consulting", "negotiating", "declined"},
		{"product-dev", "building", "delivered"},
	}
	for _, tt := range tests {
		err := ValidateTransition(tt.pipeline, tt.from, tt.to)
		if err != nil {
			t.Errorf("ValidateTransition(%q, %q, %q) = %v, want nil (terminal)", tt.pipeline, tt.from, tt.to, err)
		}
	}
}

func TestValidateTransition_Invalid(t *testing.T) {
	tests := []struct {
		pipeline string
		from     string
		to       string
	}{
		{"job", "applied", "discovered"},       // backward
		{"job", "applied", "applied"},           // same stage
		{"freelance", "delivering", "found"},    // backward
		{"unknown", "a", "b"},                   // unknown pipeline
		{"job", "nonexistent", "applied"},       // unknown from
		{"job", "discovered", "nonexistent"},    // unknown to
	}
	for _, tt := range tests {
		err := ValidateTransition(tt.pipeline, tt.from, tt.to)
		if err == nil {
			t.Errorf("ValidateTransition(%q, %q, %q) = nil, want error", tt.pipeline, tt.from, tt.to)
		}
	}
}

func TestDefaultStage(t *testing.T) {
	tests := []struct {
		pipeline string
		want     string
	}{
		{"job", "discovered"},
		{"freelance", "found"},
		{"contract", "discovered"},
		{"consulting", "lead"},
		{"product-dev", "lead"},
		{"unknown", ""},
	}
	for _, tt := range tests {
		got := DefaultStage(tt.pipeline)
		if got != tt.want {
			t.Errorf("DefaultStage(%q) = %q, want %q", tt.pipeline, got, tt.want)
		}
	}
}

func TestIsTerminal(t *testing.T) {
	tests := []struct {
		pipeline string
		stage    string
		want     bool
	}{
		{"job", "joined", true},
		{"job", "rejected", true},
		{"job", "withdrawn", true},
		{"job", "discovered", false},
		{"job", "interview", false},
		{"freelance", "completed", true},
		{"freelance", "found", false},
		{"consulting", "delivered", true},
		{"consulting", "lead", false},
		{"unknown", "anything", false},
	}
	for _, tt := range tests {
		got := IsTerminal(tt.pipeline, tt.stage)
		if got != tt.want {
			t.Errorf("IsTerminal(%q, %q) = %v, want %v", tt.pipeline, tt.stage, got, tt.want)
		}
	}
}
