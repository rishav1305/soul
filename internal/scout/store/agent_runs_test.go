package store

import "testing"

func TestAddAgentRun(t *testing.T) {
	s := newTestStore(t)
	run := AgentRun{
		Platform: "linkedin",
		Mode:     "resume",
		LeadID:   1,
	}
	id, err := s.AddAgentRun(run)
	if err != nil {
		t.Fatalf("AddAgentRun: %v", err)
	}
	if id <= 0 {
		t.Errorf("expected positive id, got %d", id)
	}
}

func TestGetAgentRun(t *testing.T) {
	s := newTestStore(t)
	run := AgentRun{
		Platform:        "linkedin",
		Mode:            "resume",
		LeadID:          42,
		Recommendations: "update headline",
	}
	id, _ := s.AddAgentRun(run)

	got, err := s.GetAgentRun(id)
	if err != nil {
		t.Fatalf("GetAgentRun: %v", err)
	}
	if got.Platform != "linkedin" {
		t.Errorf("Platform = %q, want linkedin", got.Platform)
	}
	if got.LeadID != 42 {
		t.Errorf("LeadID = %d, want 42", got.LeadID)
	}
	if got.Status != "pending" {
		t.Errorf("Status = %q, want pending (default)", got.Status)
	}
}

func TestUpdateAgentRun(t *testing.T) {
	s := newTestStore(t)
	id, _ := s.AddAgentRun(AgentRun{Platform: "github", Mode: "readme"})

	err := s.UpdateAgentRun(id, "completed", `{"score":95}`)
	if err != nil {
		t.Fatalf("UpdateAgentRun: %v", err)
	}

	got, _ := s.GetAgentRun(id)
	if got.Status != "completed" {
		t.Errorf("Status = %q, want completed", got.Status)
	}
	if got.Result != `{"score":95}` {
		t.Errorf("Result = %q, want json", got.Result)
	}
	if got.CompletedAt == "" {
		t.Error("expected CompletedAt to be set for completed status")
	}
}

func TestLatestAgentRun(t *testing.T) {
	s := newTestStore(t)

	// No runs yet.
	got, err := s.LatestAgentRun()
	if err != nil {
		t.Fatalf("LatestAgentRun (empty): %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for empty table, got %+v", got)
	}

	// Add two runs.
	s.AddAgentRun(AgentRun{Platform: "linkedin", Mode: "first"})
	s.AddAgentRun(AgentRun{Platform: "github", Mode: "second"})

	got, err = s.LatestAgentRun()
	if err != nil {
		t.Fatalf("LatestAgentRun: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil result")
	}
	if got.Mode != "second" {
		t.Errorf("Mode = %q, want second (most recent)", got.Mode)
	}
}

func TestListAgentRuns(t *testing.T) {
	s := newTestStore(t)
	s.AddAgentRun(AgentRun{Platform: "linkedin", Mode: "resume"})
	s.AddAgentRun(AgentRun{Platform: "github", Mode: "readme"})
	s.AddAgentRun(AgentRun{Platform: "linkedin", Mode: "outreach"})

	// All runs.
	all, err := s.ListAgentRuns("")
	if err != nil {
		t.Fatalf("ListAgentRuns (all): %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 runs, got %d", len(all))
	}

	// Filtered by platform.
	linkedin, err := s.ListAgentRuns("linkedin")
	if err != nil {
		t.Fatalf("ListAgentRuns (linkedin): %v", err)
	}
	if len(linkedin) != 2 {
		t.Errorf("expected 2 linkedin runs, got %d", len(linkedin))
	}
}
