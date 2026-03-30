package store

import "testing"

func TestAddOptimization(t *testing.T) {
	s := newTestStore(t)
	opt := Optimization{
		Platform: "linkedin",
		Section:  "headline",
		Field:    "title",
		Previous: "Software Engineer",
		Updated:  "Senior AI Architect",
	}
	id, err := s.AddOptimization(opt)
	if err != nil {
		t.Fatalf("AddOptimization: %v", err)
	}
	if id <= 0 {
		t.Errorf("expected positive id, got %d", id)
	}
}

func TestAddOptimization_DefaultFields(t *testing.T) {
	s := newTestStore(t)
	id, err := s.AddOptimization(Optimization{Platform: "github"})
	if err != nil {
		t.Fatalf("AddOptimization: %v", err)
	}
	if id <= 0 {
		t.Errorf("expected positive id, got %d", id)
	}
}

func TestListOptimizations(t *testing.T) {
	s := newTestStore(t)
	s.AddOptimization(Optimization{Platform: "linkedin", Section: "headline"})
	s.AddOptimization(Optimization{Platform: "github", Section: "readme"})

	opts, err := s.ListOptimizations()
	if err != nil {
		t.Fatalf("ListOptimizations: %v", err)
	}
	if len(opts) != 2 {
		t.Errorf("expected 2 optimizations, got %d", len(opts))
	}
}

func TestListOptimizations_Empty(t *testing.T) {
	s := newTestStore(t)
	opts, err := s.ListOptimizations()
	if err != nil {
		t.Fatalf("ListOptimizations: %v", err)
	}
	if opts != nil && len(opts) != 0 {
		t.Errorf("expected empty list, got %d", len(opts))
	}
}
