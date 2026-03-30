package store

import (
	"strings"
	"testing"
)

// --- Open error paths ---

func TestOpen_InvalidPath(t *testing.T) {
	_, err := Open("/nonexistent/deep/path/scout.db")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

// --- UpdateLead empty fields (no-op) ---

func TestUpdateLead_NoValidFields(t *testing.T) {
	s := newTestStore(t)
	id, _ := s.AddLead(Lead{JobTitle: "Test", Pipeline: "job"})

	// All fields are disallowed — should be a no-op (nil error).
	err := s.UpdateLead(id, map[string]interface{}{"bad_field": "x"})
	if err != nil {
		t.Errorf("expected nil for empty valid fields, got %v", err)
	}
}

// --- GetAgentRun not found ---

func TestGetAgentRun_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetAgentRun(99999)
	if err == nil {
		t.Error("expected error for nonexistent agent run")
	}
}

// --- AddLeadIfNotExists closed-store error path ---

func TestAddLeadIfNotExists_ClosedStore(t *testing.T) {
	s := newTestStore(t)
	s.Close()

	tsID := int64(42)
	_, _, err := s.AddLeadIfNotExists(Lead{TheirStackID: &tsID, JobTitle: "X"})
	if err == nil {
		t.Error("expected error from AddLeadIfNotExists on closed store")
	}
}

// --- Closed store operations ---

func TestClosedStore_Leads(t *testing.T) {
	s := newTestStore(t)
	s.Close()

	_, err := s.AddLead(Lead{JobTitle: "X", Pipeline: "job"})
	if err == nil {
		t.Error("expected error from AddLead on closed store")
	}
	_, err = s.GetLead(1)
	if err == nil {
		t.Error("expected error from GetLead on closed store")
	}
	_, err = s.ListLeads("", false)
	if err == nil {
		t.Error("expected error from ListLeads on closed store")
	}
	_, err = s.ScoredLeads(10)
	if err == nil {
		t.Error("expected error from ScoredLeads on closed store")
	}
	err = s.RecordStageHistory(1, "a", "b", "c")
	if err == nil {
		t.Error("expected error from RecordStageHistory on closed store")
	}
	_, err = s.GetStageHistory(1)
	if err == nil {
		t.Error("expected error from GetStageHistory on closed store")
	}
	err = s.UpdateLead(1, map[string]interface{}{"job_title": "x"})
	if err == nil {
		t.Error("expected error from UpdateLead on closed store")
	}
}

func TestClosedStore_Analytics(t *testing.T) {
	s := newTestStore(t)
	s.Close()

	_, err := s.GetAnalytics("")
	if err == nil {
		t.Error("expected error from GetAnalytics on closed store")
	}
	// Also test with pipeline filter — different code path.
	_, err = s.GetAnalytics("job")
	if err == nil {
		t.Error("expected error from GetAnalytics(job) on closed store")
	}
}

func TestClosedStore_AgentRuns(t *testing.T) {
	s := newTestStore(t)
	s.Close()

	_, err := s.AddAgentRun(AgentRun{Platform: "x"})
	if err == nil {
		t.Error("expected error from AddAgentRun on closed store")
	}
	_, err = s.GetAgentRun(1)
	if err == nil {
		t.Error("expected error from GetAgentRun on closed store")
	}
	_, err = s.ListAgentRuns("")
	if err == nil {
		t.Error("expected error from ListAgentRuns on closed store")
	}
	// LatestAgentRun returns nil on error — no error propagated.
	got, err := s.LatestAgentRun()
	if err != nil {
		t.Errorf("LatestAgentRun should not propagate error: %v", err)
	}
	if got != nil {
		t.Error("expected nil from LatestAgentRun on closed store")
	}
}

func TestClosedStore_Artifacts(t *testing.T) {
	s := newTestStore(t)
	s.Close()

	_, err := s.AddArtifact(1, "cover_letter", "x")
	if err == nil {
		t.Error("expected error from AddArtifact on closed store")
	}
	_, err = s.GetArtifacts(1)
	if err == nil {
		t.Error("expected error from GetArtifacts on closed store")
	}
	_, err = s.GetArtifactsByType(1, "cover_letter")
	if err == nil {
		t.Error("expected error from GetArtifactsByType on closed store")
	}
	_, err = s.GetLatestArtifact(1, "cover_letter")
	if err == nil {
		t.Error("expected error from GetLatestArtifact on closed store")
	}
}

func TestClosedStore_Interactions(t *testing.T) {
	s := newTestStore(t)
	s.Close()

	_, err := s.AddInteraction(1, "email", "gmail", "test")
	if err == nil {
		t.Error("expected error from AddInteraction on closed store")
	}
	_, err = s.GetInteractions(1)
	if err == nil {
		t.Error("expected error from GetInteractions on closed store")
	}
	_, err = s.GetInteractionCount(1)
	if err == nil {
		t.Error("expected error from GetInteractionCount on closed store")
	}
}

func TestClosedStore_Optimizations(t *testing.T) {
	s := newTestStore(t)
	s.Close()

	_, err := s.AddOptimization(Optimization{Platform: "x"})
	if err == nil {
		t.Error("expected error from AddOptimization on closed store")
	}
	_, err = s.ListOptimizations()
	if err == nil {
		t.Error("expected error from ListOptimizations on closed store")
	}
}

func TestClosedStore_Sync(t *testing.T) {
	s := newTestStore(t)
	s.Close()

	_, err := s.AddSyncResult(SyncResult{Platform: "x"})
	if err == nil {
		t.Error("expected error from AddSyncResult on closed store")
	}
	_, err = s.GetSyncMeta("key")
	if err == nil {
		t.Error("expected error from GetSyncMeta on closed store")
	}
	err = s.SetSyncMeta("key", "val")
	if err == nil {
		t.Error("expected error from SetSyncMeta on closed store")
	}
}

func TestClosedStore_Content(t *testing.T) {
	s := newTestStore(t)
	s.Close()

	_, err := s.AddContentPost(ContentPost{Platform: "linkedin"})
	if err == nil {
		t.Error("expected error from AddContentPost on closed store")
	}
	_, err = s.ListContentPosts("", "")
	if err == nil {
		t.Error("expected error from ListContentPosts on closed store")
	}
	err = s.UpdateContentPost(1, map[string]interface{}{"status": "x"})
	if err == nil {
		t.Error("expected error from UpdateContentPost on closed store")
	}
	_, err = s.AddBacklogItem(BacklogItem{Topic: "x"})
	if err == nil {
		t.Error("expected error from AddBacklogItem on closed store")
	}
	_, err = s.ListBacklog("")
	if err == nil {
		t.Error("expected error from ListBacklog on closed store")
	}
	err = s.UpdateBacklogItem(1, map[string]interface{}{"status": "x"})
	if err == nil {
		t.Error("expected error from UpdateBacklogItem on closed store")
	}
}

// --- Open error message ---

func TestOpen_ErrorMessage(t *testing.T) {
	_, err := Open("/nonexistent/deep/scout.db")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "scout") {
		t.Errorf("error should mention scout: %q", err)
	}
}
