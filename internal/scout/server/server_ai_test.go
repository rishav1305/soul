package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rishav1305/soul/internal/scout/store"
)

// --- Health ---

func TestHandleHealth(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "ok" {
		t.Errorf("status = %v, want ok", resp["status"])
	}
	if resp["service"] != "soul-scout" {
		t.Errorf("service = %v, want soul-scout", resp["service"])
	}
}

func TestHandleHealth_AlternatePath(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// --- WithOptions ---

func TestWithOptions_Scout(t *testing.T) {
	srv := New(
		WithHost("0.0.0.0"),
		WithPort(9999),
		WithDataDir("/tmp/test"),
		WithPgURL("postgres://localhost/test"),
		WithConfigPath("/tmp/sweep.json"),
	)
	if srv.host != "0.0.0.0" {
		t.Errorf("host = %q, want '0.0.0.0'", srv.host)
	}
	if srv.port != 9999 {
		t.Errorf("port = %d, want 9999", srv.port)
	}
	if srv.dataDir != "/tmp/test" {
		t.Errorf("dataDir = %q, want '/tmp/test'", srv.dataDir)
	}
	if srv.pgURL != "postgres://localhost/test" {
		t.Errorf("pgURL = %q", srv.pgURL)
	}
	if srv.configPath != "/tmp/sweep.json" {
		t.Errorf("configPath = %q", srv.configPath)
	}
}

// --- Sync ---

func TestHandleSync(t *testing.T) {
	srv := newTestServer(t)
	body, _ := json.Marshal(map[string]string{"platform": "linkedin"})
	req := httptest.NewRequest("POST", "/api/sync", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "checked" {
		t.Errorf("status = %v, want 'checked'", resp["status"])
	}
}

func TestHandleSync_InvalidJSON(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("POST", "/api/sync", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// --- Sweep ---

func TestHandleSweep_NoScheduler(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("POST", "/api/sweep", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleSweepNow_NoScheduler(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("POST", "/api/sweep/now", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Sweep Config ---

func TestHandleGetSweepConfig_NoPath(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/sweep/config", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestHandlePutSweepConfig_NoPath(t *testing.T) {
	srv := newTestServer(t)
	body, _ := json.Marshal(map[string]interface{}{})
	req := httptest.NewRequest("PUT", "/api/sweep/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestHandlePutSweepConfig_InvalidJSON(t *testing.T) {
	srv := newTestServer(t)
	srv.configPath = filepath.Join(t.TempDir(), "sweep.json")
	req := httptest.NewRequest("PUT", "/api/sweep/config", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandlePutSweepConfig_ValidationErrors(t *testing.T) {
	srv := newTestServer(t)
	srv.configPath = filepath.Join(t.TempDir(), "sweep.json")

	tests := []struct {
		name string
		body map[string]interface{}
	}{
		{"zero interval", map[string]interface{}{"interval_hours": 0}},
		{"zero limit", map[string]interface{}{"interval_hours": 6, "limit": 0}},
		{"zero budget", map[string]interface{}{"interval_hours": 6, "limit": 10, "credit_budget": 0}},
		{"zero age", map[string]interface{}{"interval_hours": 6, "limit": 10, "credit_budget": 100, "posted_at_max_age_days": 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("PUT", "/api/sweep/config", bytes.NewReader(b))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestHandlePutSweepConfig_HappyPath(t *testing.T) {
	srv := newTestServer(t)
	cfgPath := filepath.Join(t.TempDir(), "sweep.json")
	srv.configPath = cfgPath

	body, _ := json.Marshal(map[string]interface{}{
		"interval_hours":         6,
		"limit":                  10,
		"credit_budget":          100,
		"posted_at_max_age_days": 7,
	})
	req := httptest.NewRequest("PUT", "/api/sweep/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	// Verify file was written.
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Error("expected sweep config file to be created")
	}
}

func TestHandleGetSweepConfig_HappyPath(t *testing.T) {
	srv := newTestServer(t)
	cfgPath := filepath.Join(t.TempDir(), "sweep.json")
	srv.configPath = cfgPath

	// Write a valid config file first.
	os.WriteFile(cfgPath, []byte(`{"interval_hours":6,"limit":10,"credit_budget":100,"posted_at_max_age_days":7}`), 0644)

	req := httptest.NewRequest("GET", "/api/sweep/config", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Profile ---

func TestHandleProfilePull_NoProfileDB(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("POST", "/api/profile/pull", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestHandleProfilePush_NoProfileDB(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("POST", "/api/profile/push", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Optimizations ---

func TestHandleAddOptimization(t *testing.T) {
	srv := newTestServer(t)
	body, _ := json.Marshal(store.Optimization{
		Platform: "linkedin",
		Section:  "headline",
		Previous: "Go Developer",
		Updated:  "Senior Go Engineer | Cloud Infrastructure",
	})
	req := httptest.NewRequest("POST", "/api/optimizations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleAddOptimization_InvalidJSON(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("POST", "/api/optimizations", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleLaunchOptimizer(t *testing.T) {
	srv := newTestServer(t)
	body, _ := json.Marshal(map[string]string{"platform": "linkedin"})
	req := httptest.NewRequest("POST", "/api/optimize", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleLaunchOptimizer_MissingPlatform(t *testing.T) {
	srv := newTestServer(t)
	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest("POST", "/api/optimize", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleApplyOptimization(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("POST", "/api/optimize/apply", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// --- Helper functions ---

func TestParseID(t *testing.T) {
	tests := []struct {
		name   string
		input  map[string]interface{}
		key    string
		expect int64
	}{
		{"float64", map[string]interface{}{"id": float64(42)}, "id", 42},
		{"string", map[string]interface{}{"id": "99"}, "id", 99},
		{"json.Number", map[string]interface{}{"id": json.Number("7")}, "id", 7},
		{"missing key", map[string]interface{}{}, "id", 0},
		{"wrong type", map[string]interface{}{"id": true}, "id", 0},
		{"bad string", map[string]interface{}{"id": "abc"}, "id", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseID(tt.input, tt.key)
			if got != tt.expect {
				t.Errorf("parseID() = %d, want %d", got, tt.expect)
			}
		})
	}
}

func TestAsyncErrorStatus(t *testing.T) {
	tests := []struct {
		errMsg string
		want   int
	}{
		{"queue full", http.StatusServiceUnavailable},
		{"max 3 concurrent agents", http.StatusServiceUnavailable},
		{"lead not found", http.StatusNotFound},
		{"no rows in result", http.StatusNotFound},
		{"unexpected error", http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.errMsg, func(t *testing.T) {
			got := asyncErrorStatus(errors.New(tt.errMsg))
			if got != tt.want {
				t.Errorf("asyncErrorStatus(%q) = %d, want %d", tt.errMsg, got, tt.want)
			}
		})
	}
}

func TestAiErrorStatus(t *testing.T) {
	tests := []struct {
		errMsg string
		want   int
	}{
		{"lead not found", http.StatusNotFound},
		{"no rows", http.StatusNotFound},
		{"invalid platform", http.StatusBadRequest},
		{"profiledb not configured", http.StatusBadRequest},
		{"timeout", http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.errMsg, func(t *testing.T) {
			got := aiErrorStatus(errors.New(tt.errMsg))
			if got != tt.want {
				t.Errorf("aiErrorStatus(%q) = %d, want %d", tt.errMsg, got, tt.want)
			}
		})
	}
}

func TestEmptyDigest(t *testing.T) {
	d := emptyDigest()
	if d == nil {
		t.Fatal("emptyDigest returned nil")
	}
	if d["new_leads"] != 0 {
		t.Errorf("new_leads = %v, want 0", d["new_leads"])
	}
}

// --- AI Tool Endpoints: Table-driven validation ---
// Tests the 3 guard conditions: invalid JSON → 400, missing required field → 400, nil aiService → 503.

// aiEndpoint defines an AI endpoint to test.
type aiEndpoint struct {
	path         string
	validBody    map[string]interface{} // Body that passes validation (will hit 503 since aiService is nil)
	missingField map[string]interface{} // Body missing the required field (will hit 400)
}

func aiLeadEndpoint(path string) aiEndpoint {
	return aiEndpoint{
		path:         path,
		validBody:    map[string]interface{}{"lead_id": float64(1)},
		missingField: map[string]interface{}{"lead_id": float64(0)},
	}
}

func TestAIEndpoints_InvalidJSON(t *testing.T) {
	endpoints := []string{
		"/api/ai/match", "/api/ai/proposal", "/api/ai/cover-letter",
		"/api/ai/outreach", "/api/ai/salary", "/api/ai/referral",
		"/api/ai/pitch", "/api/ai/resume-tailor", "/api/ai/freelance-score",
		"/api/ai/networking-draft", "/api/ai/content-series",
		"/api/ai/hook-writer", "/api/ai/content-topic",
		"/api/ai/expert-application", "/api/ai/call-prep",
		"/api/ai/sow", "/api/ai/contract-followup", "/api/ai/case-study",
		"/api/ai/consulting-followup", "/api/ai/advisory-proposal",
		"/api/ai/project-proposal", "/api/ai/consulting-upsell",
		"/api/ai/thread-converter", "/api/ai/substack-expander",
		"/api/ai/reactive-content", "/api/ai/engagement-reply",
		"/api/ai/content-metrics", "/api/ai/linkedin-update",
		"/api/ai/github-readme", "/api/ai/profile-audit",
		"/api/ai/testimonial-request", "/api/ai/pin-recommendation",
		"/api/ai/contract-upsell",
	}

	srv := newTestServer(t)
	for _, ep := range endpoints {
		t.Run("InvalidJSON_"+ep, func(t *testing.T) {
			req := httptest.NewRequest("POST", ep, bytes.NewReader([]byte("not json")))
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("%s invalid JSON: expected 400, got %d", ep, w.Code)
			}
		})
	}
}

func TestAIEndpoints_MissingRequiredField(t *testing.T) {
	// Endpoints that require lead_id > 0.
	leadEndpoints := []string{
		"/api/ai/match", "/api/ai/proposal", "/api/ai/cover-letter",
		"/api/ai/outreach", "/api/ai/salary", "/api/ai/referral",
		"/api/ai/pitch", "/api/ai/resume-tailor", "/api/ai/freelance-score",
		"/api/ai/networking-draft", "/api/ai/call-prep",
		"/api/ai/sow", "/api/ai/contract-followup", "/api/ai/case-study",
		"/api/ai/consulting-followup", "/api/ai/advisory-proposal",
		"/api/ai/project-proposal", "/api/ai/consulting-upsell",
		"/api/ai/testimonial-request", "/api/ai/contract-upsell",
	}

	srv := newTestServer(t)
	for _, ep := range leadEndpoints {
		t.Run("MissingLeadID_"+ep, func(t *testing.T) {
			body, _ := json.Marshal(map[string]interface{}{"lead_id": float64(0)})
			req := httptest.NewRequest("POST", ep, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("%s missing lead_id: expected 400, got %d: %s", ep, w.Code, w.Body.String())
			}
		})
	}

	// Endpoints with non-lead_id required fields.
	otherEndpoints := []struct {
		path string
		body map[string]interface{}
	}{
		{"/api/ai/content-series", map[string]interface{}{"topic": ""}},
		{"/api/ai/hook-writer", map[string]interface{}{"draft": ""}},
		{"/api/ai/content-topic", map[string]interface{}{"week_summary": ""}},
		{"/api/ai/expert-application", map[string]interface{}{"network_name": ""}},
		{"/api/ai/thread-converter", map[string]interface{}{"post": ""}},
		{"/api/ai/substack-expander", map[string]interface{}{"post": ""}},
		{"/api/ai/reactive-content", map[string]interface{}{"news_context": ""}},
		{"/api/ai/engagement-reply", map[string]interface{}{"post_content": ""}},
		{"/api/ai/linkedin-update", map[string]interface{}{"section": "", "current_content": ""}},
		{"/api/ai/github-readme", map[string]interface{}{"repo_name": "", "description": ""}},
		{"/api/ai/profile-audit", map[string]interface{}{"platform": "", "current_profile": ""}},
		{"/api/ai/pin-recommendation", map[string]interface{}{"platform": ""}},
	}
	for _, tt := range otherEndpoints {
		t.Run("MissingField_"+tt.path, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("POST", tt.path, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("%s missing field: expected 400, got %d: %s", tt.path, w.Code, w.Body.String())
			}
		})
	}
}

func TestAIEndpoints_NilAIService(t *testing.T) {
	// Endpoints that require lead_id and have validation before AI check.
	leadEndpoints := []struct {
		path string
		body map[string]interface{}
	}{
		{"/api/ai/match", map[string]interface{}{"lead_id": float64(1)}},
		{"/api/ai/proposal", map[string]interface{}{"lead_id": float64(1)}},
		{"/api/ai/cover-letter", map[string]interface{}{"lead_id": float64(1)}},
		{"/api/ai/outreach", map[string]interface{}{"lead_id": float64(1)}},
		{"/api/ai/salary", map[string]interface{}{"lead_id": float64(1)}},
		{"/api/ai/referral", map[string]interface{}{"lead_id": float64(1)}},
		{"/api/ai/pitch", map[string]interface{}{"lead_id": float64(1)}},
		{"/api/ai/resume-tailor", map[string]interface{}{"lead_id": float64(1)}},
		{"/api/ai/freelance-score", map[string]interface{}{"lead_id": float64(1)}},
		{"/api/ai/networking-draft", map[string]interface{}{"lead_id": float64(1)}},
		{"/api/ai/call-prep", map[string]interface{}{"lead_id": float64(1)}},
		{"/api/ai/sow", map[string]interface{}{"lead_id": float64(1)}},
		{"/api/ai/contract-followup", map[string]interface{}{"lead_id": float64(1)}},
		{"/api/ai/case-study", map[string]interface{}{"lead_id": float64(1)}},
		{"/api/ai/consulting-followup", map[string]interface{}{"lead_id": float64(1)}},
		{"/api/ai/advisory-proposal", map[string]interface{}{"lead_id": float64(1)}},
		{"/api/ai/project-proposal", map[string]interface{}{"lead_id": float64(1)}},
		{"/api/ai/consulting-upsell", map[string]interface{}{"lead_id": float64(1)}},
		{"/api/ai/testimonial-request", map[string]interface{}{"lead_id": float64(1)}},
		{"/api/ai/contract-upsell", map[string]interface{}{"lead_id": float64(1)}},
		{"/api/ai/content-series", map[string]interface{}{"topic": "AI"}},
		{"/api/ai/hook-writer", map[string]interface{}{"draft": "test draft"}},
		{"/api/ai/content-topic", map[string]interface{}{"week_summary": "summary"}},
		{"/api/ai/expert-application", map[string]interface{}{"network_name": "GLG"}},
		{"/api/ai/thread-converter", map[string]interface{}{"post": "test post"}},
		{"/api/ai/substack-expander", map[string]interface{}{"post": "test post"}},
		{"/api/ai/reactive-content", map[string]interface{}{"news_context": "GPT-5 launch"}},
		{"/api/ai/engagement-reply", map[string]interface{}{"post_content": "test post"}},
		{"/api/ai/content-metrics", map[string]interface{}{"platform": "linkedin"}},
		{"/api/ai/linkedin-update", map[string]interface{}{"section": "headline", "current_content": "test"}},
		{"/api/ai/github-readme", map[string]interface{}{"repo_name": "test", "description": "test"}},
		{"/api/ai/profile-audit", map[string]interface{}{"platform": "linkedin", "current_profile": "test"}},
		{"/api/ai/pin-recommendation", map[string]interface{}{"platform": "linkedin"}},
	}

	srv := newTestServer(t)
	// Ensure aiService is nil (default from newTestServer).

	for _, tt := range leadEndpoints {
		t.Run("NilAI_"+tt.path, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("POST", tt.path, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)
			if w.Code != http.StatusServiceUnavailable {
				t.Errorf("%s nil AI: expected 503, got %d: %s", tt.path, w.Code, w.Body.String())
			}
		})
	}
}

// Test the networking-brief endpoint (no body required, just nil AI check).
func TestAINetworkingBrief_NilAI(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("POST", "/api/ai/networking-brief", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

// --- CORS edge cases ---

func TestCorsMiddleware_DisallowedOrigin(t *testing.T) {
	srv := newTestServer(t)
	handler := corsMiddleware(srv.mux)

	req := httptest.NewRequest("GET", "/api/health", nil)
	req.Header.Set("Origin", "http://evil.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no CORS header for disallowed origin, got %q", got)
	}
}

func TestCorsMiddleware_NoOrigin(t *testing.T) {
	srv := newTestServer(t)
	handler := corsMiddleware(srv.mux)

	req := httptest.NewRequest("GET", "/api/health", nil)
	// No Origin header.
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no CORS header for missing origin, got %q", got)
	}
}

func TestCorsMiddleware_AllAllowedOrigins(t *testing.T) {
	srv := newTestServer(t)
	handler := corsMiddleware(srv.mux)

	origins := []string{
		"http://127.0.0.1:3002",
		"http://localhost:3002",
		"http://127.0.0.1:5173",
		"http://localhost:5173",
	}
	for _, origin := range origins {
		t.Run(origin, func(t *testing.T) {
			req := httptest.NewRequest("OPTIONS", "/api/health", nil)
			req.Header.Set("Origin", origin)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusNoContent {
				t.Errorf("expected 204, got %d", w.Code)
			}
			if got := w.Header().Get("Access-Control-Allow-Origin"); got != origin {
				t.Errorf("ACAO = %q, want %q", got, origin)
			}
		})
	}
}

// --- Agent edge cases ---

func TestHandleAgentStatus_WithRunID(t *testing.T) {
	srv := newTestServer(t)
	// Query with a run_id that doesn't exist.
	req := httptest.NewRequest("GET", "/api/agent/status?run_id=999", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleAgentStatus_InvalidRunID(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/agent/status?run_id=abc", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleAgentHistory_WithFilter(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/agent/history?platform=linkedin", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// --- Tool Dispatch ---

func TestHandleToolExecute_LeadList(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	srv.store.AddLead(store.Lead{JobTitle: "SWE", Company: "Acme"})

	body, _ := json.Marshal(map[string]interface{}{})
	resp, err := http.Post(ts.URL+"/api/tools/lead_list/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHandleToolExecute_LeadGet(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	srv.store.AddLead(store.Lead{JobTitle: "SWE", Company: "Acme"})

	body, _ := json.Marshal(map[string]interface{}{"lead_id": float64(1)})
	resp, err := http.Post(ts.URL+"/api/tools/lead_get/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHandleToolExecute_LeadGetMissing(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{})
	resp, err := http.Post(ts.URL+"/api/tools/lead_get/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestHandleToolExecute_LeadAdd(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{"jobTitle": "SWE", "company": "Acme"})
	resp, err := http.Post(ts.URL+"/api/tools/lead_add/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}
}

func TestHandleToolExecute_LeadUpdate(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	srv.store.AddLead(store.Lead{JobTitle: "SWE", Company: "Acme"})

	body, _ := json.Marshal(map[string]interface{}{"lead_id": float64(1), "company": "NewCo"})
	resp, err := http.Post(ts.URL+"/api/tools/lead_update/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHandleToolExecute_LeadUpdateMissingID(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{"company": "NewCo"})
	resp, err := http.Post(ts.URL+"/api/tools/lead_update/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestHandleToolExecute_LeadUpdateNoFields(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	srv.store.AddLead(store.Lead{JobTitle: "SWE", Company: "Acme"})

	body, _ := json.Marshal(map[string]interface{}{"lead_id": float64(1)})
	resp, err := http.Post(ts.URL+"/api/tools/lead_update/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestHandleToolExecute_LeadAction(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	srv.store.AddLead(store.Lead{JobTitle: "SWE", Company: "Co", Pipeline: "job", Stage: "discovered"})

	body, _ := json.Marshal(map[string]interface{}{"lead_id": float64(1), "action": "qualified", "notes": "looks good"})
	resp, err := http.Post(ts.URL+"/api/tools/lead_action/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHandleToolExecute_Analytics(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{})
	resp, err := http.Post(ts.URL+"/api/tools/analytics/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHandleToolExecute_Sync(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{"platform": "linkedin"})
	resp, err := http.Post(ts.URL+"/api/tools/sync/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHandleToolExecute_SweepStatus(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{})
	resp, err := http.Post(ts.URL+"/api/tools/sweep_status/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHandleToolExecute_SweepDigest(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{})
	resp, err := http.Post(ts.URL+"/api/tools/sweep_digest/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHandleToolExecute_ProfilePush(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{})
	resp, err := http.Post(ts.URL+"/api/tools/profile_push/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHandleToolExecute_OptimizationList(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{})
	resp, err := http.Post(ts.URL+"/api/tools/optimization_list/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHandleToolExecute_OptimizeProfile(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{})
	resp, err := http.Post(ts.URL+"/api/tools/optimize_profile/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHandleToolExecute_OptimizeApply(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{})
	resp, err := http.Post(ts.URL+"/api/tools/optimize_apply/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHandleToolExecute_AgentStatus(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{})
	resp, err := http.Post(ts.URL+"/api/tools/agent_status/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHandleToolExecute_AgentHistory(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{})
	resp, err := http.Post(ts.URL+"/api/tools/agent_history/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHandleToolExecute_ScoredLeads(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{})
	resp, err := http.Post(ts.URL+"/api/tools/scored_leads/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHandleToolExecute_Unknown(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{})
	resp, err := http.Post(ts.URL+"/api/tools/nonexistent/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestHandleToolExecute_InvalidJSON(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/tools/lead_list/execute", "application/json", bytes.NewReader([]byte("not json")))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestHandleToolExecute_NilBody(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/tools/lead_list/execute", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}
