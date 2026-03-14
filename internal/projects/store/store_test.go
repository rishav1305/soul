package store

import (
	"path/filepath"
	"testing"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestProjectCRUD(t *testing.T) {
	s := openTestStore(t)

	// Create project.
	id, err := s.CreateProject("rag-pipeline", "Build a RAG pipeline", 1, 1, 22.5)
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero ID")
	}

	// Get by ID.
	p, err := s.GetProject(int(id))
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if p.Name != "rag-pipeline" || p.Phase != 1 || p.WeekPlanned != 1 || p.HoursEstimated != 22.5 {
		t.Fatalf("unexpected project fields: %+v", p)
	}
	if p.Status != "backlog" {
		t.Fatalf("expected backlog status, got %s", p.Status)
	}

	// Get by name.
	p2, err := s.GetProjectByName("rag-pipeline")
	if err != nil {
		t.Fatalf("GetProjectByName: %v", err)
	}
	if p2.ID != p.ID {
		t.Fatalf("expected ID %d, got %d", p.ID, p2.ID)
	}

	// Create second project.
	_, err = s.CreateProject("fine-tuning", "Fine-tune LLMs", 1, 2, 22.5)
	if err != nil {
		t.Fatalf("CreateProject 2: %v", err)
	}

	// List projects.
	list, err := s.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(list))
	}
	// Should be ordered by phase, week_planned.
	if list[0].Name != "rag-pipeline" || list[1].Name != "fine-tuning" {
		t.Fatalf("unexpected order: %s, %s", list[0].Name, list[1].Name)
	}

	// Update project.
	status := "active"
	hours := 5.0
	repo := "https://github.com/test/rag"
	err = s.UpdateProject(int(id), ProjectUpdate{
		Status:      &status,
		HoursActual: &hours,
		GithubRepo:  &repo,
	})
	if err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}

	updated, err := s.GetProject(int(id))
	if err != nil {
		t.Fatalf("GetProject after update: %v", err)
	}
	if updated.Status != "active" || updated.HoursActual != 5.0 || updated.GithubRepo != "https://github.com/test/rag" {
		t.Fatalf("unexpected updated fields: %+v", updated)
	}

	// ProjectCount.
	count, err := s.ProjectCount()
	if err != nil {
		t.Fatalf("ProjectCount: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2, got %d", count)
	}
}

func TestProjectUniqueName(t *testing.T) {
	s := openTestStore(t)

	_, err := s.CreateProject("rag-pipeline", "First", 1, 1, 22.5)
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	// Duplicate name should fail.
	_, err = s.CreateProject("rag-pipeline", "Second", 2, 2, 10)
	if err == nil {
		t.Fatal("expected error for duplicate name")
	}
}

func TestMilestoneCRUD(t *testing.T) {
	s := openTestStore(t)

	pid, _ := s.CreateProject("rag-pipeline", "RAG", 1, 1, 22.5)

	// Create milestones with sort_order.
	_, err := s.CreateMilestone(int(pid), "Ingestion", "Parse docs", "100+ docs", 2)
	if err != nil {
		t.Fatalf("CreateMilestone: %v", err)
	}
	mid2, err := s.CreateMilestone(int(pid), "Embedding", "Generate embeddings", "1000 chunks in <60s", 1)
	if err != nil {
		t.Fatalf("CreateMilestone 2: %v", err)
	}

	// List — should be ordered by sort_order.
	milestones, err := s.ListMilestones(int(pid))
	if err != nil {
		t.Fatalf("ListMilestones: %v", err)
	}
	if len(milestones) != 2 {
		t.Fatalf("expected 2 milestones, got %d", len(milestones))
	}
	if milestones[0].Name != "Embedding" || milestones[1].Name != "Ingestion" {
		t.Fatalf("unexpected order: %s, %s", milestones[0].Name, milestones[1].Name)
	}
	if milestones[0].Status != "pending" {
		t.Fatalf("expected pending, got %s", milestones[0].Status)
	}

	// Update to done — should set completed_at.
	err = s.UpdateMilestoneStatus(int(mid2), "done")
	if err != nil {
		t.Fatalf("UpdateMilestoneStatus done: %v", err)
	}
	milestones, _ = s.ListMilestones(int(pid))
	for _, m := range milestones {
		if m.ID == int(mid2) {
			if m.Status != "done" {
				t.Fatalf("expected done, got %s", m.Status)
			}
			if m.CompletedAt == "" {
				t.Fatal("expected completed_at to be set")
			}
		}
	}

	// Update to skipped — should clear completed_at.
	err = s.UpdateMilestoneStatus(int(mid2), "skipped")
	if err != nil {
		t.Fatalf("UpdateMilestoneStatus skipped: %v", err)
	}
	milestones, _ = s.ListMilestones(int(pid))
	for _, m := range milestones {
		if m.ID == int(mid2) {
			if m.Status != "skipped" {
				t.Fatalf("expected skipped, got %s", m.Status)
			}
			if m.CompletedAt != "" {
				t.Fatalf("expected empty completed_at, got %s", m.CompletedAt)
			}
		}
	}
}

func TestMetricCRUD(t *testing.T) {
	s := openTestStore(t)

	pid, _ := s.CreateProject("rag-pipeline", "RAG", 1, 1, 22.5)

	// Record metrics.
	_, err := s.RecordMetric(int(pid), "recall@10", "0.85", "ratio")
	if err != nil {
		t.Fatalf("RecordMetric: %v", err)
	}
	_, err = s.RecordMetric(int(pid), "latency_p95", "120", "ms")
	if err != nil {
		t.Fatalf("RecordMetric 2: %v", err)
	}

	// List — should be ordered by captured_at DESC (latest first).
	metrics, err := s.ListMetrics(int(pid))
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}
	if len(metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(metrics))
	}
	// Both inserted at same second, so order by ID DESC effectively.
	if metrics[0].Name != "latency_p95" {
		t.Fatalf("expected latest metric first, got %s", metrics[0].Name)
	}
}

func TestKeywordCRUD(t *testing.T) {
	s := openTestStore(t)

	pid, _ := s.CreateProject("rag-pipeline", "RAG", 1, 1, 22.5)
	pidInt := int(pid)

	// Create keyword with project.
	kwID, err := s.CreateKeyword(&pidInt, "RAG")
	if err != nil {
		t.Fatalf("CreateKeyword: %v", err)
	}
	if kwID == 0 {
		t.Fatal("expected non-zero keyword ID")
	}

	// Create standalone keyword (no project).
	_, err = s.CreateKeyword(nil, "Prompt Engineering")
	if err != nil {
		t.Fatalf("CreateKeyword standalone: %v", err)
	}

	// List all keywords.
	keywords, err := s.ListKeywords()
	if err != nil {
		t.Fatalf("ListKeywords: %v", err)
	}
	if len(keywords) != 2 {
		t.Fatalf("expected 2 keywords, got %d", len(keywords))
	}

	// List project keywords.
	projKW, err := s.ListProjectKeywords(pidInt)
	if err != nil {
		t.Fatalf("ListProjectKeywords: %v", err)
	}
	if len(projKW) != 1 {
		t.Fatalf("expected 1 project keyword, got %d", len(projKW))
	}
	if projKW[0].Keyword != "RAG" {
		t.Fatalf("expected RAG, got %s", projKW[0].Keyword)
	}

	// Update keyword status to shipped.
	err = s.UpdateKeywordStatus(int(kwID), "shipped")
	if err != nil {
		t.Fatalf("UpdateKeywordStatus: %v", err)
	}
	keywords, _ = s.ListKeywords()
	for _, k := range keywords {
		if k.ID == int(kwID) {
			if k.Status != "shipped" {
				t.Fatalf("expected shipped, got %s", k.Status)
			}
			if k.ShippedAt == "" {
				t.Fatal("expected shipped_at to be set")
			}
		}
	}
}

func TestKeywordUnique(t *testing.T) {
	s := openTestStore(t)

	pid, _ := s.CreateProject("rag-pipeline", "RAG", 1, 1, 22.5)
	pidInt := int(pid)

	// First insert.
	id1, err := s.CreateKeyword(&pidInt, "RAG")
	if err != nil {
		t.Fatalf("CreateKeyword: %v", err)
	}

	// Duplicate — INSERT OR IGNORE should not error, but returns 0 for ID.
	id2, err := s.CreateKeyword(&pidInt, "RAG")
	if err != nil {
		t.Fatalf("CreateKeyword duplicate: %v", err)
	}

	// Should not have created a new row.
	keywords, _ := s.ListKeywords()
	if len(keywords) != 1 {
		t.Fatalf("expected 1 keyword after duplicate, got %d", len(keywords))
	}
	// id2 should be 0 (no new row inserted) or same as id1.
	_ = id1
	_ = id2
}

func TestProfileSyncCRUD(t *testing.T) {
	s := openTestStore(t)

	pid, _ := s.CreateProject("rag-pipeline", "RAG", 1, 1, 22.5)

	// Create sync.
	_, err := s.CreateProfileSync(int(pid), "linkedin")
	if err != nil {
		t.Fatalf("CreateProfileSync: %v", err)
	}
	_, err = s.CreateProfileSync(int(pid), "github")
	if err != nil {
		t.Fatalf("CreateProfileSync 2: %v", err)
	}

	// List syncs.
	syncs, err := s.ListProfileSyncs(int(pid))
	if err != nil {
		t.Fatalf("ListProfileSyncs: %v", err)
	}
	if len(syncs) != 2 {
		t.Fatalf("expected 2 syncs, got %d", len(syncs))
	}
	// Should be ordered by platform.
	if syncs[0].Platform != "github" || syncs[1].Platform != "linkedin" {
		t.Fatalf("unexpected order: %s, %s", syncs[0].Platform, syncs[1].Platform)
	}
	if syncs[0].Synced {
		t.Fatal("expected synced=false initially")
	}

	// Update sync.
	err = s.UpdateProfileSync(int(pid), "linkedin", "Updated profile headline")
	if err != nil {
		t.Fatalf("UpdateProfileSync: %v", err)
	}
	syncs, _ = s.ListProfileSyncs(int(pid))
	for _, ps := range syncs {
		if ps.Platform == "linkedin" {
			if !ps.Synced {
				t.Fatal("expected synced=true after update")
			}
			if ps.SyncedAt == "" {
				t.Fatal("expected synced_at to be set")
			}
			if ps.Notes != "Updated profile headline" {
				t.Fatalf("expected notes to be set, got %s", ps.Notes)
			}
		}
	}
}

func TestProfileSyncUnique(t *testing.T) {
	s := openTestStore(t)

	pid, _ := s.CreateProject("rag-pipeline", "RAG", 1, 1, 22.5)

	_, err := s.CreateProfileSync(int(pid), "linkedin")
	if err != nil {
		t.Fatalf("CreateProfileSync: %v", err)
	}

	// Duplicate (project_id, platform) should fail.
	_, err = s.CreateProfileSync(int(pid), "linkedin")
	if err == nil {
		t.Fatal("expected error for duplicate (project_id, platform)")
	}
}

func TestReadinessCRUD(t *testing.T) {
	s := openTestStore(t)

	pid, _ := s.CreateProject("rag-pipeline", "RAG", 1, 1, 22.5)

	// No readiness yet.
	r, err := s.GetReadiness(int(pid))
	if err != nil {
		t.Fatalf("GetReadiness empty: %v", err)
	}
	if r != nil {
		t.Fatal("expected nil readiness for new project")
	}

	// Record readiness.
	_, err = s.RecordReadiness(int(pid), true, false, true, 3)
	if err != nil {
		t.Fatalf("RecordReadiness: %v", err)
	}

	r, err = s.GetReadiness(int(pid))
	if err != nil {
		t.Fatalf("GetReadiness: %v", err)
	}
	if r == nil {
		t.Fatal("expected non-nil readiness")
	}
	if !r.CanExplain || r.CanDemo || !r.CanTradeoffs || r.SelfScore != 3 {
		t.Fatalf("unexpected readiness: %+v", r)
	}

	// Record again — GetReadiness should return latest.
	_, err = s.RecordReadiness(int(pid), true, true, true, 5)
	if err != nil {
		t.Fatalf("RecordReadiness 2: %v", err)
	}

	r2, err := s.GetReadiness(int(pid))
	if err != nil {
		t.Fatalf("GetReadiness latest: %v", err)
	}
	if r2.SelfScore != 5 || !r2.CanDemo {
		t.Fatalf("expected latest readiness (score=5, canDemo=true), got %+v", r2)
	}
}

func TestDashboard(t *testing.T) {
	s := openTestStore(t)

	// Seed.
	if err := s.Seed(); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	// Record readiness for first 2 projects.
	s.RecordReadiness(1, true, true, true, 4)
	s.RecordReadiness(2, true, false, false, 2)

	d, err := s.GetDashboard()
	if err != nil {
		t.Fatalf("GetDashboard: %v", err)
	}

	// All 11 projects should be backlog.
	if d.TotalProjects != 11 {
		t.Fatalf("expected 11 total, got %d", d.TotalProjects)
	}
	if d.Backlog != 11 {
		t.Fatalf("expected 11 backlog, got %d", d.Backlog)
	}
	if d.Shipped != 0 || d.Active != 0 {
		t.Fatalf("expected 0 shipped/active, got %d/%d", d.Shipped, d.Active)
	}

	// Keywords: 69 project keywords - 1 duplicate (Guardrails in agent-framework + ai-safety) = 68
	// + 5 pre-shipped - 1 duplicate (Multi-Agent Systems in agent-framework) = 4 new
	// Total: 68 + 4 = 72
	if d.KeywordsTotal != 72 {
		t.Fatalf("expected 72 keywords, got %d", d.KeywordsTotal)
	}
	// 4 pre-shipped shipped (Multi-Agent Systems was already claimed from agent-framework).
	if d.KeywordsShipped != 4 {
		t.Fatalf("expected 4 shipped keywords, got %d", d.KeywordsShipped)
	}

	// Hours.
	expectedHours := 22.5 + 22.5 + 17.5 + 17.5 + 17.5 + 22.5 + 22.5 + 22.5 + 17.5 + 22.5 + 17.5
	if d.HoursEstimated != expectedHours {
		t.Fatalf("expected %.1f estimated hours, got %.1f", expectedHours, d.HoursEstimated)
	}

	// Readiness avg.
	if d.AvgReadiness != 3.0 {
		t.Fatalf("expected avg readiness 3.0, got %.1f", d.AvgReadiness)
	}

	// Project list should have 11 entries.
	if len(d.Projects) != 11 {
		t.Fatalf("expected 11 projects in list, got %d", len(d.Projects))
	}

	// First project should be rag-pipeline with 6 milestones.
	if d.Projects[0].Name != "rag-pipeline" {
		t.Fatalf("expected first project rag-pipeline, got %s", d.Projects[0].Name)
	}
	if d.Projects[0].MilestonesTotal != 6 {
		t.Fatalf("expected 6 milestones for rag-pipeline, got %d", d.Projects[0].MilestonesTotal)
	}
}

func TestSeed(t *testing.T) {
	s := openTestStore(t)

	// Seed on empty DB.
	if err := s.Seed(); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	// 11 projects.
	count, err := s.ProjectCount()
	if err != nil {
		t.Fatalf("ProjectCount: %v", err)
	}
	if count != 11 {
		t.Fatalf("expected 11 projects, got %d", count)
	}

	// Verify project names and ordering.
	projects, err := s.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	expectedNames := []string{
		"rag-pipeline", "fine-tuning",
		"llm-evaluation", "mlops-pipeline", "model-serving",
		"data-quality", "agent-framework",
		"knowledge-graph", "multimodal-ai", "streaming-ai", "ai-safety",
	}
	for i, name := range expectedNames {
		if projects[i].Name != name {
			t.Fatalf("expected project %d to be %s, got %s", i, name, projects[i].Name)
		}
	}

	// Milestone counts per project: 6,6,5,6,6,6,7,6,5,6,5 = 64 total.
	totalMilestones := 0
	expectedMilestones := []int{6, 6, 5, 6, 6, 6, 7, 6, 5, 6, 5}
	for i, p := range projects {
		milestones, err := s.ListMilestones(p.ID)
		if err != nil {
			t.Fatalf("ListMilestones(%d): %v", p.ID, err)
		}
		if len(milestones) != expectedMilestones[i] {
			t.Fatalf("expected %d milestones for %s, got %d", expectedMilestones[i], p.Name, len(milestones))
		}
		totalMilestones += len(milestones)
	}
	if totalMilestones != 64 {
		t.Fatalf("expected 64 total milestones, got %d", totalMilestones)
	}

	// Keywords: 72 total (68 unique project + 4 unique pre-shipped).
	keywords, err := s.ListKeywords()
	if err != nil {
		t.Fatalf("ListKeywords: %v", err)
	}
	if len(keywords) != 72 {
		t.Fatalf("expected 72 keywords, got %d", len(keywords))
	}

	// Profile syncs: 11 projects * 7 platforms = 77.
	totalSyncs := 0
	for _, p := range projects {
		syncs, err := s.ListProfileSyncs(p.ID)
		if err != nil {
			t.Fatalf("ListProfileSyncs(%d): %v", p.ID, err)
		}
		if len(syncs) != 7 {
			t.Fatalf("expected 7 syncs for %s, got %d", p.Name, len(syncs))
		}
		totalSyncs += len(syncs)
	}
	if totalSyncs != 77 {
		t.Fatalf("expected 77 total syncs, got %d", totalSyncs)
	}

	// Idempotent — seed again should be a no-op.
	if err := s.Seed(); err != nil {
		t.Fatalf("Seed again: %v", err)
	}
	count2, _ := s.ProjectCount()
	if count2 != 11 {
		t.Fatalf("expected 11 after re-seed, got %d", count2)
	}
}

func TestCascadeDelete(t *testing.T) {
	s := openTestStore(t)

	pid, _ := s.CreateProject("rag-pipeline", "RAG", 1, 1, 22.5)
	pidInt := int(pid)

	// Add related data.
	s.CreateMilestone(pidInt, "Milestone 1", "desc", "ac", 1)
	s.RecordMetric(pidInt, "recall", "0.85", "ratio")
	s.CreateProfileSync(pidInt, "linkedin")
	s.RecordReadiness(pidInt, true, true, true, 4)
	s.CreateKeyword(&pidInt, "RAG")
	s.CreateKeyword(&pidInt, "Embeddings")

	// Verify data exists.
	milestones, _ := s.ListMilestones(pidInt)
	if len(milestones) != 1 {
		t.Fatalf("expected 1 milestone, got %d", len(milestones))
	}
	metrics, _ := s.ListMetrics(pidInt)
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}
	syncs, _ := s.ListProfileSyncs(pidInt)
	if len(syncs) != 1 {
		t.Fatalf("expected 1 sync, got %d", len(syncs))
	}
	readiness, _ := s.GetReadiness(pidInt)
	if readiness == nil {
		t.Fatal("expected readiness to exist")
	}
	projKW, _ := s.ListProjectKeywords(pidInt)
	if len(projKW) != 2 {
		t.Fatalf("expected 2 project keywords, got %d", len(projKW))
	}

	// Delete project.
	_, err := s.db.Exec("DELETE FROM projects WHERE id = ?", pid)
	if err != nil {
		t.Fatalf("DELETE project: %v", err)
	}

	// Cascaded deletes: milestones, metrics, syncs, readiness.
	milestones, _ = s.ListMilestones(pidInt)
	if len(milestones) != 0 {
		t.Fatalf("expected 0 milestones after cascade, got %d", len(milestones))
	}
	metrics, _ = s.ListMetrics(pidInt)
	if len(metrics) != 0 {
		t.Fatalf("expected 0 metrics after cascade, got %d", len(metrics))
	}
	syncs, _ = s.ListProfileSyncs(pidInt)
	if len(syncs) != 0 {
		t.Fatalf("expected 0 syncs after cascade, got %d", len(syncs))
	}
	readiness, _ = s.GetReadiness(pidInt)
	if readiness != nil {
		t.Fatal("expected nil readiness after cascade")
	}

	// Keywords: ON DELETE SET NULL — keywords should still exist but with project_id = NULL.
	allKW, _ := s.ListKeywords()
	if len(allKW) != 2 {
		t.Fatalf("expected 2 keywords to survive cascade, got %d", len(allKW))
	}
	for _, k := range allKW {
		if k.ProjectID != nil {
			t.Fatalf("expected NULL project_id after cascade, got %v", k.ProjectID)
		}
	}

	// Project keywords for deleted project should be empty.
	projKW, _ = s.ListProjectKeywords(pidInt)
	if len(projKW) != 0 {
		t.Fatalf("expected 0 project keywords after cascade, got %d", len(projKW))
	}
}
