package store

import (
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "scout_test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestAddLead(t *testing.T) {
	s := newTestStore(t)
	id, err := s.AddLead(Lead{
		Title:   "Senior Go Developer",
		Company: "Acme Corp",
		Type:    "job",
		Source:  "linkedin",
		Stage:   "discovered",
	})
	if err != nil {
		t.Fatalf("AddLead: %v", err)
	}
	if id == 0 {
		t.Error("expected non-zero ID")
	}
}

func TestGetLead(t *testing.T) {
	s := newTestStore(t)
	id, err := s.AddLead(Lead{
		Title:      "Backend Engineer",
		Company:    "TechCo",
		Type:       "job",
		Source:     "indeed",
		Stage:      "applied",
		MatchScore: 0.85,
	})
	if err != nil {
		t.Fatalf("AddLead: %v", err)
	}

	lead, err := s.GetLead(id)
	if err != nil {
		t.Fatalf("GetLead: %v", err)
	}
	if lead.Title != "Backend Engineer" {
		t.Errorf("Title = %q, want %q", lead.Title, "Backend Engineer")
	}
	if lead.Company != "TechCo" {
		t.Errorf("Company = %q, want %q", lead.Company, "TechCo")
	}
	if lead.MatchScore != 0.85 {
		t.Errorf("MatchScore = %f, want %f", lead.MatchScore, 0.85)
	}

	// Not found.
	_, err = s.GetLead(999)
	if err == nil {
		t.Error("expected error for non-existent lead")
	}
}

func TestUpdateLead(t *testing.T) {
	s := newTestStore(t)
	id, _ := s.AddLead(Lead{
		Title: "Original Title",
		Type:  "job",
		Stage: "discovered",
	})

	err := s.UpdateLead(id, map[string]interface{}{
		"title":       "Updated Title",
		"stage":       "applied",
		"match_score": 0.92,
	})
	if err != nil {
		t.Fatalf("UpdateLead: %v", err)
	}

	lead, _ := s.GetLead(id)
	if lead.Title != "Updated Title" {
		t.Errorf("Title = %q, want %q", lead.Title, "Updated Title")
	}
	if lead.Stage != "applied" {
		t.Errorf("Stage = %q, want %q", lead.Stage, "applied")
	}
	if lead.MatchScore != 0.92 {
		t.Errorf("MatchScore = %f, want %f", lead.MatchScore, 0.92)
	}

	// Not found.
	err = s.UpdateLead(999, map[string]interface{}{"title": "x"})
	if err == nil {
		t.Error("expected error for non-existent lead")
	}
}

func TestListLeads_Filter(t *testing.T) {
	s := newTestStore(t)
	s.AddLead(Lead{Title: "Job 1", Type: "job", Stage: "discovered"})
	s.AddLead(Lead{Title: "Job 2", Type: "job", Stage: "applied", ClosedAt: "2025-01-01T00:00:00Z"})
	s.AddLead(Lead{Title: "Freelance 1", Type: "freelance", Stage: "found"})

	// All leads.
	all, err := s.ListLeads("", false)
	if err != nil {
		t.Fatalf("ListLeads all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("len = %d, want 3", len(all))
	}

	// Filter by type.
	jobs, err := s.ListLeads("job", false)
	if err != nil {
		t.Fatalf("ListLeads job: %v", err)
	}
	if len(jobs) != 2 {
		t.Errorf("len = %d, want 2", len(jobs))
	}

	// Active only.
	active, err := s.ListLeads("", true)
	if err != nil {
		t.Fatalf("ListLeads active: %v", err)
	}
	if len(active) != 2 {
		t.Errorf("len = %d, want 2", len(active))
	}

	// Type + active.
	jobActive, err := s.ListLeads("job", true)
	if err != nil {
		t.Fatalf("ListLeads job active: %v", err)
	}
	if len(jobActive) != 1 {
		t.Errorf("len = %d, want 1", len(jobActive))
	}
}

func TestScoredLeads(t *testing.T) {
	s := newTestStore(t)
	s.AddLead(Lead{Title: "Low", Type: "job", MatchScore: 0.3})
	s.AddLead(Lead{Title: "High", Type: "job", MatchScore: 0.95})
	s.AddLead(Lead{Title: "Mid", Type: "job", MatchScore: 0.6})

	leads, err := s.ScoredLeads(2)
	if err != nil {
		t.Fatalf("ScoredLeads: %v", err)
	}
	if len(leads) != 2 {
		t.Fatalf("len = %d, want 2", len(leads))
	}
	if leads[0].Title != "High" {
		t.Errorf("first = %q, want %q", leads[0].Title, "High")
	}
	if leads[1].Title != "Mid" {
		t.Errorf("second = %q, want %q", leads[1].Title, "Mid")
	}
}

func TestRecordStageHistory(t *testing.T) {
	s := newTestStore(t)
	id, _ := s.AddLead(Lead{Title: "Test", Type: "job", Stage: "discovered"})

	err := s.RecordStageHistory(id, "discovered", "applied", "submitted application")
	if err != nil {
		t.Fatalf("RecordStageHistory: %v", err)
	}

	history, err := s.GetStageHistory(id)
	if err != nil {
		t.Fatalf("GetStageHistory: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("len = %d, want 1", len(history))
	}
	if history[0].FromStage != "discovered" {
		t.Errorf("FromStage = %q, want %q", history[0].FromStage, "discovered")
	}
	if history[0].ToStage != "applied" {
		t.Errorf("ToStage = %q, want %q", history[0].ToStage, "applied")
	}
	if history[0].Notes != "submitted application" {
		t.Errorf("Notes = %q, want %q", history[0].Notes, "submitted application")
	}
}

func TestGetAnalytics(t *testing.T) {
	s := newTestStore(t)

	// Add leads with varying types and statuses.
	staleTime := time.Now().UTC().Add(-10 * 24 * time.Hour).Format(time.RFC3339)
	freshTime := time.Now().UTC().Format(time.RFC3339)
	closedTime := time.Now().UTC().Add(-2 * 24 * time.Hour).Format(time.RFC3339)

	s.AddLead(Lead{Title: "Job 1", Type: "job", Source: "linkedin", Stage: "discovered", CreatedAt: staleTime, UpdatedAt: staleTime})
	s.AddLead(Lead{Title: "Job 2", Type: "job", Source: "indeed", Stage: "applied", CreatedAt: freshTime, UpdatedAt: freshTime, NextDate: "2020-01-01"})
	s.AddLead(Lead{Title: "Job 3", Type: "job", Source: "linkedin", Stage: "offer", CreatedAt: staleTime, UpdatedAt: closedTime, ClosedAt: closedTime})
	s.AddLead(Lead{Title: "Freelance 1", Type: "freelance", Source: "upwork", Stage: "found", CreatedAt: freshTime, UpdatedAt: freshTime})

	a, err := s.GetAnalytics("")
	if err != nil {
		t.Fatalf("GetAnalytics: %v", err)
	}

	// Stats.
	if a.Stats.ByType["job"] != 3 {
		t.Errorf("ByType[job] = %d, want 3", a.Stats.ByType["job"])
	}
	if a.Stats.ByType["freelance"] != 1 {
		t.Errorf("ByType[freelance] = %d, want 1", a.Stats.ByType["freelance"])
	}
	if a.Stats.Active != 3 {
		t.Errorf("Active = %d, want 3", a.Stats.Active)
	}
	if a.Stats.Closed != 1 {
		t.Errorf("Closed = %d, want 1", a.Stats.Closed)
	}
	if a.Stats.Stale != 1 {
		t.Errorf("Stale = %d, want 1", a.Stats.Stale)
	}

	// Conversion.
	if len(a.Conversion.Funnels) < 2 {
		t.Fatalf("Funnels len = %d, want >= 2", len(a.Conversion.Funnels))
	}

	// Insights.
	if len(a.Insights.StaleLeads) != 1 {
		t.Errorf("StaleLeads len = %d, want 1", len(a.Insights.StaleLeads))
	}
	if len(a.Insights.FollowUpsDue) != 1 {
		t.Errorf("FollowUpsDue len = %d, want 1", len(a.Insights.FollowUpsDue))
	}
	// Pipeline gaps: contract, consulting, product-dev should have no active leads.
	if len(a.Insights.PipelineGaps) < 3 {
		t.Errorf("PipelineGaps len = %d, want >= 3", len(a.Insights.PipelineGaps))
	}

	// Filtered analytics.
	aJob, err := s.GetAnalytics("job")
	if err != nil {
		t.Fatalf("GetAnalytics(job): %v", err)
	}
	if aJob.Stats.ByType["job"] != 3 {
		t.Errorf("filtered ByType[job] = %d, want 3", aJob.Stats.ByType["job"])
	}
}
