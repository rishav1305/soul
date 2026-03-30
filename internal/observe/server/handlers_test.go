package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rishav1305/soul/internal/chat/metrics"
)

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func findConstraint(p pillarResult, name string) *pillarConstraint {
	for i, c := range p.Constraints {
		if c.Name == name {
			return &p.Constraints[i]
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// countStatuses
// ---------------------------------------------------------------------------

func TestCountStatuses(t *testing.T) {
	p := pillarResult{
		Constraints: []pillarConstraint{
			{Status: "pass"},
			{Status: "pass"},
			{Status: "warn"},
			{Status: "fail"},
			{Status: "static"},
			{Status: "static"},
		},
	}
	countStatuses(&p)
	if p.Pass != 2 {
		t.Errorf("Pass = %d, want 2", p.Pass)
	}
	if p.Warn != 1 {
		t.Errorf("Warn = %d, want 1", p.Warn)
	}
	if p.Fail != 1 {
		t.Errorf("Fail = %d, want 1", p.Fail)
	}
	if p.Static != 2 {
		t.Errorf("Static = %d, want 2", p.Static)
	}
}

func TestCountStatuses_EmptyConstraints(t *testing.T) {
	p := pillarResult{}
	countStatuses(&p)
	if p.Pass != 0 || p.Warn != 0 || p.Fail != 0 || p.Static != 0 {
		t.Error("expected all zero counts for empty constraints")
	}
}

// ---------------------------------------------------------------------------
// buildPerformantPillar
// ---------------------------------------------------------------------------

func TestBuildPerformantPillar_NilInputs(t *testing.T) {
	dir := t.TempDir()
	logger, err := metrics.NewEventLogger(dir, "")
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	defer logger.Close()
	agg := metrics.NewAggregator(dir)

	result := buildPerformantPillar(nil, nil, nil, agg)

	if result.Name != "performant" {
		t.Errorf("Name = %q, want %q", result.Name, "performant")
	}
	if len(result.Constraints) != 4 {
		t.Errorf("expected 4 constraints, got %d", len(result.Constraints))
	}

	// All should show "no data" value when nil
	for _, c := range result.Constraints {
		if c.Status == "fail" {
			t.Errorf("constraint %q should not fail with nil input, got status=%q", c.Name, c.Status)
		}
	}
}

func TestBuildPerformantPillar_FastLatency(t *testing.T) {
	dir := t.TempDir()
	logger, err := metrics.NewEventLogger(dir, "")
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	defer logger.Close()
	agg := metrics.NewAggregator(dir)

	latency := &metrics.LatencyReport{
		FirstTokenP50: 100 * time.Millisecond,
		SampleCount:   10,
	}
	result := buildPerformantPillar(latency, nil, nil, agg)
	c := findConstraint(result, "first-token-p50")
	if c == nil {
		t.Fatal("first-token-p50 constraint missing")
	}
	if c.Status != "pass" {
		t.Errorf("first-token-p50 status = %q, want pass (100ms < 200ms)", c.Status)
	}
	if c.Value != "100ms" {
		t.Errorf("first-token-p50 value = %q, want 100ms", c.Value)
	}
}

func TestBuildPerformantPillar_SlowLatency_Warn(t *testing.T) {
	dir := t.TempDir()
	logger, err := metrics.NewEventLogger(dir, "")
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	defer logger.Close()
	agg := metrics.NewAggregator(dir)

	latency := &metrics.LatencyReport{
		FirstTokenP50: 160 * time.Millisecond,
		SampleCount:   5,
	}
	result := buildPerformantPillar(latency, nil, nil, agg)
	c := findConstraint(result, "first-token-p50")
	if c == nil {
		t.Fatal("first-token-p50 constraint missing")
	}
	if c.Status != "warn" {
		t.Errorf("first-token-p50 status = %q, want warn (160ms > 150ms)", c.Status)
	}
}

func TestBuildPerformantPillar_VerySlowLatency_Fail(t *testing.T) {
	dir := t.TempDir()
	logger, err := metrics.NewEventLogger(dir, "")
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	defer logger.Close()
	agg := metrics.NewAggregator(dir)

	latency := &metrics.LatencyReport{
		FirstTokenP50: 250 * time.Millisecond,
		SampleCount:   5,
	}
	result := buildPerformantPillar(latency, nil, nil, agg)
	c := findConstraint(result, "first-token-p50")
	if c == nil {
		t.Fatal("first-token-p50 constraint missing")
	}
	if c.Status != "fail" {
		t.Errorf("first-token-p50 status = %q, want fail (250ms > 200ms)", c.Status)
	}
}

func TestBuildPerformantPillar_FastDB(t *testing.T) {
	dir := t.TempDir()
	logger, err := metrics.NewEventLogger(dir, "")
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	defer logger.Close()
	agg := metrics.NewAggregator(dir)

	db := &metrics.DBReport{
		Methods: map[string]*metrics.MethodStats{
			"GetSession":   {Method: "GetSession", P50: 20 * time.Millisecond},
			"ListSessions": {Method: "ListSessions", P50: 45 * time.Millisecond},
		},
	}
	result := buildPerformantPillar(nil, db, nil, agg)
	c := findConstraint(result, "db-query-p50")
	if c == nil {
		t.Fatal("db-query-p50 constraint missing")
	}
	if c.Status != "pass" {
		t.Errorf("db-query-p50 status = %q, want pass (max 45ms < 100ms)", c.Status)
	}
	if c.Value != "45ms" {
		t.Errorf("db-query-p50 value = %q, want 45ms", c.Value)
	}
}

func TestBuildPerformantPillar_SlowDB_Fail(t *testing.T) {
	dir := t.TempDir()
	logger, err := metrics.NewEventLogger(dir, "")
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	defer logger.Close()
	agg := metrics.NewAggregator(dir)

	db := &metrics.DBReport{
		Methods: map[string]*metrics.MethodStats{
			"HeavyQuery": {Method: "HeavyQuery", P50: 150 * time.Millisecond},
		},
	}
	result := buildPerformantPillar(nil, db, nil, agg)
	c := findConstraint(result, "db-query-p50")
	if c == nil {
		t.Fatal("db-query-p50 constraint missing")
	}
	if c.Status != "fail" {
		t.Errorf("db-query-p50 status = %q, want fail (150ms > 100ms)", c.Status)
	}
}

func TestBuildPerformantPillar_FastHTTP(t *testing.T) {
	dir := t.TempDir()
	logger, err := metrics.NewEventLogger(dir, "")
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	defer logger.Close()
	agg := metrics.NewAggregator(dir)

	requests := &metrics.RequestsReport{
		Paths: map[string]*metrics.PathStats{
			"/api/sessions": {Path: "/api/sessions", P50: 100 * time.Millisecond},
		},
	}
	result := buildPerformantPillar(nil, nil, requests, agg)
	c := findConstraint(result, "http-request-p50")
	if c == nil {
		t.Fatal("http-request-p50 constraint missing")
	}
	if c.Status != "pass" {
		t.Errorf("http-request-p50 status = %q, want pass (100ms < 500ms)", c.Status)
	}
}

func TestBuildPerformantPillar_SlowHTTP_Fail(t *testing.T) {
	dir := t.TempDir()
	logger, err := metrics.NewEventLogger(dir, "")
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	defer logger.Close()
	agg := metrics.NewAggregator(dir)

	requests := &metrics.RequestsReport{
		Paths: map[string]*metrics.PathStats{
			"/api/slow": {Path: "/api/slow", P50: 600 * time.Millisecond},
		},
	}
	result := buildPerformantPillar(nil, nil, requests, agg)
	c := findConstraint(result, "http-request-p50")
	if c == nil {
		t.Fatal("http-request-p50 constraint missing")
	}
	if c.Status != "fail" {
		t.Errorf("http-request-p50 status = %q, want fail (600ms > 500ms)", c.Status)
	}
}

// ---------------------------------------------------------------------------
// buildRobustPillar
// ---------------------------------------------------------------------------

func TestBuildRobustPillar_NilFrontend(t *testing.T) {
	result := buildRobustPillar(nil)
	if result.Name != "robust" {
		t.Errorf("Name = %q, want %q", result.Name, "robust")
	}
	// Should have 3 constraints: frontend-errors + 2 static
	if len(result.Constraints) != 3 {
		t.Errorf("expected 3 constraints, got %d", len(result.Constraints))
	}
}

func TestBuildRobustPillar_ZeroErrors(t *testing.T) {
	result := buildRobustPillar(&metrics.FrontendReport{Errors: 0})
	c := findConstraint(result, "frontend-errors")
	if c == nil {
		t.Fatal("frontend-errors constraint missing")
	}
	if c.Status != "pass" {
		t.Errorf("status = %q, want pass", c.Status)
	}
	if c.Value != "0" {
		t.Errorf("value = %q, want 0", c.Value)
	}
}

func TestBuildRobustPillar_SomeErrors_Warn(t *testing.T) {
	result := buildRobustPillar(&metrics.FrontendReport{Errors: 5})
	c := findConstraint(result, "frontend-errors")
	if c == nil {
		t.Fatal("frontend-errors constraint missing")
	}
	if c.Status != "warn" {
		t.Errorf("status = %q, want warn (5 > 0, but <= 10)", c.Status)
	}
}

func TestBuildRobustPillar_ManyErrors_Fail(t *testing.T) {
	result := buildRobustPillar(&metrics.FrontendReport{Errors: 15})
	c := findConstraint(result, "frontend-errors")
	if c == nil {
		t.Fatal("frontend-errors constraint missing")
	}
	if c.Status != "fail" {
		t.Errorf("status = %q, want fail (15 > 10)", c.Status)
	}
}

func TestBuildRobustPillar_StaticConstraints(t *testing.T) {
	result := buildRobustPillar(nil)
	eb := findConstraint(result, "error-boundaries")
	gd := findConstraint(result, "graceful-degradation")
	if eb == nil || gd == nil {
		t.Fatal("missing static constraints")
	}
	if eb.Status != "static" || gd.Status != "static" {
		t.Error("static constraints should have status=static")
	}
}

// ---------------------------------------------------------------------------
// buildResilientPillar
// ---------------------------------------------------------------------------

func TestBuildResilientPillar_LiveConstraints(t *testing.T) {
	report := &metrics.ConnectionHealthReport{
		TotalConnects:       100,
		TotalDisconnects:    100,
		AbnormalDisconnects: 0,
		ReconnectSuccesses:  5,
		ReconnectFailures:   0,
	}

	result := buildResilientPillar(report)

	staticCount := 0
	for _, c := range result.Constraints {
		if c.Status == "static" {
			staticCount++
		}
	}
	if staticCount == len(result.Constraints) {
		t.Error("all constraints are static — expected live constraints")
	}

	for _, c := range result.Constraints {
		if c.Name == "chat-drop-rate" && c.Status != "pass" {
			t.Errorf("expected chat-drop-rate to pass, got %s", c.Status)
		}
	}
}

func TestBuildResilientPillar_NilReport(t *testing.T) {
	result := buildResilientPillar(nil)
	if result.Name != "resilient" {
		t.Errorf("Name = %q, want resilient", result.Name)
	}
	if len(result.Constraints) != 3 {
		t.Errorf("expected 3 constraints, got %d", len(result.Constraints))
	}
}

func TestBuildResilientPillar_HighDropRate_Fail(t *testing.T) {
	report := &metrics.ConnectionHealthReport{
		TotalConnects:       100,
		TotalDisconnects:    100,
		AbnormalDisconnects: 10, // 10% drop rate
	}
	result := buildResilientPillar(report)
	c := findConstraint(result, "chat-drop-rate")
	if c == nil {
		t.Fatal("chat-drop-rate constraint missing")
	}
	if c.Status != "fail" {
		t.Errorf("status = %q, want fail (drop rate > 0.5%%)", c.Status)
	}
}

func TestBuildResilientPillar_LowReconnectRate_Fail(t *testing.T) {
	report := &metrics.ConnectionHealthReport{
		TotalConnects:      100,
		TotalDisconnects:   100,
		ReconnectSuccesses: 50,
		ReconnectFailures:  50, // 50% reconnect rate
	}
	result := buildResilientPillar(report)
	c := findConstraint(result, "reconnect-success-rate")
	if c == nil {
		t.Fatal("reconnect-success-rate constraint missing")
	}
	if c.Status != "fail" {
		t.Errorf("status = %q, want fail (50%% < 95%%)", c.Status)
	}
}

func TestBuildResilientPillar_GoodReconnectRate_Pass(t *testing.T) {
	report := &metrics.ConnectionHealthReport{
		TotalConnects:      100,
		TotalDisconnects:   100,
		ReconnectSuccesses: 99,
		ReconnectFailures:  1,
	}
	result := buildResilientPillar(report)
	c := findConstraint(result, "reconnect-success-rate")
	if c == nil {
		t.Fatal("reconnect-success-rate constraint missing")
	}
	if c.Status != "pass" {
		t.Errorf("status = %q, want pass (99%% > 98%%)", c.Status)
	}
}

// ---------------------------------------------------------------------------
// buildSecurePillar
// ---------------------------------------------------------------------------

func TestBuildSecurePillar_AllStatic(t *testing.T) {
	result := buildSecurePillar()
	if result.Name != "secure" {
		t.Errorf("Name = %q, want secure", result.Name)
	}
	if len(result.Constraints) != 4 {
		t.Errorf("expected 4 constraints, got %d", len(result.Constraints))
	}
	for _, c := range result.Constraints {
		if c.Status != "static" {
			t.Errorf("constraint %q status = %q, want static", c.Name, c.Status)
		}
	}
	if result.Static != 4 {
		t.Errorf("Static = %d, want 4", result.Static)
	}
}

func TestBuildSecurePillar_HasExpectedConstraints(t *testing.T) {
	result := buildSecurePillar()
	expected := []string{"csp-headers", "sql-injection", "no-hardcoded-secrets", "oauth-token-security"}
	for _, name := range expected {
		if findConstraint(result, name) == nil {
			t.Errorf("missing constraint: %s", name)
		}
	}
}

// ---------------------------------------------------------------------------
// buildSovereignPillar
// ---------------------------------------------------------------------------

func TestBuildSovereignPillar_AllStatic(t *testing.T) {
	result := buildSovereignPillar()
	if result.Name != "sovereign" {
		t.Errorf("Name = %q, want sovereign", result.Name)
	}
	if len(result.Constraints) != 3 {
		t.Errorf("expected 3 constraints, got %d", len(result.Constraints))
	}
	for _, c := range result.Constraints {
		if c.Status != "static" {
			t.Errorf("constraint %q status = %q, want static", c.Name, c.Status)
		}
	}
}

func TestBuildSovereignPillar_HasExpectedConstraints(t *testing.T) {
	result := buildSovereignPillar()
	expected := []string{"local-data", "self-hosted", "no-cloud-deps"}
	for _, name := range expected {
		if findConstraint(result, name) == nil {
			t.Errorf("missing constraint: %s", name)
		}
	}
}

// ---------------------------------------------------------------------------
// buildTransparentPillar
// ---------------------------------------------------------------------------

func TestBuildTransparentPillar_IncludesReliabilitySignals(t *testing.T) {
	usage := &metrics.UsageReport{
		TotalEvents: 10,
		Actions:     map[string]int{"chat.send": 5},
	}
	ch := &metrics.ConnectionHealthReport{
		TotalConnects:       5,
		AuthFailures:        1,
		AuthSuccesses:       4,
		TotalDisconnects:    5,
		AbnormalDisconnects: 0,
	}

	result := buildTransparentPillar(usage, ch)

	constraintNames := make(map[string]bool)
	for _, c := range result.Constraints {
		constraintNames[c.Name] = true
	}

	required := []string{"event-tracking", "usage-tracking", "auth-event-coverage", "ws-lifecycle-coverage"}
	for _, name := range required {
		if !constraintNames[name] {
			t.Errorf("missing required constraint: %s", name)
		}
	}
}

func TestBuildTransparentPillar_NilInputs(t *testing.T) {
	result := buildTransparentPillar(nil, nil)
	if result.Name != "transparent" {
		t.Errorf("Name = %q, want transparent", result.Name)
	}
	// Should have 6 constraints (4 live + 2 static)
	if len(result.Constraints) != 6 {
		t.Errorf("expected 6 constraints, got %d", len(result.Constraints))
	}
}

func TestBuildTransparentPillar_ZeroEvents_Warn(t *testing.T) {
	usage := &metrics.UsageReport{TotalEvents: 0, Actions: map[string]int{}}
	result := buildTransparentPillar(usage, nil)
	c := findConstraint(result, "event-tracking")
	if c == nil {
		t.Fatal("event-tracking constraint missing")
	}
	if c.Status != "warn" {
		t.Errorf("status = %q, want warn (0 events)", c.Status)
	}
}

func TestBuildTransparentPillar_HasEvents_Pass(t *testing.T) {
	usage := &metrics.UsageReport{
		TotalEvents: 50,
		Actions:     map[string]int{"chat.send": 25, "tool.use": 25},
	}
	result := buildTransparentPillar(usage, nil)

	et := findConstraint(result, "event-tracking")
	if et == nil {
		t.Fatal("event-tracking constraint missing")
	}
	if et.Status != "pass" {
		t.Errorf("event-tracking status = %q, want pass", et.Status)
	}
	if et.Value != "50 events" {
		t.Errorf("event-tracking value = %q, want '50 events'", et.Value)
	}

	ut := findConstraint(result, "usage-tracking")
	if ut == nil {
		t.Fatal("usage-tracking constraint missing")
	}
	if ut.Status != "pass" {
		t.Errorf("usage-tracking status = %q, want pass", ut.Status)
	}
	if ut.Value != "2 action types" {
		t.Errorf("usage-tracking value = %q, want '2 action types'", ut.Value)
	}
}

func TestBuildTransparentPillar_NoAuthEvents_Warn(t *testing.T) {
	ch := &metrics.ConnectionHealthReport{
		AuthFailures:  0,
		AuthSuccesses: 0,
	}
	result := buildTransparentPillar(nil, ch)
	c := findConstraint(result, "auth-event-coverage")
	if c == nil {
		t.Fatal("auth-event-coverage constraint missing")
	}
	if c.Status != "warn" {
		t.Errorf("status = %q, want warn (0 auth events)", c.Status)
	}
}

func TestBuildTransparentPillar_HasAuthEvents_Pass(t *testing.T) {
	ch := &metrics.ConnectionHealthReport{
		AuthFailures:  2,
		AuthSuccesses: 10,
	}
	result := buildTransparentPillar(nil, ch)
	c := findConstraint(result, "auth-event-coverage")
	if c == nil {
		t.Fatal("auth-event-coverage constraint missing")
	}
	if c.Status != "pass" {
		t.Errorf("status = %q, want pass (12 auth events)", c.Status)
	}
}

func TestBuildTransparentPillar_WSLifecycle(t *testing.T) {
	ch := &metrics.ConnectionHealthReport{
		TotalConnects:    10,
		TotalDisconnects: 8,
	}
	result := buildTransparentPillar(nil, ch)
	c := findConstraint(result, "ws-lifecycle-coverage")
	if c == nil {
		t.Fatal("ws-lifecycle-coverage constraint missing")
	}
	if c.Status != "pass" {
		t.Errorf("status = %q, want pass (18 ws events)", c.Status)
	}
	if c.Value != "18 events" {
		t.Errorf("value = %q, want '18 events'", c.Value)
	}
}

func TestBuildTransparentPillar_StaticConstraints(t *testing.T) {
	result := buildTransparentPillar(nil, nil)
	mp := findConstraint(result, "metrics-pipeline")
	cr := findConstraint(result, "cli-reports")
	if mp == nil || cr == nil {
		t.Fatal("missing static constraints")
	}
	if mp.Status != "static" || cr.Status != "static" {
		t.Error("static constraints should have status=static")
	}
}

// ---------------------------------------------------------------------------
// Server + Handler HTTP tests
// ---------------------------------------------------------------------------

func newTestObserveServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	// Seed a metrics file so the aggregator has data.
	logger, err := metrics.NewEventLogger(dir, "")
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	logger.Log(metrics.EventAPIRequest, map[string]interface{}{
		"method": "GET", "path": "/api/health", "status": 200, "duration_ms": 5,
	})
	logger.Close()

	return New(WithDataDir(dir), WithHost("127.0.0.1"), WithPort(0))
}

func TestWithHost_Observe(t *testing.T) {
	srv := New(WithHost("0.0.0.0"))
	if srv.host != "0.0.0.0" {
		t.Errorf("host = %q, want 0.0.0.0", srv.host)
	}
}

func TestWithPort_Observe(t *testing.T) {
	srv := New(WithPort(9999))
	if srv.port != 9999 {
		t.Errorf("port = %d, want 9999", srv.port)
	}
}

func TestWithDataDir_Observe(t *testing.T) {
	srv := New(WithDataDir("/tmp/observe-test"))
	if srv.dataDir != "/tmp/observe-test" {
		t.Errorf("dataDir = %q, want /tmp/observe-test", srv.dataDir)
	}
}

func TestHandleHealth_HTTP(t *testing.T) {
	srv := newTestObserveServer(t)
	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("status = %q, want ok", body["status"])
	}
}

func TestHandleOverview_HTTP(t *testing.T) {
	srv := newTestObserveServer(t)
	req := httptest.NewRequest("GET", "/api/overview", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleLatency_HTTP(t *testing.T) {
	srv := newTestObserveServer(t)
	req := httptest.NewRequest("GET", "/api/latency", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestHandleAlerts_HTTP(t *testing.T) {
	srv := newTestObserveServer(t)
	req := httptest.NewRequest("GET", "/api/alerts", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestHandleDB_HTTP(t *testing.T) {
	srv := newTestObserveServer(t)
	req := httptest.NewRequest("GET", "/api/db", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestHandleRequests_HTTP(t *testing.T) {
	srv := newTestObserveServer(t)
	req := httptest.NewRequest("GET", "/api/requests", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestHandleFrontend_HTTP(t *testing.T) {
	srv := newTestObserveServer(t)
	req := httptest.NewRequest("GET", "/api/frontend", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestHandleUsage_HTTP(t *testing.T) {
	srv := newTestObserveServer(t)
	req := httptest.NewRequest("GET", "/api/usage", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestHandleQuality_HTTP(t *testing.T) {
	srv := newTestObserveServer(t)
	req := httptest.NewRequest("GET", "/api/quality", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestHandleLayers_HTTP(t *testing.T) {
	srv := newTestObserveServer(t)
	req := httptest.NewRequest("GET", "/api/layers", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestHandleSystem_HTTP(t *testing.T) {
	srv := newTestObserveServer(t)
	req := httptest.NewRequest("GET", "/api/system", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestHandleTail_HTTP(t *testing.T) {
	srv := newTestObserveServer(t)
	req := httptest.NewRequest("GET", "/api/tail", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandlePillars_HTTP(t *testing.T) {
	srv := newTestObserveServer(t)
	req := httptest.NewRequest("GET", "/api/pillars", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	// Verify pillars response structure — wrapped in {"pillars": [...]}.
	var resp struct {
		Pillars []pillarResult `json:"pillars"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)
	if len(resp.Pillars) != 6 {
		t.Errorf("expected 6 pillars, got %d", len(resp.Pillars))
	}
	names := make(map[string]bool)
	for _, p := range resp.Pillars {
		names[p.Name] = true
	}
	for _, expected := range []string{"performant", "robust", "resilient", "secure", "sovereign", "transparent"} {
		if !names[expected] {
			t.Errorf("missing pillar: %s", expected)
		}
	}
}

func TestHandlePillars_WithProductFilter(t *testing.T) {
	srv := newTestObserveServer(t)
	req := httptest.NewRequest("GET", "/api/pillars?product=chat", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestCorsMiddleware(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := corsMiddleware(inner)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	origin := rec.Header().Get("Access-Control-Allow-Origin")
	if origin == "" {
		t.Error("expected CORS header")
	}
}

func TestCorsMiddleware_OPTIONS(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := corsMiddleware(inner)

	req := httptest.NewRequest("OPTIONS", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204 for OPTIONS", rec.Code)
	}
}

func TestRecoveryMiddleware_Observe(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test")
	})
	handler := recoveryMiddleware(inner)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "error") {
		t.Error("expected JSON error in body")
	}
}

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, map[string]string{"key": "value"})

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusBadRequest, "bad request")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)
	if body["error"] != "bad request" {
		t.Errorf("error = %q, want 'bad request'", body["error"])
	}
}
