package planner

import (
	"testing"
	"time"
)

func TestStageConstants(t *testing.T) {
	valid := []Stage{
		StageBacklog, StageBrainstorm, StageActive,
		StageBlocked, StageValidation, StageDone,
	}
	for _, s := range valid {
		if !s.Valid() {
			t.Errorf("expected stage %q to be valid", s)
		}
	}
	if Stage("unknown").Valid() {
		t.Error("expected stage 'unknown' to be invalid")
	}
	if Stage("").Valid() {
		t.Error("expected empty stage to be invalid")
	}
}

func TestSubstepConstants(t *testing.T) {
	valid := []Substep{
		SubstepTDD, SubstepImplementing, SubstepReviewing,
		SubstepQATest, SubstepE2ETest, SubstepSecurityReview,
	}
	for _, ss := range valid {
		if !ss.Valid() {
			t.Errorf("expected substep %q to be valid", ss)
		}
	}
	if Substep("unknown").Valid() {
		t.Error("expected substep 'unknown' to be invalid")
	}
}

func TestStageTransitionValid(t *testing.T) {
	cases := []struct {
		from, to Stage
		want     bool
	}{
		// Valid transitions
		{StageBacklog, StageBrainstorm, true},
		{StageBacklog, StageActive, true},
		{StageBrainstorm, StageActive, true},
		{StageActive, StageBlocked, true},
		{StageActive, StageValidation, true},
		{StageBlocked, StageActive, true},
		{StageValidation, StageDone, true},
		{StageValidation, StageActive, true},

		// Invalid transitions
		{StageBacklog, StageDone, false},
		{StageBacklog, StageBlocked, false},
		{StageBrainstorm, StageBacklog, false},
		{StageActive, StageBacklog, false},
		{StageDone, StageBacklog, false},
		{StageDone, StageActive, false},
		{StageBlocked, StageDone, false},
	}
	for _, tc := range cases {
		got := ValidTransition(tc.from, tc.to)
		if got != tc.want {
			t.Errorf("ValidTransition(%q, %q) = %v, want %v", tc.from, tc.to, got, tc.want)
		}
	}
}

func TestNewTask(t *testing.T) {
	tk := NewTask("My Title", "My Description")

	if tk.Title != "My Title" {
		t.Errorf("Title = %q, want %q", tk.Title, "My Title")
	}
	if tk.Description != "My Description" {
		t.Errorf("Description = %q, want %q", tk.Description, "My Description")
	}
	if tk.Stage != StageBacklog {
		t.Errorf("Stage = %q, want %q", tk.Stage, StageBacklog)
	}
	if tk.Source != "manual" {
		t.Errorf("Source = %q, want %q", tk.Source, "manual")
	}
	if tk.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", tk.MaxRetries)
	}
	if tk.CreatedAt == "" {
		t.Error("CreatedAt should be set")
	}
	// Verify it parses as a valid RFC3339 timestamp.
	_, err := time.Parse(time.RFC3339, tk.CreatedAt)
	if err != nil {
		t.Errorf("CreatedAt %q is not valid RFC3339: %v", tk.CreatedAt, err)
	}
}

func TestSubstepNext(t *testing.T) {
	cases := []struct {
		ss      Substep
		want    Substep
		wantOK  bool
	}{
		{SubstepTDD, SubstepImplementing, true},
		{SubstepImplementing, SubstepReviewing, true},
		{SubstepReviewing, SubstepQATest, true},
		{SubstepQATest, SubstepE2ETest, true},
		{SubstepE2ETest, SubstepSecurityReview, true},
		{SubstepSecurityReview, "", false},
		{Substep("invalid"), "", false},
	}
	for _, tc := range cases {
		got, ok := tc.ss.Next()
		if got != tc.want || ok != tc.wantOK {
			t.Errorf("%q.Next() = (%q, %v), want (%q, %v)", tc.ss, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestSubstepIndex(t *testing.T) {
	cases := []struct {
		ss   Substep
		want int
	}{
		{SubstepTDD, 1},
		{SubstepImplementing, 2},
		{SubstepReviewing, 3},
		{SubstepQATest, 4},
		{SubstepE2ETest, 5},
		{SubstepSecurityReview, 6},
		{Substep("invalid"), 0},
	}
	for _, tc := range cases {
		got := tc.ss.Index()
		if got != tc.want {
			t.Errorf("%q.Index() = %d, want %d", tc.ss, got, tc.want)
		}
	}
}
