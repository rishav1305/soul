package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/rishav1305/soul/internal/projects/store"
)

// testServer returns a Server backed by a temp SQLite store and an httptest.Server
// for routing that requires PathValue (e.g. /api/projects/{id}).
func testServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "projects.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	s := New(WithStore(st), WithContentDir(t.TempDir()))
	ts := httptest.NewServer(s.mux)
	t.Cleanup(ts.Close)
	return s, ts
}

// seedProject creates a project via the store and returns its ID.
func seedProject(t *testing.T, st *store.Store, name string) int {
	t.Helper()
	id, err := st.CreateProject(name, "desc", 1, 1, 10.0)
	if err != nil {
		t.Fatalf("seed project: %v", err)
	}
	return int(id)
}

// --- Health ---

func TestHandleHealth(t *testing.T) {
	s, _ := testServer(t)
	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != "ok" {
		t.Errorf("status = %v, want ok", body["status"])
	}
	if _, ok := body["project_count"]; !ok {
		t.Error("expected project_count in response")
	}
	if _, ok := body["uptime"]; !ok {
		t.Error("expected uptime in response")
	}
}

// --- Dashboard ---

func TestHandleDashboard_Empty(t *testing.T) {
	_, ts := testServer(t)
	resp, err := http.Get(ts.URL + "/api/projects/dashboard")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var dash store.Dashboard
	json.NewDecoder(resp.Body).Decode(&dash)
	if dash.Projects == nil {
		t.Error("expected non-nil projects slice")
	}
	if len(dash.Projects) != 0 {
		t.Errorf("expected 0 projects, got %d", len(dash.Projects))
	}
}

func TestHandleDashboard_WithProjects(t *testing.T) {
	s, ts := testServer(t)
	seedProject(t, s.store, "proj-alpha")
	seedProject(t, s.store, "proj-beta")

	resp, err := http.Get(ts.URL + "/api/projects/dashboard")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var dash store.Dashboard
	json.NewDecoder(resp.Body).Decode(&dash)
	if dash.TotalProjects != 2 {
		t.Errorf("total = %d, want 2", dash.TotalProjects)
	}
	if len(dash.Projects) != 2 {
		t.Errorf("projects = %d, want 2", len(dash.Projects))
	}
}

// --- Keywords ---

func TestHandleKeywords_Empty(t *testing.T) {
	_, ts := testServer(t)
	resp, err := http.Get(ts.URL + "/api/projects/keywords")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var keywords []store.Keyword
	json.NewDecoder(resp.Body).Decode(&keywords)
	if keywords == nil {
		t.Error("expected non-nil keywords slice")
	}
}

func TestHandleKeywords_WithData(t *testing.T) {
	s, ts := testServer(t)
	pid := seedProject(t, s.store, "kw-proj")
	s.store.CreateKeyword(&pid, "golang")
	s.store.CreateKeyword(&pid, "react")

	resp, err := http.Get(ts.URL + "/api/projects/keywords")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var keywords []store.Keyword
	json.NewDecoder(resp.Body).Decode(&keywords)
	if len(keywords) != 2 {
		t.Errorf("keywords = %d, want 2", len(keywords))
	}
}

// --- Get Project ---

func TestHandleGetProject_Valid(t *testing.T) {
	s, ts := testServer(t)
	pid := seedProject(t, s.store, "detail-proj")
	// Add related data.
	s.store.CreateMilestone(pid, "M1", "first", "pass tests", 1)
	s.store.RecordMetric(pid, "coverage", "80", "%")
	s.store.CreateKeyword(&pid, "kubernetes")
	s.store.CreateProfileSync(pid, "github")
	s.store.RecordReadiness(pid, true, false, true, 7)

	resp, err := http.Get(ts.URL + "/api/projects/1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var detail store.ProjectDetail
	json.NewDecoder(resp.Body).Decode(&detail)
	if detail.Name != "detail-proj" {
		t.Errorf("name = %q, want %q", detail.Name, "detail-proj")
	}
	if len(detail.Milestones) != 1 {
		t.Errorf("milestones = %d, want 1", len(detail.Milestones))
	}
	if len(detail.Metrics) != 1 {
		t.Errorf("metrics = %d, want 1", len(detail.Metrics))
	}
	if len(detail.Keywords) != 1 {
		t.Errorf("keywords = %d, want 1", len(detail.Keywords))
	}
	if len(detail.Syncs) != 1 {
		t.Errorf("syncs = %d, want 1", len(detail.Syncs))
	}
	if detail.Readiness == nil {
		t.Error("expected non-nil readiness")
	} else if detail.Readiness.SelfScore != 7 {
		t.Errorf("self_score = %d, want 7", detail.Readiness.SelfScore)
	}
}

func TestHandleGetProject_NotFound(t *testing.T) {
	_, ts := testServer(t)
	resp, err := http.Get(ts.URL + "/api/projects/999")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestHandleGetProject_InvalidID(t *testing.T) {
	_, ts := testServer(t)
	resp, err := http.Get(ts.URL + "/api/projects/abc")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// --- Update Project ---

func TestHandleUpdateProject_Valid(t *testing.T) {
	s, ts := testServer(t)
	seedProject(t, s.store, "update-proj")

	body, _ := json.Marshal(map[string]interface{}{"status": "active"})
	req, _ := http.NewRequest("PATCH", ts.URL+"/api/projects/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var project store.Project
	json.NewDecoder(resp.Body).Decode(&project)
	if project.Status != "active" {
		t.Errorf("status = %q, want %q", project.Status, "active")
	}
}

func TestHandleUpdateProject_NotFound(t *testing.T) {
	_, ts := testServer(t)
	body, _ := json.Marshal(map[string]interface{}{"status": "active"})
	req, _ := http.NewRequest("PATCH", ts.URL+"/api/projects/999", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestHandleUpdateProject_InvalidJSON(t *testing.T) {
	s, ts := testServer(t)
	seedProject(t, s.store, "bad-json-proj")

	req, _ := http.NewRequest("PATCH", ts.URL+"/api/projects/1", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// --- Update Milestone ---

func TestHandleUpdateMilestone_Valid(t *testing.T) {
	s, ts := testServer(t)
	pid := seedProject(t, s.store, "ms-proj")
	s.store.CreateMilestone(pid, "M1", "first", "pass", 1)

	body, _ := json.Marshal(map[string]string{"status": "done"})
	req, _ := http.NewRequest("PATCH", ts.URL+"/api/projects/1/milestones/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestHandleUpdateMilestone_MissingStatus(t *testing.T) {
	s, ts := testServer(t)
	pid := seedProject(t, s.store, "ms-proj2")
	s.store.CreateMilestone(pid, "M1", "first", "pass", 1)

	body, _ := json.Marshal(map[string]string{})
	req, _ := http.NewRequest("PATCH", ts.URL+"/api/projects/1/milestones/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestHandleUpdateMilestone_NotFound(t *testing.T) {
	s, ts := testServer(t)
	seedProject(t, s.store, "ms-proj3")

	body, _ := json.Marshal(map[string]string{"status": "done"})
	req, _ := http.NewRequest("PATCH", ts.URL+"/api/projects/1/milestones/999", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// --- Record Metric ---

func TestHandleRecordMetric_Valid(t *testing.T) {
	s, ts := testServer(t)
	seedProject(t, s.store, "metric-proj")

	body, _ := json.Marshal(map[string]string{"name": "latency", "value": "42ms", "unit": "ms"})
	resp, err := http.Post(ts.URL+"/api/projects/1/metrics", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["name"] != "latency" {
		t.Errorf("name = %v, want latency", result["name"])
	}
}

func TestHandleRecordMetric_MissingFields(t *testing.T) {
	s, ts := testServer(t)
	seedProject(t, s.store, "metric-proj2")

	body, _ := json.Marshal(map[string]string{"name": "latency"})
	resp, err := http.Post(ts.URL+"/api/projects/1/metrics", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// --- Sync Platform ---

func TestHandleSyncPlatform_Valid(t *testing.T) {
	s, ts := testServer(t)
	pid := seedProject(t, s.store, "sync-proj")
	s.store.CreateProfileSync(pid, "github")

	body, _ := json.Marshal(map[string]string{"platform": "github", "notes": "updated"})
	resp, err := http.Post(ts.URL+"/api/projects/1/syncs", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestHandleSyncPlatform_MissingPlatform(t *testing.T) {
	s, ts := testServer(t)
	seedProject(t, s.store, "sync-proj2")

	body, _ := json.Marshal(map[string]string{"notes": "nothing"})
	resp, err := http.Post(ts.URL+"/api/projects/1/syncs", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestHandleSyncPlatform_NotFound(t *testing.T) {
	s, ts := testServer(t)
	seedProject(t, s.store, "sync-proj3")

	body, _ := json.Marshal(map[string]string{"platform": "gitlab", "notes": "nothing"})
	resp, err := http.Post(ts.URL+"/api/projects/1/syncs", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// --- Record Readiness ---

func TestHandleRecordReadiness_Valid(t *testing.T) {
	s, ts := testServer(t)
	seedProject(t, s.store, "ready-proj")

	body, _ := json.Marshal(map[string]interface{}{
		"can_explain":   true,
		"can_demo":      false,
		"can_tradeoffs": true,
		"self_score":    8,
	})
	resp, err := http.Post(ts.URL+"/api/projects/1/readiness", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["self_score"] != float64(8) {
		t.Errorf("self_score = %v, want 8", result["self_score"])
	}
}

// --- Get Guide ---

func TestHandleGetGuide_NotFound(t *testing.T) {
	s, ts := testServer(t)
	seedProject(t, s.store, "guide-proj")

	resp, err := http.Get(ts.URL + "/api/projects/1/guide")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestHandleGetGuide_InvalidID(t *testing.T) {
	_, ts := testServer(t)
	resp, err := http.Get(ts.URL + "/api/projects/abc/guide")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// --- Tool Execute ---

func TestHandleToolExecute_Dashboard(t *testing.T) {
	s, ts := testServer(t)
	seedProject(t, s.store, "tool-proj")

	body, _ := json.Marshal(map[string]interface{}{"input": map[string]interface{}{}})
	resp, err := http.Post(ts.URL+"/api/tools/dashboard/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var result ToolResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if !result.Success {
		t.Errorf("success = false, error = %q", result.Error)
	}
}

func TestHandleToolExecute_DashboardKeywordsView(t *testing.T) {
	s, ts := testServer(t)
	pid := seedProject(t, s.store, "kw-tool-proj")
	s.store.CreateKeyword(&pid, "docker")

	body, _ := json.Marshal(map[string]interface{}{"input": map[string]interface{}{"view": "keywords"}})
	resp, err := http.Post(ts.URL+"/api/tools/dashboard/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var result ToolResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if !result.Success {
		t.Errorf("success = false, error = %q", result.Error)
	}
}

func TestHandleToolExecute_ProjectDetail(t *testing.T) {
	s, ts := testServer(t)
	seedProject(t, s.store, "detail-tool-proj")

	body, _ := json.Marshal(map[string]interface{}{
		"input": map[string]interface{}{"project_id": 1},
	})
	resp, err := http.Post(ts.URL+"/api/tools/project_detail/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var result ToolResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if !result.Success {
		t.Errorf("success = false, error = %q", result.Error)
	}
}

func TestHandleToolExecute_ProjectDetailByName(t *testing.T) {
	s, ts := testServer(t)
	seedProject(t, s.store, "named-proj")

	body, _ := json.Marshal(map[string]interface{}{
		"input": map[string]interface{}{"project_name": "named-proj"},
	})
	resp, err := http.Post(ts.URL+"/api/tools/project_detail/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var result ToolResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if !result.Success {
		t.Errorf("success = false, error = %q", result.Error)
	}
}

func TestHandleToolExecute_ProjectDetailMissingID(t *testing.T) {
	_, ts := testServer(t)

	body, _ := json.Marshal(map[string]interface{}{
		"input": map[string]interface{}{},
	})
	resp, err := http.Post(ts.URL+"/api/tools/project_detail/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var result ToolResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Success {
		t.Error("expected success = false for missing project_id")
	}
	if result.Error == "" {
		t.Error("expected error message")
	}
}

func TestHandleToolExecute_UpdateProgress(t *testing.T) {
	s, ts := testServer(t)
	seedProject(t, s.store, "progress-proj")

	body, _ := json.Marshal(map[string]interface{}{
		"input": map[string]interface{}{
			"project_id": 1,
			"status":     "active",
		},
	})
	resp, err := http.Post(ts.URL+"/api/tools/update_progress/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var result ToolResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if !result.Success {
		t.Errorf("success = false, error = %q", result.Error)
	}

	// Verify the project was updated.
	proj, _ := s.store.GetProject(1)
	if proj.Status != "active" {
		t.Errorf("status = %q, want %q", proj.Status, "active")
	}
}

func TestHandleToolExecute_UpdateProgressWithMilestone(t *testing.T) {
	s, ts := testServer(t)
	pid := seedProject(t, s.store, "ms-progress-proj")
	s.store.CreateMilestone(pid, "M1", "first", "pass", 1)

	body, _ := json.Marshal(map[string]interface{}{
		"input": map[string]interface{}{
			"project_id":       1,
			"milestone_id":     1,
			"milestone_status": "done",
		},
	})
	resp, err := http.Post(ts.URL+"/api/tools/update_progress/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var result ToolResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if !result.Success {
		t.Errorf("success = false, error = %q", result.Error)
	}
}

func TestHandleToolExecute_UpdateProgressWithReadiness(t *testing.T) {
	s, ts := testServer(t)
	seedProject(t, s.store, "ready-progress-proj")

	body, _ := json.Marshal(map[string]interface{}{
		"input": map[string]interface{}{
			"project_id":   1,
			"self_score":   9,
			"can_explain":  true,
			"can_demo":     true,
			"can_tradeoffs": true,
		},
	})
	resp, err := http.Post(ts.URL+"/api/tools/update_progress/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var result ToolResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if !result.Success {
		t.Errorf("success = false, error = %q", result.Error)
	}
}

func TestHandleToolExecute_UpdateProgressMissingID(t *testing.T) {
	_, ts := testServer(t)

	body, _ := json.Marshal(map[string]interface{}{
		"input": map[string]interface{}{},
	})
	resp, err := http.Post(ts.URL+"/api/tools/update_progress/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var result ToolResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Success {
		t.Error("expected success = false for missing project_id")
	}
}

func TestHandleToolExecute_RecordMetric(t *testing.T) {
	s, ts := testServer(t)
	seedProject(t, s.store, "metric-tool-proj")

	body, _ := json.Marshal(map[string]interface{}{
		"input": map[string]interface{}{
			"project_id": 1,
			"name":       "latency_p99",
			"value":      "120ms",
			"unit":       "ms",
		},
	})
	resp, err := http.Post(ts.URL+"/api/tools/record_metric/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var result ToolResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if !result.Success {
		t.Errorf("success = false, error = %q", result.Error)
	}
}

func TestHandleToolExecute_RecordMetricMissingFields(t *testing.T) {
	s, ts := testServer(t)
	seedProject(t, s.store, "metric-tool-proj2")

	body, _ := json.Marshal(map[string]interface{}{
		"input": map[string]interface{}{
			"project_id": 1,
			"name":       "latency",
		},
	})
	resp, err := http.Post(ts.URL+"/api/tools/record_metric/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var result ToolResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Success {
		t.Error("expected success = false for missing value")
	}
}

func TestHandleToolExecute_SyncProfile(t *testing.T) {
	s, ts := testServer(t)
	pid := seedProject(t, s.store, "sync-tool-proj")
	s.store.CreateProfileSync(pid, "linkedin")

	body, _ := json.Marshal(map[string]interface{}{
		"input": map[string]interface{}{
			"project_id": 1,
			"platform":   "linkedin",
			"notes":      "updated bio",
		},
	})
	resp, err := http.Post(ts.URL+"/api/tools/sync_profile/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var result ToolResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if !result.Success {
		t.Errorf("success = false, error = %q", result.Error)
	}
}

func TestHandleToolExecute_SyncProfileMissingPlatform(t *testing.T) {
	s, ts := testServer(t)
	seedProject(t, s.store, "sync-tool-proj2")

	body, _ := json.Marshal(map[string]interface{}{
		"input": map[string]interface{}{"project_id": 1},
	})
	resp, err := http.Post(ts.URL+"/api/tools/sync_profile/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var result ToolResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Success {
		t.Error("expected success = false for missing platform")
	}
}

func TestHandleToolExecute_UnknownTool(t *testing.T) {
	_, ts := testServer(t)

	body, _ := json.Marshal(map[string]interface{}{"input": map[string]string{}})
	resp, err := http.Post(ts.URL+"/api/tools/nonexistent/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestHandleToolExecute_InvalidJSON(t *testing.T) {
	_, ts := testServer(t)

	resp, err := http.Post(ts.URL+"/api/tools/dashboard/execute", "application/json", bytes.NewReader([]byte("bad")))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestHandleToolExecute_NilInput(t *testing.T) {
	s, ts := testServer(t)
	seedProject(t, s.store, "nil-input-proj")

	// Send body with no input field — should default to empty map.
	body, _ := json.Marshal(map[string]interface{}{})
	resp, err := http.Post(ts.URL+"/api/tools/dashboard/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// --- Middleware ---

func TestCspMiddleware(t *testing.T) {
	s, _ := testServer(t)
	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(w, req)

	if got := w.Header().Get("Content-Security-Policy"); got == "" {
		t.Error("expected CSP header")
	}
	if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := w.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("X-Frame-Options = %q, want DENY", got)
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	// Create a handler that panics, wrap in recovery.
	panicker := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})
	handler := recoveryMiddleware(panicker)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestBodyLimitMiddleware(t *testing.T) {
	// A handler that reads the full body.
	echoer := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		_, err := buf.ReadFrom(r.Body)
		if err != nil {
			http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.Write(buf.Bytes())
	})
	handler := bodyLimitMiddleware(16)(echoer)

	// Small body should pass.
	req := httptest.NewRequest("POST", "/", bytes.NewReader([]byte("hello")))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("small body: status = %d, want %d", w.Code, http.StatusOK)
	}

	// Large body should fail.
	bigBody := bytes.Repeat([]byte("x"), 100)
	req = httptest.NewRequest("POST", "/", bytes.NewReader(bigBody))
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code == http.StatusOK && w.Body.Len() == 100 {
		t.Error("expected body limit to reject large POST body")
	}
}

// --- Helpers ---

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusCreated, map[string]string{"key": "val"})
	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusNotFound, "missing")
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
	var body map[string]string
	json.NewDecoder(w.Body).Decode(&body)
	if body["error"] != "missing" {
		t.Errorf("error = %q, want missing", body["error"])
	}
}

func TestWithOptions(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "opts.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	s := New(WithStore(st), WithHost("0.0.0.0"), WithPort("9999"), WithContentDir("/tmp/guides"))
	if s.host != "0.0.0.0" {
		t.Errorf("host = %q, want %q", s.host, "0.0.0.0")
	}
	if s.port != "9999" {
		t.Errorf("port = %q, want 9999", s.port)
	}
	if s.contentDir != "/tmp/guides" {
		t.Errorf("contentDir = %q, want /tmp/guides", s.contentDir)
	}
}

func TestToInt(t *testing.T) {
	tests := []struct {
		input interface{}
		want  int
		err   bool
	}{
		{float64(42), 42, false},
		{int(7), 7, false},
		{"123", 123, false},
		{"abc", 0, true},
		{true, 0, true},
	}
	for _, tc := range tests {
		got, err := toInt(tc.input)
		if (err != nil) != tc.err {
			t.Errorf("toInt(%v): err=%v, wantErr=%v", tc.input, err, tc.err)
			continue
		}
		if got != tc.want {
			t.Errorf("toInt(%v) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestToFloat(t *testing.T) {
	tests := []struct {
		input interface{}
		want  float64
		err   bool
	}{
		{float64(3.14), 3.14, false},
		{int(7), 7.0, false},
		{"2.5", 2.5, false},
		{"abc", 0, true},
		{true, 0, true},
	}
	for _, tc := range tests {
		got, err := toFloat(tc.input)
		if (err != nil) != tc.err {
			t.Errorf("toFloat(%v): err=%v, wantErr=%v", tc.input, err, tc.err)
			continue
		}
		if got != tc.want {
			t.Errorf("toFloat(%v) = %f, want %f", tc.input, got, tc.want)
		}
	}
}
