package harness

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rishav1305/soul/internal/bench/prompts"
	"github.com/rishav1305/soul/internal/bench/scoring"
)

// BenchConfig configures a benchmark run.
type BenchConfig struct {
	ModelEndpoint string   // URL to send prompts to
	Categories    []string // empty = all
	MaxTokens     int      // default 256
	GPU           bool
}

// BenchResult holds the full result of a benchmark run.
type BenchResult struct {
	Model            string             `json:"model"`
	Timestamp        string             `json:"timestamp"`
	Results          []PromptResult     `json:"results"`
	Summary          Summary            `json:"summary"`
	CARSRam          float64            `json:"cars_ram"`
	CARSSize         float64            `json:"cars_size"`
	CARSVram         float64            `json:"cars_vram"`
	CategoryAccuracy map[string]float64 `json:"category_accuracy"`
}

// PromptResult holds the result of a single prompt evaluation.
type PromptResult struct {
	ID              string  `json:"id"`
	Task            string  `json:"task"`
	Prompt          string  `json:"prompt"`
	Expected        string  `json:"expected"`
	Response        string  `json:"response"`
	Accuracy        float64 `json:"accuracy"`
	LatencyS        float64 `json:"latency_s"`
	PeakRAMMB       float64 `json:"peak_ram_mb"`
	TokensPerSecond float64 `json:"tokens_per_second"`
	PeakVRAMMB      float64 `json:"peak_vram_mb"`
}

// Summary holds aggregate statistics.
type Summary struct {
	AvgAccuracy        float64 `json:"avg_accuracy"`
	AvgLatencyS        float64 `json:"avg_latency_s"`
	AvgPeakRAMMB       float64 `json:"avg_peak_ram_mb"`
	AvgTokensPerSecond float64 `json:"avg_tokens_per_second"`
	AvgPeakVRAMMB      float64 `json:"avg_peak_vram_mb"`
}

// inferenceRequest is the payload sent to the model endpoint.
type inferenceRequest struct {
	Prompt    string `json:"prompt"`
	MaxTokens int    `json:"max_tokens"`
}

// inferenceResponse is the expected response from the model endpoint.
type inferenceResponse struct {
	Response        string  `json:"response"`
	TokensPerSecond float64 `json:"tokens_per_second,omitempty"`
	PeakRAMMB       float64 `json:"peak_ram_mb,omitempty"`
	PeakVRAMMB      float64 `json:"peak_vram_mb,omitempty"`
}

// RunBenchmark loads prompts, sends them to the model endpoint, scores responses, and computes CARS metrics.
func RunBenchmark(config BenchConfig) (*BenchResult, error) {
	if config.MaxTokens == 0 {
		config.MaxTokens = 256
	}

	// Load prompts.
	var allPrompts []scoring.PromptData
	if len(config.Categories) == 0 {
		loaded, err := prompts.LoadAll()
		if err != nil {
			return nil, fmt.Errorf("load prompts: %w", err)
		}
		allPrompts = loaded
	} else {
		for _, cat := range config.Categories {
			loaded, err := prompts.LoadCategory(cat)
			if err != nil {
				return nil, fmt.Errorf("load category %s: %w", cat, err)
			}
			allPrompts = append(allPrompts, loaded...)
		}
	}

	result := &BenchResult{
		Model:            config.ModelEndpoint,
		Timestamp:        time.Now().UTC().Format(time.RFC3339),
		Results:          make([]PromptResult, 0, len(allPrompts)),
		CategoryAccuracy: make(map[string]float64),
	}

	// Track per-category scores.
	catScores := make(map[string][]float64)

	for _, p := range allPrompts {
		pr := evaluatePrompt(config, p)
		result.Results = append(result.Results, pr)

		// Derive category from prompt ID prefix.
		cat := categoryFromID(p.ID)
		catScores[cat] = append(catScores[cat], pr.Accuracy)
	}

	// Compute summary.
	result.Summary = computeSummary(result.Results)

	// Compute category accuracy.
	for cat, scores := range catScores {
		sum := 0.0
		for _, s := range scores {
			sum += s
		}
		result.CategoryAccuracy[cat] = sum / float64(len(scores))
	}

	// Compute CARS metrics.
	// CARS_RAM = accuracy / (ram_gb * latency_s)
	// CARS_Size = accuracy / (model_size_gb * latency_s)  — use 1.0 as placeholder
	// CARS_VRAM = accuracy / (vram_gb * latency_s)
	acc := result.Summary.AvgAccuracy
	lat := result.Summary.AvgLatencyS
	ramGB := result.Summary.AvgPeakRAMMB / 1024.0
	vramGB := result.Summary.AvgPeakVRAMMB / 1024.0

	if ramGB > 0 && lat > 0 {
		result.CARSRam = acc / (ramGB * lat)
	}
	if lat > 0 {
		result.CARSSize = acc / (1.0 * lat) // model_size_gb placeholder = 1.0
	}
	if vramGB > 0 && lat > 0 {
		result.CARSVram = acc / (vramGB * lat)
	}

	return result, nil
}

// RunSmoke runs only the smoke-test prompts.
func RunSmoke(config BenchConfig) (*BenchResult, error) {
	config.Categories = []string{"smoke-test"}
	return RunBenchmark(config)
}

// evaluatePrompt sends a single prompt to the endpoint and scores the response.
func evaluatePrompt(config BenchConfig, p scoring.PromptData) PromptResult {
	pr := PromptResult{
		ID:       p.ID,
		Task:     p.Task,
		Prompt:   p.Prompt,
		Expected: p.ExpectedAnswer,
	}

	start := time.Now()
	resp, err := callEndpoint(config.ModelEndpoint, p.Prompt, config.MaxTokens)
	pr.LatencyS = time.Since(start).Seconds()

	if err != nil {
		// Endpoint unreachable — return mock result.
		pr.Response = "[endpoint unreachable]"
		pr.Accuracy = 0.0
		pr.PeakRAMMB = 0.0
		pr.TokensPerSecond = 0.0
		pr.PeakVRAMMB = 0.0
		return pr
	}

	pr.Response = resp.Response
	pr.PeakRAMMB = resp.PeakRAMMB
	pr.TokensPerSecond = resp.TokensPerSecond
	pr.PeakVRAMMB = resp.PeakVRAMMB
	pr.Accuracy = scoring.ScoreResult(resp.Response, p)

	return pr
}

// callEndpoint sends a prompt to the model inference endpoint.
func callEndpoint(endpoint, prompt string, maxTokens int) (*inferenceResponse, error) {
	reqBody, err := json.Marshal(inferenceRequest{
		Prompt:    prompt,
		MaxTokens: maxTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Post(endpoint, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("post to endpoint: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var ir inferenceResponse
	if err := json.Unmarshal(body, &ir); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &ir, nil
}

// computeSummary calculates average metrics across all prompt results.
func computeSummary(results []PromptResult) Summary {
	if len(results) == 0 {
		return Summary{}
	}
	var s Summary
	for _, r := range results {
		s.AvgAccuracy += r.Accuracy
		s.AvgLatencyS += r.LatencyS
		s.AvgPeakRAMMB += r.PeakRAMMB
		s.AvgTokensPerSecond += r.TokensPerSecond
		s.AvgPeakVRAMMB += r.PeakVRAMMB
	}
	n := float64(len(results))
	s.AvgAccuracy /= n
	s.AvgLatencyS /= n
	s.AvgPeakRAMMB /= n
	s.AvgTokensPerSecond /= n
	s.AvgPeakVRAMMB /= n
	return s
}

// categoryFromID derives a category key from a prompt ID (e.g., "sh-001" -> "sh").
func categoryFromID(id string) string {
	for i, c := range id {
		if c == '-' {
			return id[:i]
		}
	}
	return id
}
