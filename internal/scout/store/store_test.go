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

// TestMigration_NewSchema verifies the new leads schema has the theirstack_id column and index.
func TestMigration_NewSchema(t *testing.T) {
	s := newTestStore(t)

	// Verify theirstack_id column exists by querying PRAGMA table_info.
	rows, err := s.db.Query("PRAGMA table_info(leads)")
	if err != nil {
		t.Fatalf("PRAGMA table_info: %v", err)
	}
	defer rows.Close()

	var foundTherStackID bool
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var dfltValue interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dfltValue, &pk); err != nil {
			t.Fatalf("scan table_info: %v", err)
		}
		if name == "theirstack_id" {
			foundTherStackID = true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("table_info rows error: %v", err)
	}
	if !foundTherStackID {
		t.Error("leads table missing theirstack_id column")
	}

	// Verify idx_leads_theirstack_id index exists.
	indexRows, err := s.db.Query("PRAGMA index_list(leads)")
	if err != nil {
		t.Fatalf("PRAGMA index_list: %v", err)
	}
	defer indexRows.Close()

	var foundIndex bool
	for indexRows.Next() {
		var seq int
		var name string
		var unique int
		var origin string
		var partial int
		if err := indexRows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			t.Fatalf("scan index_list: %v", err)
		}
		if name == "idx_leads_theirstack_id" {
			foundIndex = true
		}
	}
	if err := indexRows.Err(); err != nil {
		t.Fatalf("index_list rows error: %v", err)
	}
	if !foundIndex {
		t.Error("missing index idx_leads_theirstack_id on leads table")
	}
}

func TestAddLead(t *testing.T) {
	s := newTestStore(t)
	id, err := s.AddLead(Lead{
		JobTitle: "Senior Go Developer",
		Company:  "Acme Corp",
		Pipeline: "job",
		Source:   "theirstack",
		Stage:    "discovered",
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
		JobTitle:   "Backend Engineer",
		Company:    "TechCo",
		Pipeline:   "job",
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
	if lead.JobTitle != "Backend Engineer" {
		t.Errorf("JobTitle = %q, want %q", lead.JobTitle, "Backend Engineer")
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
		JobTitle: "Original Title",
		Pipeline: "job",
		Stage:    "discovered",
	})

	err := s.UpdateLead(id, map[string]interface{}{
		"job_title":   "Updated Title",
		"stage":       "applied",
		"match_score": 0.92,
	})
	if err != nil {
		t.Fatalf("UpdateLead: %v", err)
	}

	lead, _ := s.GetLead(id)
	if lead.JobTitle != "Updated Title" {
		t.Errorf("JobTitle = %q, want %q", lead.JobTitle, "Updated Title")
	}
	if lead.Stage != "applied" {
		t.Errorf("Stage = %q, want %q", lead.Stage, "applied")
	}
	if lead.MatchScore != 0.92 {
		t.Errorf("MatchScore = %f, want %f", lead.MatchScore, 0.92)
	}

	// Not found.
	err = s.UpdateLead(999, map[string]interface{}{"job_title": "x"})
	if err == nil {
		t.Error("expected error for non-existent lead")
	}
}

func TestListLeads_Filter(t *testing.T) {
	s := newTestStore(t)
	s.AddLead(Lead{JobTitle: "Job 1", Pipeline: "job", Stage: "discovered"})
	s.AddLead(Lead{JobTitle: "Job 2", Pipeline: "job", Stage: "applied", ClosedAt: "2025-01-01T00:00:00Z"})
	s.AddLead(Lead{JobTitle: "Freelance 1", Pipeline: "freelance", Stage: "found"})

	// All leads.
	all, err := s.ListLeads("", false)
	if err != nil {
		t.Fatalf("ListLeads all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("len = %d, want 3", len(all))
	}

	// Filter by pipeline.
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

	// Pipeline + active.
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
	s.AddLead(Lead{JobTitle: "Low", Pipeline: "job", MatchScore: 0.3})
	s.AddLead(Lead{JobTitle: "High", Pipeline: "job", MatchScore: 0.95})
	s.AddLead(Lead{JobTitle: "Mid", Pipeline: "job", MatchScore: 0.6})

	leads, err := s.ScoredLeads(2)
	if err != nil {
		t.Fatalf("ScoredLeads: %v", err)
	}
	if len(leads) != 2 {
		t.Fatalf("len = %d, want 2", len(leads))
	}
	if leads[0].JobTitle != "High" {
		t.Errorf("first = %q, want %q", leads[0].JobTitle, "High")
	}
	if leads[1].JobTitle != "Mid" {
		t.Errorf("second = %q, want %q", leads[1].JobTitle, "Mid")
	}
}

func TestRecordStageHistory(t *testing.T) {
	s := newTestStore(t)
	id, _ := s.AddLead(Lead{JobTitle: "Test", Pipeline: "job", Stage: "discovered"})

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

	// Add leads with varying pipelines and statuses.
	staleTime := time.Now().UTC().Add(-10 * 24 * time.Hour).Format(time.RFC3339)
	freshTime := time.Now().UTC().Format(time.RFC3339)
	closedTime := time.Now().UTC().Add(-2 * 24 * time.Hour).Format(time.RFC3339)

	s.AddLead(Lead{JobTitle: "Job 1", Pipeline: "job", Source: "linkedin", Stage: "discovered", CreatedAt: staleTime, UpdatedAt: staleTime})
	s.AddLead(Lead{JobTitle: "Job 2", Pipeline: "job", Source: "indeed", Stage: "applied", CreatedAt: freshTime, UpdatedAt: freshTime, NextDate: "2020-01-01"})
	s.AddLead(Lead{JobTitle: "Job 3", Pipeline: "job", Source: "linkedin", Stage: "offer", CreatedAt: staleTime, UpdatedAt: closedTime, ClosedAt: closedTime})
	s.AddLead(Lead{JobTitle: "Freelance 1", Pipeline: "freelance", Source: "upwork", Stage: "found", CreatedAt: freshTime, UpdatedAt: freshTime})

	a, err := s.GetAnalytics("")
	if err != nil {
		t.Fatalf("GetAnalytics: %v", err)
	}

	// Stats: ByType is keyed by pipeline in the new schema.
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
