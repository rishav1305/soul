package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rishav1305/soul/internal/scout/store"
)

// --- Leads CRUD ---

func TestHandleAddLead(t *testing.T) {
	srv := newTestServer(t)

	body, _ := json.Marshal(map[string]interface{}{
		"jobTitle": "Go Engineer",
		"company":  "Acme Corp",
		"pipeline": "job",
		"stage":    "discovered",
	})
	req := httptest.NewRequest("POST", "/api/leads", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("POST /api/leads: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var lead store.Lead
	json.NewDecoder(w.Body).Decode(&lead)
	if lead.JobTitle != "Go Engineer" {
		t.Errorf("job_title = %q, want 'Go Engineer'", lead.JobTitle)
	}
	if lead.Company != "Acme Corp" {
		t.Errorf("company = %q, want 'Acme Corp'", lead.Company)
	}
}

func TestHandleAddLead_MissingFields(t *testing.T) {
	srv := newTestServer(t)

	body, _ := json.Marshal(map[string]interface{}{})
	req := httptest.NewRequest("POST", "/api/leads", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("POST /api/leads with empty body: expected 400, got %d", w.Code)
	}
}

func TestHandleAddLead_InvalidJSON(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("POST", "/api/leads", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("POST /api/leads with invalid JSON: expected 400, got %d", w.Code)
	}
}

func TestHandleListLeads_Empty(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/leads", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/leads: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var leads []store.Lead
	json.NewDecoder(w.Body).Decode(&leads)
	if leads == nil {
		t.Error("expected non-nil leads slice")
	}
	if len(leads) != 0 {
		t.Errorf("expected 0 leads, got %d", len(leads))
	}
}

func TestHandleListLeads_WithData(t *testing.T) {
	srv := newTestServer(t)
	// Add 2 leads.
	srv.store.AddLead(store.Lead{JobTitle: "SWE", Company: "A", Pipeline: "job", Stage: "discovered"})
	srv.store.AddLead(store.Lead{JobTitle: "SRE", Company: "B", Pipeline: "job", Stage: "discovered"})

	req := httptest.NewRequest("GET", "/api/leads", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var leads []store.Lead
	json.NewDecoder(w.Body).Decode(&leads)
	if len(leads) != 2 {
		t.Errorf("expected 2 leads, got %d", len(leads))
	}
}

func TestHandleListLeads_PipelineFilter(t *testing.T) {
	srv := newTestServer(t)
	srv.store.AddLead(store.Lead{JobTitle: "SWE", Company: "A", Pipeline: "job", Stage: "discovered"})
	srv.store.AddLead(store.Lead{JobTitle: "Gig", Company: "B", Pipeline: "freelance", Stage: "found"})

	req := httptest.NewRequest("GET", "/api/leads?pipeline=freelance", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var leads []store.Lead
	json.NewDecoder(w.Body).Decode(&leads)
	if len(leads) != 1 {
		t.Errorf("expected 1 lead with pipeline=freelance, got %d", len(leads))
	}
}

func TestHandleGetLead_Valid(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	srv.store.AddLead(store.Lead{JobTitle: "SWE", Company: "Acme"})

	resp, err := http.Get(ts.URL + "/api/leads/1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /api/leads/1: expected 200, got %d", resp.StatusCode)
	}
	var lead store.Lead
	json.NewDecoder(resp.Body).Decode(&lead)
	if lead.JobTitle != "SWE" {
		t.Errorf("job_title = %q, want 'SWE'", lead.JobTitle)
	}
}

func TestHandleGetLead_NotFound(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/leads/999")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET /api/leads/999: expected 404, got %d", resp.StatusCode)
	}
}

func TestHandleGetLead_InvalidID(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/leads/abc")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("GET /api/leads/abc: expected 400, got %d", resp.StatusCode)
	}
}

func TestHandleUpdateLead(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	srv.store.AddLead(store.Lead{JobTitle: "SWE", Company: "OldCo"})

	body, _ := json.Marshal(map[string]interface{}{"company": "NewCo"})
	req, _ := http.NewRequest("PATCH", ts.URL+"/api/leads/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("PATCH /api/leads/1: expected 200, got %d", resp.StatusCode)
	}
	var lead store.Lead
	json.NewDecoder(resp.Body).Decode(&lead)
	if lead.Company != "NewCo" {
		t.Errorf("company = %q, want 'NewCo'", lead.Company)
	}
}

func TestHandleUpdateLead_NotFound(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{"company": "X"})
	req, _ := http.NewRequest("PATCH", ts.URL+"/api/leads/999", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("PATCH /api/leads/999: expected 404, got %d", resp.StatusCode)
	}
}

func TestHandleScoredLeads_Empty(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/leads/scored", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/leads/scored: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var leads []store.Lead
	json.NewDecoder(w.Body).Decode(&leads)
	if leads == nil {
		t.Error("expected non-nil leads slice")
	}
}

func TestHandleScoredLeads_WithLimit(t *testing.T) {
	srv := newTestServer(t)
	for i := 0; i < 5; i++ {
		srv.store.AddLead(store.Lead{JobTitle: "SWE", Company: "Co"})
	}

	req := httptest.NewRequest("GET", "/api/leads/scored?limit=3", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var leads []store.Lead
	json.NewDecoder(w.Body).Decode(&leads)
	if len(leads) > 3 {
		t.Errorf("expected at most 3 leads, got %d", len(leads))
	}
}

// --- Record Action ---

func TestHandleRecordAction_StageChange(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	srv.store.AddLead(store.Lead{JobTitle: "SWE", Company: "Co", Pipeline: "job", Stage: "discovered"})

	body, _ := json.Marshal(map[string]interface{}{
		"action": "qualified",
		"notes":  "good fit",
	})
	resp, err := http.Post(ts.URL+"/api/leads/1/action", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/leads/1/action: expected 200, got %d", resp.StatusCode)
	}
	var lead store.Lead
	json.NewDecoder(resp.Body).Decode(&lead)
	if lead.Stage != "qualified" {
		t.Errorf("stage = %q, want 'qualified'", lead.Stage)
	}
}

func TestHandleRecordAction_NotFound(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{"action": "qualified"})
	resp, err := http.Post(ts.URL+"/api/leads/999/action", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// --- Analytics ---

func TestHandleAnalytics(t *testing.T) {
	srv := newTestServer(t)
	srv.store.AddLead(store.Lead{JobTitle: "SWE", Company: "A", Pipeline: "job", Stage: "discovered"})
	srv.store.AddLead(store.Lead{JobTitle: "SRE", Company: "B", Pipeline: "job", Stage: "qualified"})

	req := httptest.NewRequest("GET", "/api/analytics", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/analytics: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var analytics map[string]interface{}
	json.NewDecoder(w.Body).Decode(&analytics)
	if analytics == nil {
		t.Fatal("expected non-nil analytics")
	}
}

func TestHandleAnalytics_PipelineFilter(t *testing.T) {
	srv := newTestServer(t)
	srv.store.AddLead(store.Lead{JobTitle: "SWE", Company: "A", Pipeline: "job", Stage: "discovered"})
	srv.store.AddLead(store.Lead{JobTitle: "Gig", Company: "B", Pipeline: "freelance", Stage: "found"})

	req := httptest.NewRequest("GET", "/api/analytics?pipeline=freelance", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Agent ---

func TestHandleAgentStatus(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/agent/status", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

func TestHandleAgentHistory(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/agent/history", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Optimizations ---

func TestHandleListOptimizations_Empty(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/optimizations", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Sweep ---

func TestHandleSweepStatus(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/sweep/status", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleSweepDigest(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/sweep/digest", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Profile ---

func TestHandleGetProfile(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/profile", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// Profile returns 503 when profiledb is not configured (no SOUL_SCOUT_PG_URL).
	if w.Code != http.StatusOK && w.Code != http.StatusServiceUnavailable && w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 200, 503, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Middleware ---

func TestCorsMiddleware(t *testing.T) {
	srv := newTestServer(t)
	// Build the full middleware chain.
	handler := http.Handler(srv.mux)
	handler = corsMiddleware(handler)

	req := httptest.NewRequest("OPTIONS", "/api/leads", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("OPTIONS: expected 204, got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got == "" {
		t.Error("expected CORS origin header")
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	panicker := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})
	handler := recoveryMiddleware(panicker)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// --- Helpers ---

func TestWriteJSON_Scout(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusCreated, map[string]string{"key": "val"})
	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestWriteError_Scout(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusNotFound, "missing")
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}
