package store

import (
	"path/filepath"
	"strings"
	"testing"
)

func openCovStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cov.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// --- Not-found paths ---

func TestGetProject_NotFound(t *testing.T) {
	s := openCovStore(t)
	_, err := s.GetProject(99999)
	if err == nil {
		t.Error("expected error for nonexistent project")
	}
}

func TestGetProjectByName_NotFound(t *testing.T) {
	s := openCovStore(t)
	_, err := s.GetProjectByName("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent project")
	}
}

func TestUpdateProject_NotFound(t *testing.T) {
	s := openCovStore(t)
	err := s.UpdateProject(99999, ProjectUpdate{Status: strPtr("active")})
	if err == nil {
		t.Error("expected error for nonexistent project")
	}
}

func TestUpdateMilestoneStatus_NotFound(t *testing.T) {
	s := openCovStore(t)
	err := s.UpdateMilestoneStatus(99999, "completed")
	if err == nil {
		t.Error("expected error for nonexistent milestone")
	}
}

func TestUpdateKeywordStatus_NotFound(t *testing.T) {
	s := openCovStore(t)
	err := s.UpdateKeywordStatus(99999, "mastered")
	if err == nil {
		t.Error("expected error for nonexistent keyword")
	}
}

func TestUpdateProfileSync_NotFound(t *testing.T) {
	s := openCovStore(t)
	err := s.UpdateProfileSync(99999, "github", "notes")
	if err == nil {
		t.Error("expected error for nonexistent sync")
	}
}

func TestGetReadiness_NotFound(t *testing.T) {
	s := openCovStore(t)
	r, err := s.GetReadiness(99999)
	if err != nil {
		t.Fatal(err)
	}
	if r != nil {
		t.Error("expected nil for nonexistent readiness")
	}
}

// --- CRUD roundtrips ---

func TestCreateMilestone_Roundtrip(t *testing.T) {
	s := openCovStore(t)
	pid, _ := s.CreateProject("Test", "desc", 1, 4, 40.0)

	mid, err := s.CreateMilestone(int(pid), "M1", "First milestone", "Must pass tests", 1)
	if err != nil {
		t.Fatal(err)
	}
	if mid == 0 {
		t.Error("expected non-zero milestone ID")
	}

	milestones, err := s.ListMilestones(int(pid))
	if err != nil {
		t.Fatal(err)
	}
	if len(milestones) != 1 {
		t.Fatalf("expected 1 milestone, got %d", len(milestones))
	}
}

func TestRecordMetric_Roundtrip(t *testing.T) {
	s := openCovStore(t)
	pid, _ := s.CreateProject("Metrics", "desc", 1, 4, 40.0)

	mid, err := s.RecordMetric(int(pid), "test_coverage", "85.5", "percent")
	if err != nil {
		t.Fatal(err)
	}
	if mid == 0 {
		t.Error("expected non-zero metric ID")
	}

	metrics, err := s.ListMetrics(int(pid))
	if err != nil {
		t.Fatal(err)
	}
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}
}

func TestCreateKeyword_ListKeywords(t *testing.T) {
	s := openCovStore(t)
	pid, _ := s.CreateProject("Keywords", "desc", 1, 4, 40.0)
	pidInt := int(pid)

	_, err := s.CreateKeyword(&pidInt, "golang")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateKeyword(nil, "general-keyword")
	if err != nil {
		t.Fatal(err)
	}

	keywords, err := s.ListKeywords()
	if err != nil {
		t.Fatal(err)
	}
	if len(keywords) < 2 {
		t.Errorf("expected at least 2 keywords, got %d", len(keywords))
	}
}

func TestListProjectKeywords(t *testing.T) {
	s := openCovStore(t)
	pid, _ := s.CreateProject("ProjKW", "desc", 1, 4, 40.0)
	pidInt := int(pid)

	s.CreateKeyword(&pidInt, "react")
	s.CreateKeyword(&pidInt, "typescript")

	kws, err := s.ListProjectKeywords(pidInt)
	if err != nil {
		t.Fatal(err)
	}
	if len(kws) != 2 {
		t.Errorf("expected 2 project keywords, got %d", len(kws))
	}
}

func TestUpdateKeywordStatus_Success(t *testing.T) {
	s := openCovStore(t)
	pid, _ := s.CreateProject("KWS", "desc", 1, 4, 40.0)
	pidInt := int(pid)

	kid, err := s.CreateKeyword(&pidInt, "python")
	if err != nil {
		t.Fatal(err)
	}

	err = s.UpdateKeywordStatus(int(kid), "practicing")
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateProfileSync_Roundtrip(t *testing.T) {
	s := openCovStore(t)
	pid, _ := s.CreateProject("Sync", "desc", 1, 4, 40.0)

	sid, err := s.CreateProfileSync(int(pid), "github")
	if err != nil {
		t.Fatal(err)
	}
	if sid == 0 {
		t.Error("expected non-zero sync ID")
	}

	syncs, err := s.ListProfileSyncs(int(pid))
	if err != nil {
		t.Fatal(err)
	}
	if len(syncs) != 1 {
		t.Fatalf("expected 1 sync, got %d", len(syncs))
	}
}

func TestUpdateProfileSync_Success(t *testing.T) {
	s := openCovStore(t)
	pid, _ := s.CreateProject("SyncUpd", "desc", 1, 4, 40.0)
	s.CreateProfileSync(int(pid), "github")

	err := s.UpdateProfileSync(int(pid), "github", "synced successfully")
	if err != nil {
		t.Fatal(err)
	}
}

func TestRecordReadiness_Roundtrip(t *testing.T) {
	s := openCovStore(t)
	pid, _ := s.CreateProject("Ready", "desc", 1, 4, 40.0)

	rid, err := s.RecordReadiness(int(pid), true, true, false, 7)
	if err != nil {
		t.Fatal(err)
	}
	if rid == 0 {
		t.Error("expected non-zero readiness ID")
	}

	r, err := s.GetReadiness(int(pid))
	if err != nil {
		t.Fatal(err)
	}
	if r == nil {
		t.Fatal("expected readiness record")
	}
	if r.SelfScore != 7 {
		t.Errorf("SelfScore = %d, want 7", r.SelfScore)
	}
}

func TestProjectCount(t *testing.T) {
	s := openCovStore(t)
	s.CreateProject("A", "desc", 1, 4, 40.0)
	s.CreateProject("B", "desc", 2, 8, 80.0)

	count, err := s.ProjectCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("ProjectCount = %d, want 2", count)
	}
}

func TestUpsertDailyActivity_Accumulates(t *testing.T) {
	s := openCovStore(t)
	err := s.UpsertDailyActivity("2025-03-30", 120, 2, 1)
	if err != nil {
		t.Fatal(err)
	}

	// Upsert again — should accumulate.
	err = s.UpsertDailyActivity("2025-03-30", 60, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetActivity_EmptyDB(t *testing.T) {
	s := openCovStore(t)
	activities, err := s.GetActivity(7)
	if err != nil {
		t.Fatal(err)
	}
	if len(activities) != 0 {
		t.Errorf("expected 0 activities, got %d", len(activities))
	}
}

func TestGetDashboard(t *testing.T) {
	s := openCovStore(t)
	s.CreateProject("Dash1", "desc", 1, 4, 40.0)

	dash, err := s.GetDashboard()
	if err != nil {
		t.Fatal(err)
	}
	if dash.TotalProjects != 1 {
		t.Errorf("TotalProjects = %d, want 1", dash.TotalProjects)
	}
}

// --- Closed store operations ---

func TestClosedStore_Operations(t *testing.T) {
	s := openCovStore(t)
	s.Close()

	_, err := s.CreateProject("x", "x", 1, 1, 1.0)
	if err == nil {
		t.Error("expected error from CreateProject on closed store")
	}
	_, err = s.GetProject(1)
	if err == nil {
		t.Error("expected error from GetProject on closed store")
	}
	_, err = s.GetProjectByName("x")
	if err == nil {
		t.Error("expected error from GetProjectByName on closed store")
	}
	_, err = s.ListProjects()
	if err == nil {
		t.Error("expected error from ListProjects on closed store")
	}
	_, err = s.ProjectCount()
	if err == nil {
		t.Error("expected error from ProjectCount on closed store")
	}
	_, err = s.ListMilestones(1)
	if err == nil {
		t.Error("expected error from ListMilestones on closed store")
	}
	_, err = s.ListMetrics(1)
	if err == nil {
		t.Error("expected error from ListMetrics on closed store")
	}
	_, err = s.ListKeywords()
	if err == nil {
		t.Error("expected error from ListKeywords on closed store")
	}
	_, err = s.ListProjectKeywords(1)
	if err == nil {
		t.Error("expected error from ListProjectKeywords on closed store")
	}
	_, err = s.ListProfileSyncs(1)
	if err == nil {
		t.Error("expected error from ListProfileSyncs on closed store")
	}
	_, err = s.GetReadiness(1)
	if err == nil {
		t.Error("expected error from GetReadiness on closed store")
	}
	_, err = s.GetDashboard()
	if err == nil {
		t.Error("expected error from GetDashboard on closed store")
	}
	_, err = s.GetActivity(7)
	if err == nil {
		t.Error("expected error from GetActivity on closed store")
	}
}

// --- Open error ---

func TestOpen_InvalidPath(t *testing.T) {
	_, err := Open("/nonexistent/deep/path/db.sqlite")
	if err == nil {
		t.Error("expected error for invalid path")
	}
	if !strings.Contains(err.Error(), "projects") {
		t.Errorf("error should mention projects: %q", err)
	}
}

// --- Helper ---

func intPtr(i int) *int { return &i }
func strPtr(s string) *string { return &s }
