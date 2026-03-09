package metrics

import (
	"fmt"
	"testing"
	"time"
)

func TestNewAggregator(t *testing.T) {
	agg := NewAggregator("/tmp/test")
	if agg == nil {
		t.Fatal("NewAggregator returned nil")
	}
	if agg.dataDir != "/tmp/test" {
		t.Errorf("dataDir = %q, want %q", agg.dataDir, "/tmp/test")
	}
}

// --- Status ---

func TestStatus_EmptyEvents(t *testing.T) {
	dir := t.TempDir()
	agg := NewAggregator(dir)

	report, err := agg.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if report.TotalSessions != 0 {
		t.Errorf("TotalSessions = %d, want 0", report.TotalSessions)
	}
	if report.TotalMessages != 0 {
		t.Errorf("TotalMessages = %d, want 0", report.TotalMessages)
	}
	if report.TotalErrors != 0 {
		t.Errorf("TotalErrors = %d, want 0", report.TotalErrors)
	}
	if report.ActiveStreams != 0 {
		t.Errorf("ActiveStreams = %d, want 0", report.ActiveStreams)
	}
	if report.Uptime != "unknown" {
		t.Errorf("Uptime = %q, want %q", report.Uptime, "unknown")
	}
}

func TestStatus_CountsEventsCorrectly(t *testing.T) {
	dir := t.TempDir()
	writeTestEvents(t, dir, []string{
		`{"ts":"2026-03-09T10:00:00Z","event":"system.start","data":{"port":3002}}`,
		`{"ts":"2026-03-09T10:00:01Z","event":"ws.connect","data":{"client":"c1"}}`,
		`{"ts":"2026-03-09T10:00:02Z","event":"ws.connect","data":{"client":"c2"}}`,
		`{"ts":"2026-03-09T10:00:03Z","event":"api.request","data":{"path":"/chat","input_tokens":100}}`,
		`{"ts":"2026-03-09T10:00:04Z","event":"api.request","data":{"path":"/chat","input_tokens":200}}`,
		`{"ts":"2026-03-09T10:00:05Z","event":"api.error","data":{"error":"timeout"}}`,
		`{"ts":"2026-03-09T10:00:06Z","event":"override.error","data":{"type":"syntax","reason":"bad"}}`,
		`{"ts":"2026-03-09T10:00:07Z","event":"ws.stream.start","data":{"stream_id":"s1"}}`,
		`{"ts":"2026-03-09T10:00:08Z","event":"ws.stream.end","data":{"stream_id":"s1"}}`,
		`{"ts":"2026-03-09T10:00:09Z","event":"ws.stream.start","data":{"stream_id":"s2"}}`,
	})

	agg := NewAggregator(dir)
	report, err := agg.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	if report.TotalSessions != 2 {
		t.Errorf("TotalSessions = %d, want 2", report.TotalSessions)
	}
	if report.TotalMessages != 2 {
		t.Errorf("TotalMessages = %d, want 2", report.TotalMessages)
	}
	if report.TotalErrors != 2 {
		t.Errorf("TotalErrors = %d, want 2", report.TotalErrors)
	}
	if report.ActiveStreams != 1 {
		t.Errorf("ActiveStreams = %d, want 1", report.ActiveStreams)
	}
	if report.LastEvent.IsZero() {
		t.Error("LastEvent should not be zero")
	}
}

func TestStatus_MissingFile(t *testing.T) {
	dir := t.TempDir()
	agg := NewAggregator(dir)

	report, err := agg.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if report.TotalSessions != 0 {
		t.Errorf("TotalSessions = %d, want 0", report.TotalSessions)
	}
}

// --- Quality ---

func TestQuality_EmptyEvents(t *testing.T) {
	dir := t.TempDir()
	agg := NewAggregator(dir)

	report, err := agg.Quality()
	if err != nil {
		t.Fatalf("Quality: %v", err)
	}
	if report.TotalErrors != 0 {
		t.Errorf("TotalErrors = %d, want 0", report.TotalErrors)
	}
	if report.FalsePositives != 0 {
		t.Errorf("FalsePositives = %d, want 0", report.FalsePositives)
	}
	if len(report.ErrorCounts) != 0 {
		t.Errorf("ErrorCounts should be empty, got %v", report.ErrorCounts)
	}
	if len(report.QualityRatings) != 0 {
		t.Errorf("QualityRatings should be empty, got %v", report.QualityRatings)
	}
}

func TestQuality_CountsErrorsByType(t *testing.T) {
	dir := t.TempDir()
	writeTestEvents(t, dir, []string{
		`{"ts":"2026-03-09T10:00:00Z","event":"override.error","data":{"type":"syntax","reason":"missing semicolon"}}`,
		`{"ts":"2026-03-09T10:00:01Z","event":"override.error","data":{"type":"syntax","reason":"bad import"}}`,
		`{"ts":"2026-03-09T10:00:02Z","event":"override.error","data":{"type":"logic","reason":"wrong condition"}}`,
		`{"ts":"2026-03-09T10:00:03Z","event":"override.error","data":{"type":"false_positive","reason":"not a bug"}}`,
		`{"ts":"2026-03-09T10:00:04Z","event":"override.quality","data":{"step":"step-1","rating":4,"notes":"good"}}`,
		`{"ts":"2026-03-09T10:00:05Z","event":"override.quality","data":{"step":"step-2","rating":2,"notes":"poor"}}`,
	})

	agg := NewAggregator(dir)
	report, err := agg.Quality()
	if err != nil {
		t.Fatalf("Quality: %v", err)
	}

	if report.TotalErrors != 3 {
		t.Errorf("TotalErrors = %d, want 3", report.TotalErrors)
	}
	if report.FalsePositives != 1 {
		t.Errorf("FalsePositives = %d, want 1", report.FalsePositives)
	}
	if report.ErrorCounts["syntax"] != 2 {
		t.Errorf("ErrorCounts[syntax] = %d, want 2", report.ErrorCounts["syntax"])
	}
	if report.ErrorCounts["logic"] != 1 {
		t.Errorf("ErrorCounts[logic] = %d, want 1", report.ErrorCounts["logic"])
	}
	if len(report.QualityRatings) != 2 {
		t.Fatalf("QualityRatings len = %d, want 2", len(report.QualityRatings))
	}
	if report.QualityRatings[0].Rating != 4 {
		t.Errorf("QualityRatings[0].Rating = %d, want 4", report.QualityRatings[0].Rating)
	}
	if report.QualityRatings[1].Step != "step-2" {
		t.Errorf("QualityRatings[1].Step = %q, want %q", report.QualityRatings[1].Step, "step-2")
	}
}

func TestQuality_ErrorWithNoType(t *testing.T) {
	dir := t.TempDir()
	writeTestEvents(t, dir, []string{
		`{"ts":"2026-03-09T10:00:00Z","event":"override.error","data":{"reason":"missing type field"}}`,
	})

	agg := NewAggregator(dir)
	report, err := agg.Quality()
	if err != nil {
		t.Fatalf("Quality: %v", err)
	}
	if report.TotalErrors != 1 {
		t.Errorf("TotalErrors = %d, want 1", report.TotalErrors)
	}
	if report.ErrorCounts["unknown"] != 1 {
		t.Errorf("ErrorCounts[unknown] = %d, want 1", report.ErrorCounts["unknown"])
	}
}

// --- Layers ---

func TestLayers_EmptyEvents(t *testing.T) {
	dir := t.TempDir()
	agg := NewAggregator(dir)

	report, err := agg.Layers()
	if err != nil {
		t.Fatalf("Layers: %v", err)
	}
	if len(report.GateResults) != 0 {
		t.Errorf("GateResults should be empty, got %v", report.GateResults)
	}
}

func TestLayers_CountsGateResults(t *testing.T) {
	dir := t.TempDir()
	writeTestEvents(t, dir, []string{
		`{"ts":"2026-03-09T10:00:00Z","event":"gate.pass","data":{"gate":"build"}}`,
		`{"ts":"2026-03-09T10:00:01Z","event":"gate.pass","data":{"gate":"build"}}`,
		`{"ts":"2026-03-09T10:00:02Z","event":"gate.fail","data":{"gate":"build"}}`,
		`{"ts":"2026-03-09T10:00:03Z","event":"gate.pass","data":{"gate":"test"}}`,
		`{"ts":"2026-03-09T10:00:04Z","event":"gate.retry","data":{"gate":"test"}}`,
		`{"ts":"2026-03-09T10:00:05Z","event":"gate.fail","data":{"gate":"visual"}}`,
	})

	agg := NewAggregator(dir)
	report, err := agg.Layers()
	if err != nil {
		t.Fatalf("Layers: %v", err)
	}

	if len(report.GateResults) != 3 {
		t.Fatalf("GateResults len = %d, want 3", len(report.GateResults))
	}

	// Results are sorted by gate name.
	gateByName := make(map[string]GateResult)
	for _, r := range report.GateResults {
		gateByName[r.Gate] = r
	}

	build := gateByName["build"]
	if build.Pass != 2 || build.Fail != 1 || build.Total != 3 {
		t.Errorf("build gate = {pass:%d fail:%d total:%d}, want {pass:2 fail:1 total:3}",
			build.Pass, build.Fail, build.Total)
	}

	test := gateByName["test"]
	if test.Pass != 1 || test.Retry != 1 || test.Total != 2 {
		t.Errorf("test gate = {pass:%d retry:%d total:%d}, want {pass:1 retry:1 total:2}",
			test.Pass, test.Retry, test.Total)
	}

	visual := gateByName["visual"]
	if visual.Fail != 1 || visual.Total != 1 {
		t.Errorf("visual gate = {fail:%d total:%d}, want {fail:1 total:1}",
			visual.Fail, visual.Total)
	}
}

func TestLayers_UsesNameFieldFallback(t *testing.T) {
	dir := t.TempDir()
	writeTestEvents(t, dir, []string{
		`{"ts":"2026-03-09T10:00:00Z","event":"gate.pass","data":{"name":"lint"}}`,
	})

	agg := NewAggregator(dir)
	report, err := agg.Layers()
	if err != nil {
		t.Fatalf("Layers: %v", err)
	}

	if len(report.GateResults) != 1 {
		t.Fatalf("GateResults len = %d, want 1", len(report.GateResults))
	}
	if report.GateResults[0].Gate != "lint" {
		t.Errorf("Gate = %q, want %q", report.GateResults[0].Gate, "lint")
	}
}

func TestLayers_MissingGateNameUsesUnknown(t *testing.T) {
	dir := t.TempDir()
	writeTestEvents(t, dir, []string{
		`{"ts":"2026-03-09T10:00:00Z","event":"gate.pass","data":{}}`,
	})

	agg := NewAggregator(dir)
	report, err := agg.Layers()
	if err != nil {
		t.Fatalf("Layers: %v", err)
	}

	if len(report.GateResults) != 1 {
		t.Fatalf("GateResults len = %d, want 1", len(report.GateResults))
	}
	if report.GateResults[0].Gate != "unknown" {
		t.Errorf("Gate = %q, want %q", report.GateResults[0].Gate, "unknown")
	}
}

// --- Cost ---

func TestCost_EmptyEvents(t *testing.T) {
	dir := t.TempDir()
	agg := NewAggregator(dir)

	report, err := agg.Cost()
	if err != nil {
		t.Fatalf("Cost: %v", err)
	}
	if report.InputTokens != 0 {
		t.Errorf("InputTokens = %d, want 0", report.InputTokens)
	}
	if report.OutputTokens != 0 {
		t.Errorf("OutputTokens = %d, want 0", report.OutputTokens)
	}
	if report.TotalRequests != 0 {
		t.Errorf("TotalRequests = %d, want 0", report.TotalRequests)
	}
	if report.EstimatedCost != 0 {
		t.Errorf("EstimatedCost = %f, want 0", report.EstimatedCost)
	}
}

func TestCost_SumsTokensCorrectly(t *testing.T) {
	dir := t.TempDir()
	writeTestEvents(t, dir, []string{
		`{"ts":"2026-03-09T10:00:00Z","event":"api.request","data":{"input_tokens":1000,"output_tokens":500}}`,
		`{"ts":"2026-03-09T10:00:01Z","event":"api.request","data":{"input_tokens":2000,"output_tokens":1000}}`,
		`{"ts":"2026-03-09T10:00:02Z","event":"api.error","data":{"error":"rate_limit"}}`,
		`{"ts":"2026-03-09T10:00:03Z","event":"api.request","data":{"input_tokens":500,"output_tokens":250}}`,
	})

	agg := NewAggregator(dir)
	report, err := agg.Cost()
	if err != nil {
		t.Fatalf("Cost: %v", err)
	}

	if report.TotalRequests != 3 {
		t.Errorf("TotalRequests = %d, want 3", report.TotalRequests)
	}
	if report.InputTokens != 3500 {
		t.Errorf("InputTokens = %d, want 3500", report.InputTokens)
	}
	if report.OutputTokens != 1750 {
		t.Errorf("OutputTokens = %d, want 1750", report.OutputTokens)
	}

	// Expected cost: (3500/1M * $3) + (1750/1M * $15) = $0.0105 + $0.02625 = $0.03675
	// Rounded to 4 decimal places: $0.0368
	expectedCost := 0.0368
	if report.EstimatedCost != expectedCost {
		t.Errorf("EstimatedCost = %f, want %f", report.EstimatedCost, expectedCost)
	}
}

func TestCost_MissingTokenFields(t *testing.T) {
	dir := t.TempDir()
	writeTestEvents(t, dir, []string{
		`{"ts":"2026-03-09T10:00:00Z","event":"api.request","data":{"path":"/chat"}}`,
	})

	agg := NewAggregator(dir)
	report, err := agg.Cost()
	if err != nil {
		t.Fatalf("Cost: %v", err)
	}

	if report.TotalRequests != 1 {
		t.Errorf("TotalRequests = %d, want 1", report.TotalRequests)
	}
	if report.InputTokens != 0 {
		t.Errorf("InputTokens = %d, want 0", report.InputTokens)
	}
	if report.OutputTokens != 0 {
		t.Errorf("OutputTokens = %d, want 0", report.OutputTokens)
	}
}

// --- Latency ---

func TestLatency_EmptyEvents(t *testing.T) {
	dir := t.TempDir()
	agg := NewAggregator(dir)

	report, err := agg.Latency()
	if err != nil {
		t.Fatalf("Latency: %v", err)
	}
	if report.SampleCount != 0 {
		t.Errorf("SampleCount = %d, want 0", report.SampleCount)
	}
	if report.FirstTokenP50 != 0 {
		t.Errorf("FirstTokenP50 = %v, want 0", report.FirstTokenP50)
	}
}

func TestLatency_ComputesPercentilesCorrectly(t *testing.T) {
	dir := t.TempDir()
	// 10 first-token samples: 100, 200, 300, 400, 500, 600, 700, 800, 900, 1000 ms
	// 10 stream samples:      500, 1000, 1500, 2000, 2500, 3000, 3500, 4000, 4500, 5000 ms
	var lines []string
	firstTokenMs := []float64{100, 200, 300, 400, 500, 600, 700, 800, 900, 1000}
	streamMs := []float64{500, 1000, 1500, 2000, 2500, 3000, 3500, 4000, 4500, 5000}

	for i, ft := range firstTokenMs {
		lines = append(lines, fmt.Sprintf(
			`{"ts":"2026-03-09T10:00:%02dZ","event":"ws.stream.token","data":{"first_token_ms":%v}}`,
			i, ft,
		))
	}
	for i, s := range streamMs {
		lines = append(lines, fmt.Sprintf(
			`{"ts":"2026-03-09T10:01:%02dZ","event":"ws.stream.end","data":{"duration_ms":%v}}`,
			i, s,
		))
	}

	writeTestEvents(t, dir, lines)

	agg := NewAggregator(dir)
	report, err := agg.Latency()
	if err != nil {
		t.Fatalf("Latency: %v", err)
	}

	if report.SampleCount != 10 {
		t.Errorf("SampleCount = %d, want 10", report.SampleCount)
	}

	// p50 of [100..1000]: rank = 0.5*10 = 5, ceil=5, idx=4 → 500ms
	if report.FirstTokenP50 != 500*time.Millisecond {
		t.Errorf("FirstTokenP50 = %v, want 500ms", report.FirstTokenP50)
	}

	// p95 of [100..1000]: rank = 0.95*10 = 9.5, ceil=10, idx=9 → 1000ms
	if report.FirstTokenP95 != 1000*time.Millisecond {
		t.Errorf("FirstTokenP95 = %v, want 1000ms", report.FirstTokenP95)
	}

	// p99 of [100..1000]: rank = 0.99*10 = 9.9, ceil=10, idx=9 → 1000ms
	if report.FirstTokenP99 != 1000*time.Millisecond {
		t.Errorf("FirstTokenP99 = %v, want 1000ms", report.FirstTokenP99)
	}

	// p50 of [500..5000]: rank = 0.5*10 = 5, ceil=5, idx=4 → 2500ms
	if report.StreamP50 != 2500*time.Millisecond {
		t.Errorf("StreamP50 = %v, want 2500ms", report.StreamP50)
	}
}

func TestLatency_SingleSample(t *testing.T) {
	dir := t.TempDir()
	writeTestEvents(t, dir, []string{
		`{"ts":"2026-03-09T10:00:00Z","event":"ws.stream.token","data":{"first_token_ms":42.5}}`,
		`{"ts":"2026-03-09T10:00:01Z","event":"ws.stream.end","data":{"duration_ms":1234.5}}`,
	})

	agg := NewAggregator(dir)
	report, err := agg.Latency()
	if err != nil {
		t.Fatalf("Latency: %v", err)
	}

	if report.SampleCount != 1 {
		t.Errorf("SampleCount = %d, want 1", report.SampleCount)
	}

	expected := msToDuration(42.5)
	if report.FirstTokenP50 != expected {
		t.Errorf("FirstTokenP50 = %v, want %v", report.FirstTokenP50, expected)
	}
	if report.FirstTokenP95 != expected {
		t.Errorf("FirstTokenP95 = %v, want %v", report.FirstTokenP95, expected)
	}
	if report.FirstTokenP99 != expected {
		t.Errorf("FirstTokenP99 = %v, want %v", report.FirstTokenP99, expected)
	}

	expectedStream := msToDuration(1234.5)
	if report.StreamP50 != expectedStream {
		t.Errorf("StreamP50 = %v, want %v", report.StreamP50, expectedStream)
	}
}

func TestLatency_IgnoresZeroValues(t *testing.T) {
	dir := t.TempDir()
	writeTestEvents(t, dir, []string{
		`{"ts":"2026-03-09T10:00:00Z","event":"ws.stream.token","data":{"first_token_ms":0}}`,
		`{"ts":"2026-03-09T10:00:01Z","event":"ws.stream.token","data":{"first_token_ms":100}}`,
		`{"ts":"2026-03-09T10:00:02Z","event":"ws.stream.end","data":{"duration_ms":0}}`,
	})

	agg := NewAggregator(dir)
	report, err := agg.Latency()
	if err != nil {
		t.Fatalf("Latency: %v", err)
	}

	if report.SampleCount != 1 {
		t.Errorf("SampleCount = %d, want 1 (zero values excluded)", report.SampleCount)
	}
}

// --- Helpers ---

func TestPercentile_EmptySlice(t *testing.T) {
	result := percentile(nil, 50)
	if result != 0 {
		t.Errorf("percentile(nil, 50) = %f, want 0", result)
	}
}

func TestPercentile_SingleValue(t *testing.T) {
	result := percentile([]float64{42}, 99)
	if result != 42 {
		t.Errorf("percentile([42], 99) = %f, want 42", result)
	}
}

func TestPercentile_KnownValues(t *testing.T) {
	sorted := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	// p50: rank=5, idx=4 → 5
	if p := percentile(sorted, 50); p != 5 {
		t.Errorf("p50 = %f, want 5", p)
	}

	// p90: rank=9, idx=8 → 9
	if p := percentile(sorted, 90); p != 9 {
		t.Errorf("p90 = %f, want 9", p)
	}

	// p100: rank=10, idx=9 → 10
	if p := percentile(sorted, 100); p != 10 {
		t.Errorf("p100 = %f, want 10", p)
	}
}

func TestEstimateCost(t *testing.T) {
	// 1M input tokens = $3, 1M output tokens = $15
	cost := estimateCost(1_000_000, 1_000_000)
	if cost != 18.0 {
		t.Errorf("estimateCost(1M, 1M) = %f, want 18.0", cost)
	}

	// Zero tokens = $0
	cost = estimateCost(0, 0)
	if cost != 0 {
		t.Errorf("estimateCost(0, 0) = %f, want 0", cost)
	}
}

func TestGetStringField(t *testing.T) {
	data := map[string]interface{}{
		"name":   "test",
		"number": 42.0,
	}

	if v := getStringField(data, "name"); v != "test" {
		t.Errorf("getStringField(name) = %q, want %q", v, "test")
	}
	if v := getStringField(data, "missing"); v != "" {
		t.Errorf("getStringField(missing) = %q, want empty", v)
	}
	if v := getStringField(data, "number"); v != "" {
		t.Errorf("getStringField(number) = %q, want empty (not a string)", v)
	}
	if v := getStringField(nil, "name"); v != "" {
		t.Errorf("getStringField(nil, name) = %q, want empty", v)
	}
}

func TestGetIntField(t *testing.T) {
	data := map[string]interface{}{
		"count":  42.0, // JSON decodes numbers as float64
		"text":   "hello",
	}

	if v := getIntField(data, "count"); v != 42 {
		t.Errorf("getIntField(count) = %d, want 42", v)
	}
	if v := getIntField(data, "missing"); v != 0 {
		t.Errorf("getIntField(missing) = %d, want 0", v)
	}
	if v := getIntField(data, "text"); v != 0 {
		t.Errorf("getIntField(text) = %d, want 0 (not a number)", v)
	}
	if v := getIntField(nil, "count"); v != 0 {
		t.Errorf("getIntField(nil, count) = %d, want 0", v)
	}
}

func TestGetFloatField(t *testing.T) {
	data := map[string]interface{}{
		"value": 3.14,
		"text":  "hello",
	}

	if v := getFloatField(data, "value"); v != 3.14 {
		t.Errorf("getFloatField(value) = %f, want 3.14", v)
	}
	if v := getFloatField(data, "missing"); v != 0 {
		t.Errorf("getFloatField(missing) = %f, want 0", v)
	}
	if v := getFloatField(nil, "value"); v != 0 {
		t.Errorf("getFloatField(nil, value) = %f, want 0", v)
	}
}

