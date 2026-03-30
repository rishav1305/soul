package results

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/rishav1305/soul/internal/bench/harness"
)

func approxEq(a, b, epsilon float64) bool {
	return math.Abs(a-b) < epsilon
}

func testResult(model, ts string) *harness.BenchResult {
	return &harness.BenchResult{
		Model:     model,
		Timestamp: ts,
		Results:   []harness.PromptResult{},
		Summary: harness.Summary{
			AvgAccuracy:        0.85,
			AvgLatencyS:        1.2,
			AvgPeakRAMMB:       256,
			AvgTokensPerSecond: 40,
		},
		CARSRam:          3.4,
		CategoryAccuracy: map[string]float64{"qa": 0.9, "math": 0.8},
	}
}

func TestSanitize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"http://localhost:3000", "http-localhost-3000"},
		{"model.v2", "model-v2"},
		{"simple", "simple"},
		{"a/b/c", "a-b-c"},
	}
	for _, tc := range tests {
		got := sanitize(tc.input)
		if got != tc.want {
			t.Errorf("sanitize(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestSanitize_Truncation(t *testing.T) {
	long := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 57 chars
	got := sanitize(long)
	if len(got) > 50 {
		t.Errorf("sanitize should truncate to 50 chars, got %d", len(got))
	}
}

func TestResultsDir(t *testing.T) {
	got := resultsDir("/data")
	want := filepath.Join("/data", "bench", "results")
	if got != want {
		t.Errorf("resultsDir = %q, want %q", got, want)
	}
}

func TestSaveAndGetResult(t *testing.T) {
	dir := t.TempDir()
	r := testResult("llama-3b", "2026-03-30T10:00:00Z")

	if err := SaveResult(dir, r); err != nil {
		t.Fatalf("SaveResult: %v", err)
	}

	// List should find it.
	summaries, err := ListResults(dir)
	if err != nil {
		t.Fatalf("ListResults: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 result, got %d", len(summaries))
	}
	if summaries[0].Model != "llama-3b" {
		t.Errorf("Model = %q, want %q", summaries[0].Model, "llama-3b")
	}
	if summaries[0].AvgAccuracy != 0.85 {
		t.Errorf("AvgAccuracy = %f, want 0.85", summaries[0].AvgAccuracy)
	}

	// Get by ID.
	loaded, err := GetResult(dir, summaries[0].ID)
	if err != nil {
		t.Fatalf("GetResult: %v", err)
	}
	if loaded.Model != "llama-3b" {
		t.Errorf("loaded Model = %q, want %q", loaded.Model, "llama-3b")
	}
	if loaded.CARSRam != 3.4 {
		t.Errorf("CARSRam = %f, want 3.4", loaded.CARSRam)
	}
}

func TestListResults_Empty(t *testing.T) {
	dir := t.TempDir()
	summaries, err := ListResults(dir)
	if err != nil {
		t.Fatalf("ListResults: %v", err)
	}
	if summaries != nil {
		t.Errorf("expected nil for empty dir, got %v", summaries)
	}
}

func TestListResults_SkipsNonJSON(t *testing.T) {
	dir := t.TempDir()
	rDir := filepath.Join(dir, "bench", "results")
	os.MkdirAll(rDir, 0755)
	os.WriteFile(filepath.Join(rDir, "notes.txt"), []byte("hello"), 0644)
	os.MkdirAll(filepath.Join(rDir, "subdir"), 0755)

	summaries, err := ListResults(dir)
	if err != nil {
		t.Fatalf("ListResults: %v", err)
	}
	if len(summaries) != 0 {
		t.Errorf("expected 0 results (only non-JSON files), got %d", len(summaries))
	}
}

func TestGetResult_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := GetResult(dir, "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing result")
	}
}

func TestCompareResults(t *testing.T) {
	dir := t.TempDir()

	r1 := testResult("model-a", "2026-03-29T10:00:00Z")
	r1.Summary.AvgAccuracy = 0.70
	r1.Summary.AvgLatencyS = 2.0
	r1.CARSRam = 2.0
	r1.CategoryAccuracy = map[string]float64{"qa": 0.8, "math": 0.6}

	r2 := testResult("model-b", "2026-03-30T10:00:00Z")
	r2.Summary.AvgAccuracy = 0.90
	r2.Summary.AvgLatencyS = 1.0
	r2.CARSRam = 4.0
	r2.CategoryAccuracy = map[string]float64{"qa": 0.95, "code": 0.85}

	SaveResult(dir, r1)
	SaveResult(dir, r2)

	summaries, _ := ListResults(dir)
	if len(summaries) != 2 {
		t.Fatalf("expected 2 results, got %d", len(summaries))
	}

	comp, err := CompareResults(dir, summaries[1].ID, summaries[0].ID)
	if err != nil {
		t.Fatalf("CompareResults: %v", err)
	}

	if !approxEq(comp.AccuracyDelta, 0.20, 0.001) {
		t.Errorf("AccuracyDelta = %f, want ~0.20", comp.AccuracyDelta)
	}
	if !approxEq(comp.LatencyDelta, -1.0, 0.001) {
		t.Errorf("LatencyDelta = %f, want ~-1.0", comp.LatencyDelta)
	}
	if !approxEq(comp.CARSRamDelta, 2.0, 0.001) {
		t.Errorf("CARSRamDelta = %f, want ~2.0", comp.CARSRamDelta)
	}

	// qa exists in both: 0.95 - 0.8 = 0.15
	if !approxEq(comp.CategoryDeltas["qa"], 0.15, 0.001) {
		t.Errorf("CategoryDeltas[qa] = %f, want ~0.15", comp.CategoryDeltas["qa"])
	}
	// code only in r2: 0.85 - 0 = 0.85
	if !approxEq(comp.CategoryDeltas["code"], 0.85, 0.001) {
		t.Errorf("CategoryDeltas[code] = %f, want ~0.85", comp.CategoryDeltas["code"])
	}
}

func TestCompareResults_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := CompareResults(dir, "missing1", "missing2")
	if err == nil {
		t.Fatal("expected error for missing results")
	}
}

func TestListResults_SortedByTimestamp(t *testing.T) {
	dir := t.TempDir()

	SaveResult(dir, testResult("older", "2026-03-28T10:00:00Z"))
	SaveResult(dir, testResult("newer", "2026-03-30T10:00:00Z"))

	summaries, err := ListResults(dir)
	if err != nil {
		t.Fatalf("ListResults: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("expected 2, got %d", len(summaries))
	}
	// Sorted newest first.
	if summaries[0].Model != "newer" {
		t.Errorf("first result Model = %q, want %q", summaries[0].Model, "newer")
	}
}
