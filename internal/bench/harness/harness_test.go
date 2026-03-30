package harness

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rishav1305/soul/internal/bench/scoring"
)

func TestComputeSummary_Empty(t *testing.T) {
	s := computeSummary(nil)
	if s.AvgAccuracy != 0 {
		t.Errorf("AvgAccuracy = %f, want 0", s.AvgAccuracy)
	}
	if s.AvgLatencyS != 0 {
		t.Errorf("AvgLatencyS = %f, want 0", s.AvgLatencyS)
	}
}

func TestComputeSummary_Averages(t *testing.T) {
	results := []PromptResult{
		{Accuracy: 0.8, LatencyS: 1.0, PeakRAMMB: 100, TokensPerSecond: 10, PeakVRAMMB: 50},
		{Accuracy: 0.6, LatencyS: 2.0, PeakRAMMB: 200, TokensPerSecond: 20, PeakVRAMMB: 100},
	}
	s := computeSummary(results)
	if s.AvgAccuracy != 0.7 {
		t.Errorf("AvgAccuracy = %f, want 0.7", s.AvgAccuracy)
	}
	if s.AvgLatencyS != 1.5 {
		t.Errorf("AvgLatencyS = %f, want 1.5", s.AvgLatencyS)
	}
	if s.AvgPeakRAMMB != 150 {
		t.Errorf("AvgPeakRAMMB = %f, want 150", s.AvgPeakRAMMB)
	}
	if s.AvgTokensPerSecond != 15 {
		t.Errorf("AvgTokensPerSecond = %f, want 15", s.AvgTokensPerSecond)
	}
	if s.AvgPeakVRAMMB != 75 {
		t.Errorf("AvgPeakVRAMMB = %f, want 75", s.AvgPeakVRAMMB)
	}
}

func TestCategoryFromID(t *testing.T) {
	tests := []struct {
		id   string
		want string
	}{
		{"sh-001", "sh"},
		{"qa-012", "qa"},
		{"math-005", "math"},
		{"nodash", "nodash"},
		{"a-b-c", "a"},
	}
	for _, tc := range tests {
		got := categoryFromID(tc.id)
		if got != tc.want {
			t.Errorf("categoryFromID(%q) = %q, want %q", tc.id, got, tc.want)
		}
	}
}

func TestCallEndpoint_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req inferenceRequest
		json.NewDecoder(r.Body).Decode(&req)
		resp := inferenceResponse{
			Response:        "42",
			TokensPerSecond: 50.0,
			PeakRAMMB:       256.0,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	resp, err := callEndpoint(ts.URL, "What is 6*7?", 256)
	if err != nil {
		t.Fatalf("callEndpoint: %v", err)
	}
	if resp.Response != "42" {
		t.Errorf("Response = %q, want %q", resp.Response, "42")
	}
	if resp.TokensPerSecond != 50.0 {
		t.Errorf("TokensPerSecond = %f, want 50.0", resp.TokensPerSecond)
	}
}

func TestCallEndpoint_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer ts.Close()

	_, err := callEndpoint(ts.URL, "test", 256)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestCallEndpoint_Unreachable(t *testing.T) {
	_, err := callEndpoint("http://127.0.0.1:1", "test", 256)
	if err == nil {
		t.Fatal("expected error for unreachable endpoint")
	}
}

func TestRunBenchmark_WithMockEndpoint(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := inferenceResponse{
			Response:        "test response",
			TokensPerSecond: 30.0,
			PeakRAMMB:       128.0,
			PeakVRAMMB:      64.0,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	config := BenchConfig{
		ModelEndpoint: ts.URL,
		Categories:    []string{"smoke-test"},
		MaxTokens:     128,
	}

	result, err := RunBenchmark(config)
	if err != nil {
		t.Fatalf("RunBenchmark: %v", err)
	}
	if result.Model != ts.URL {
		t.Errorf("Model = %q, want endpoint URL", result.Model)
	}
	if len(result.Results) == 0 {
		t.Error("expected at least one prompt result")
	}
	if result.Summary.AvgLatencyS <= 0 {
		t.Errorf("AvgLatencyS = %f, want > 0", result.Summary.AvgLatencyS)
	}
}

func TestRunSmoke(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := inferenceResponse{Response: "answer", PeakRAMMB: 64.0}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	result, err := RunSmoke(BenchConfig{ModelEndpoint: ts.URL})
	if err != nil {
		t.Fatalf("RunSmoke: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestEvaluatePrompt_UnreachableEndpoint(t *testing.T) {
	config := BenchConfig{
		ModelEndpoint: "http://127.0.0.1:1",
		MaxTokens:     128,
	}
	p := scoring.PromptData{
		ID:             "test-001",
		Task:           "test",
		Prompt:         "hello",
		ExpectedAnswer: "world",
		Scoring:        "contains_keywords",
	}
	pr := evaluatePrompt(config, p)
	if pr.Response != "[endpoint unreachable]" {
		t.Errorf("Response = %q, want [endpoint unreachable]", pr.Response)
	}
	if pr.Accuracy != 0 {
		t.Errorf("Accuracy = %f, want 0", pr.Accuracy)
	}
}

func TestRunBenchmark_DefaultMaxTokens(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req inferenceRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.MaxTokens != 256 {
			t.Errorf("MaxTokens = %d, want 256 (default)", req.MaxTokens)
		}
		resp := inferenceResponse{Response: "ok"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	_, err := RunBenchmark(BenchConfig{
		ModelEndpoint: ts.URL,
		Categories:    []string{"smoke-test"},
		// MaxTokens deliberately omitted — should default to 256
	})
	if err != nil {
		t.Fatalf("RunBenchmark: %v", err)
	}
}
