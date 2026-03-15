package results

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rishav1305/soul-v2/internal/bench/harness"
)

// ResultSummary is a lightweight view of a stored result.
type ResultSummary struct {
	ID          string  `json:"id"`
	Model       string  `json:"model"`
	Timestamp   string  `json:"timestamp"`
	AvgAccuracy float64 `json:"avg_accuracy"`
	AvgLatencyS float64 `json:"avg_latency_s"`
	CARSRam     float64 `json:"cars_ram"`
}

// Comparison holds a side-by-side comparison of two benchmark results.
type Comparison struct {
	Result1        ResultSummary      `json:"result1"`
	Result2        ResultSummary      `json:"result2"`
	AccuracyDelta  float64            `json:"accuracy_delta"`
	LatencyDelta   float64            `json:"latency_delta"`
	CARSRamDelta   float64            `json:"cars_ram_delta"`
	CategoryDeltas map[string]float64 `json:"category_deltas"`
}

// resultsDir returns the path to the bench results directory.
func resultsDir(dataDir string) string {
	return filepath.Join(dataDir, "bench", "results")
}

// SaveResult writes a benchmark result as JSON to the data directory.
func SaveResult(dataDir string, result *harness.BenchResult) error {
	dir := resultsDir(dataDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create results dir: %w", err)
	}

	// Generate ID from timestamp and model.
	ts := strings.ReplaceAll(result.Timestamp[:10], ":", "-")
	model := sanitize(result.Model)
	id := ts + "-" + model
	filename := id + ".json"

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}

	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write result: %w", err)
	}
	return nil
}

// ListResults returns summaries of all stored benchmark results.
func ListResults(dataDir string) ([]ResultSummary, error) {
	dir := resultsDir(dataDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read results dir: %w", err)
	}

	var summaries []ResultSummary
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		result, err := loadResultFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		summaries = append(summaries, ResultSummary{
			ID:          id,
			Model:       result.Model,
			Timestamp:   result.Timestamp,
			AvgAccuracy: result.Summary.AvgAccuracy,
			AvgLatencyS: result.Summary.AvgLatencyS,
			CARSRam:     result.CARSRam,
		})
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Timestamp > summaries[j].Timestamp
	})
	return summaries, nil
}

// GetResult loads a full benchmark result by ID.
func GetResult(dataDir, id string) (*harness.BenchResult, error) {
	path := filepath.Join(resultsDir(dataDir), id+".json")
	return loadResultFile(path)
}

// CompareResults computes a delta comparison between two stored results.
func CompareResults(dataDir, id1, id2 string) (*Comparison, error) {
	r1, err := GetResult(dataDir, id1)
	if err != nil {
		return nil, fmt.Errorf("load result %s: %w", id1, err)
	}
	r2, err := GetResult(dataDir, id2)
	if err != nil {
		return nil, fmt.Errorf("load result %s: %w", id2, err)
	}

	comp := &Comparison{
		Result1: ResultSummary{
			ID:          id1,
			Model:       r1.Model,
			Timestamp:   r1.Timestamp,
			AvgAccuracy: r1.Summary.AvgAccuracy,
			AvgLatencyS: r1.Summary.AvgLatencyS,
			CARSRam:     r1.CARSRam,
		},
		Result2: ResultSummary{
			ID:          id2,
			Model:       r2.Model,
			Timestamp:   r2.Timestamp,
			AvgAccuracy: r2.Summary.AvgAccuracy,
			AvgLatencyS: r2.Summary.AvgLatencyS,
			CARSRam:     r2.CARSRam,
		},
		AccuracyDelta:  r2.Summary.AvgAccuracy - r1.Summary.AvgAccuracy,
		LatencyDelta:   r2.Summary.AvgLatencyS - r1.Summary.AvgLatencyS,
		CARSRamDelta:   r2.CARSRam - r1.CARSRam,
		CategoryDeltas: make(map[string]float64),
	}

	// Compute per-category deltas.
	allCats := make(map[string]bool)
	for k := range r1.CategoryAccuracy {
		allCats[k] = true
	}
	for k := range r2.CategoryAccuracy {
		allCats[k] = true
	}
	for cat := range allCats {
		comp.CategoryDeltas[cat] = r2.CategoryAccuracy[cat] - r1.CategoryAccuracy[cat]
	}

	return comp, nil
}

// loadResultFile reads and parses a result JSON file.
func loadResultFile(path string) (*harness.BenchResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var result harness.BenchResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// sanitize creates a filesystem-safe string from a URL or model name.
func sanitize(s string) string {
	s = strings.ReplaceAll(s, "://", "-")
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, ":", "-")
	s = strings.ReplaceAll(s, ".", "-")
	if len(s) > 50 {
		s = s[:50]
	}
	return s
}
