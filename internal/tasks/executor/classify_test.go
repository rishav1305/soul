package executor

import (
	"testing"
)

func TestClassifyWorkflow(t *testing.T) {
	tests := []struct {
		name        string
		title       string
		description string
		want        string
	}{
		// micro cases
		{name: "add button", title: "add button to header", description: "", want: "micro"},
		{name: "fix typo", title: "fix typo in welcome message", description: "", want: "micro"},
		{name: "change color", title: "change color of primary button", description: "", want: "micro"},
		{name: "rename", title: "rename the submit action", description: "", want: "micro"},
		{name: "add tooltip", title: "add tooltip to info icon", description: "", want: "micro"},
		// quick cases
		{name: "default", title: "update dashboard layout", description: "", want: "quick"},
		{name: "improve error messages", title: "improve error messages", description: "", want: "quick"},
		// full cases
		{name: "refactor", title: "refactor authentication module", description: "", want: "full"},
		{name: "new feature", title: "new feature for user profiles", description: "", want: "full"},
		{name: "add api", title: "add api for notifications", description: "", want: "full"},
		{name: "database", title: "database schema update", description: "", want: "full"},
		{name: "override in desc", title: "update button", description: "this requires a full database migration", want: "full"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyWorkflow(tt.title, tt.description)
			if got != tt.want {
				t.Errorf("ClassifyWorkflow(%q, %q) = %q, want %q", tt.title, tt.description, got, tt.want)
			}
		})
	}
}

func TestIterationLimit(t *testing.T) {
	tests := []struct {
		workflow string
		want     int
	}{
		{workflow: "micro", want: 15},
		{workflow: "quick", want: 30},
		{workflow: "full", want: 40},
		{workflow: "unknown", want: 40},
	}

	for _, tt := range tests {
		t.Run(tt.workflow, func(t *testing.T) {
			got := IterationLimit(tt.workflow)
			if got != tt.want {
				t.Errorf("IterationLimit(%q) = %d, want %d", tt.workflow, got, tt.want)
			}
		})
	}
}
