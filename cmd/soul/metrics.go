package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/rishav1305/soul-v2/internal/metrics"
)

func runMetrics(args []string) {
	if len(args) < 1 {
		fmt.Println("usage: soul metrics <subcommand>")
		fmt.Println("subcommands: status, quality, layers, cost, latency, alerts, db, requests, frontend, tail, log")
		os.Exit(1)
	}

	switch args[0] {
	case "status":
		runMetricsStatus()
	case "quality":
		runMetricsQuality()
	case "layers":
		runMetricsLayers()
	case "cost":
		runMetricsCost()
	case "latency":
		runMetricsLatency()
	case "alerts":
		runMetricsAlerts()
	case "db":
		runMetricsDB()
	case "requests":
		runMetricsRequests()
	case "frontend":
		runMetricsFrontend()
	case "tail":
		runMetricsTail(args[1:])
	case "log":
		runMetricsLog(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown metrics subcommand: %s\n", args[0])
		fmt.Println("subcommands: status, quality, layers, cost, latency, alerts, db, requests, frontend, tail, log")
		os.Exit(1)
	}
}

// runMetricsStatus handles: soul metrics status
func runMetricsStatus() {
	dataDir := getDataDir()
	agg := metrics.NewAggregator(dataDir)

	report, err := agg.Status()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== System Status ===")
	fmt.Printf("  Uptime:          %s\n", report.Uptime)
	fmt.Printf("  Total Sessions:  %d\n", report.TotalSessions)
	fmt.Printf("  Total Messages:  %d\n", report.TotalMessages)
	fmt.Printf("  Active Streams:  %d\n", report.ActiveStreams)
	fmt.Printf("  Total Errors:    %d\n", report.TotalErrors)
	if !report.LastEvent.IsZero() {
		fmt.Printf("  Last Event:      %s\n", report.LastEvent.Format("2006-01-02T15:04:05Z07:00"))
	} else {
		fmt.Printf("  Last Event:      (none)\n")
	}
}

// runMetricsQuality handles: soul metrics quality
func runMetricsQuality() {
	dataDir := getDataDir()
	agg := metrics.NewAggregator(dataDir)

	report, err := agg.Quality()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== Quality Report ===")
	fmt.Printf("  Total Errors:     %d\n", report.TotalErrors)
	fmt.Printf("  False Positives:  %d\n", report.FalsePositives)

	if len(report.ErrorCounts) > 0 {
		fmt.Println("\n  Error Counts by Type:")

		// Sort keys for deterministic output.
		keys := make([]string, 0, len(report.ErrorCounts))
		for k := range report.ErrorCounts {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			fmt.Printf("    %-16s %d\n", k, report.ErrorCounts[k])
		}
	}

	if len(report.QualityRatings) > 0 {
		fmt.Println("\n  Quality Ratings:")
		fmt.Printf("    %-20s %-8s %s\n", "STEP", "RATING", "NOTES")
		fmt.Printf("    %-20s %-8s %s\n", strings.Repeat("-", 20), strings.Repeat("-", 6), strings.Repeat("-", 30))
		for _, q := range report.QualityRatings {
			step := q.Step
			if len(step) > 20 {
				step = step[:17] + "..."
			}
			fmt.Printf("    %-20s %-8d %s\n", step, q.Rating, q.Notes)
		}
	}
}

// runMetricsLayers handles: soul metrics layers
func runMetricsLayers() {
	dataDir := getDataDir()
	agg := metrics.NewAggregator(dataDir)

	report, err := agg.Layers()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== Gate Layers Report ===")
	if len(report.GateResults) == 0 {
		fmt.Println("  No gate events recorded.")
		return
	}

	fmt.Printf("  %-16s %-8s %-8s %-8s %-8s %s\n", "GATE", "PASS", "FAIL", "RETRY", "TOTAL", "PASS%")
	fmt.Printf("  %-16s %-8s %-8s %-8s %-8s %s\n",
		strings.Repeat("-", 16), strings.Repeat("-", 6),
		strings.Repeat("-", 6), strings.Repeat("-", 6),
		strings.Repeat("-", 6), strings.Repeat("-", 6))

	for _, r := range report.GateResults {
		passRate := "N/A"
		if r.Total > 0 {
			passRate = fmt.Sprintf("%.0f%%", float64(r.Pass)/float64(r.Total)*100)
		}
		fmt.Printf("  %-16s %-8d %-8d %-8d %-8d %s\n",
			r.Gate, r.Pass, r.Fail, r.Retry, r.Total, passRate)
	}
}

// runMetricsCost handles: soul metrics cost
func runMetricsCost() {
	dataDir := getDataDir()
	agg := metrics.NewAggregator(dataDir)

	report, err := agg.Cost()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== Cost Report ===")
	fmt.Printf("  Total Requests:   %d\n", report.TotalRequests)
	fmt.Printf("  Input Tokens:     %d\n", report.InputTokens)
	fmt.Printf("  Output Tokens:    %d\n", report.OutputTokens)
	fmt.Printf("  Estimated Cost:   $%.4f\n", report.EstimatedCost)
}

// runMetricsLatency handles: soul metrics latency
func runMetricsLatency() {
	dataDir := getDataDir()
	agg := metrics.NewAggregator(dataDir)

	report, err := agg.Latency()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== Latency Report ===")
	fmt.Printf("  Sample Count:     %d\n", report.SampleCount)

	if report.SampleCount == 0 {
		fmt.Println("  No latency data recorded.")
		return
	}

	fmt.Println("\n  First Token Latency:")
	fmt.Printf("    P50:  %s\n", report.FirstTokenP50)
	fmt.Printf("    P95:  %s\n", report.FirstTokenP95)
	fmt.Printf("    P99:  %s\n", report.FirstTokenP99)

	fmt.Println("\n  Stream Duration:")
	fmt.Printf("    P50:  %s\n", report.StreamP50)
	fmt.Printf("    P95:  %s\n", report.StreamP95)
	fmt.Printf("    P99:  %s\n", report.StreamP99)
}

// runMetricsAlerts handles: soul metrics alerts
func runMetricsAlerts() {
	dataDir := getDataDir()
	agg := metrics.NewAggregator(dataDir)
	report, err := agg.Alerts()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("=== Alert Thresholds ===")
	if len(report.Alerts) == 0 {
		fmt.Println("  No threshold breaches recorded.")
		return
	}
	fmt.Printf("  %-20s %-16s %-12s %-10s %-10s %s\n", "TIMESTAMP", "METRIC", "FIELD", "VALUE", "THRESHOLD", "SEVERITY")
	for _, a := range report.Alerts {
		fmt.Printf("  %-20s %-16s %-12s %-10.0f %-10.0f %s\n",
			a.Timestamp.Format("01-02 15:04:05"), a.Metric, a.Field, a.Value, a.Threshold, a.Severity)
	}
}

// runMetricsDB handles: soul metrics db
func runMetricsDB() {
	dataDir := getDataDir()
	agg := metrics.NewAggregator(dataDir)
	report, err := agg.DB()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("=== Database Query Performance ===")
	if len(report.Methods) == 0 {
		fmt.Println("  No database events recorded.")
		return
	}
	// Sort method names for deterministic output
	methods := make([]string, 0, len(report.Methods))
	for m := range report.Methods {
		methods = append(methods, m)
	}
	sort.Strings(methods)

	fmt.Printf("  %-24s %-8s %-12s %-12s %-12s\n", "METHOD", "COUNT", "P50", "P95", "P99")
	for _, m := range methods {
		s := report.Methods[m]
		fmt.Printf("  %-24s %-8d %-12s %-12s %-12s\n", s.Method, s.Count, s.P50, s.P95, s.P99)
	}
	if len(report.SlowQueries) > 0 {
		fmt.Printf("\n  Slow Queries (last %d):\n", len(report.SlowQueries))
		for _, sq := range report.SlowQueries {
			fmt.Printf("    %s %-20s %.0fms %s\n",
				sq.Timestamp.Format("01-02 15:04:05"), sq.Method, sq.DurationMs, sq.SessionID)
		}
	}
}

// runMetricsRequests handles: soul metrics requests
func runMetricsRequests() {
	dataDir := getDataDir()
	agg := metrics.NewAggregator(dataDir)
	report, err := agg.Requests()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("=== HTTP Request Performance ===")
	if len(report.Paths) == 0 {
		fmt.Println("  No request events recorded.")
		return
	}
	paths := make([]string, 0, len(report.Paths))
	for p := range report.Paths {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	fmt.Printf("  %-30s %-8s %-12s %-12s %-12s\n", "PATH", "COUNT", "P50", "P95", "P99")
	for _, p := range paths {
		s := report.Paths[p]
		fmt.Printf("  %-30s %-8d %-12s %-12s %-12s\n", s.Path, s.Count, s.P50, s.P95, s.P99)
	}
	if len(report.StatusCodes) > 0 {
		fmt.Println("\n  Status Code Distribution:")
		codes := make([]int, 0, len(report.StatusCodes))
		for c := range report.StatusCodes {
			codes = append(codes, c)
		}
		sort.Ints(codes)
		for _, c := range codes {
			fmt.Printf("    %d: %d\n", c, report.StatusCodes[c])
		}
	}
}

// runMetricsFrontend handles: soul metrics frontend
func runMetricsFrontend() {
	dataDir := getDataDir()
	agg := metrics.NewAggregator(dataDir)
	report, err := agg.Frontend()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("=== Frontend Report ===")
	fmt.Printf("  Total Errors: %d\n", report.Errors)
	if len(report.TopErrors) > 0 {
		fmt.Println("\n  Errors by Component:")
		comps := make([]string, 0, len(report.TopErrors))
		for c := range report.TopErrors {
			comps = append(comps, c)
		}
		sort.Strings(comps)
		for _, c := range comps {
			fmt.Printf("    %-24s %d\n", c, report.TopErrors[c])
		}
	}
	if len(report.SlowRenders) > 0 {
		fmt.Printf("\n  Slow Renders (%d):\n", len(report.SlowRenders))
		for _, r := range report.SlowRenders {
			fmt.Printf("    %-24s %.0fms\n", r.Component, r.DurationMs)
		}
	}
}

// runMetricsTail handles: soul metrics tail [--type PREFIX] [-n COUNT]
func runMetricsTail(args []string) {
	fs := flag.NewFlagSet("metrics tail", flag.ExitOnError)
	typePrefix := fs.String("type", "", "filter events by type prefix (e.g. ws, api)")
	count := fs.Int("n", 20, "number of recent events to show")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	dataDir := getDataDir()

	var events []metrics.Event
	var err error

	if *typePrefix != "" {
		events, err = metrics.ReadLastNFiltered(dataDir, *typePrefix, *count)
	} else {
		events, err = metrics.ReadLastN(dataDir, *count)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading metrics: %v\n", err)
		os.Exit(1)
	}

	if len(events) == 0 {
		fmt.Println("no events found")
		return
	}

	for _, ev := range events {
		printEvent(ev)
	}
}

// runMetricsLog handles: soul metrics log <kind> [flags]
func runMetricsLog(args []string) {
	if len(args) < 1 {
		fmt.Println("usage: soul metrics log <kind>")
		fmt.Println("kinds: error, false-positive, quality")
		os.Exit(1)
	}

	kind := args[0]
	rest := args[1:]

	switch kind {
	case "error":
		runLogError(rest)
	case "false-positive":
		runLogFalsePositive(rest)
	case "quality":
		runLogQuality(rest)
	default:
		fmt.Fprintf(os.Stderr, "unknown log kind: %s\n", kind)
		fmt.Println("kinds: error, false-positive, quality")
		os.Exit(1)
	}
}

// runLogError handles: soul metrics log error --step STEP --type ERROR_TYPE --reason "REASON" [--should-have-caught LAYER]
func runLogError(args []string) {
	fs := flag.NewFlagSet("metrics log error", flag.ExitOnError)
	step := fs.String("step", "", "step identifier (required)")
	errType := fs.String("type", "", "error type from taxonomy (required)")
	reason := fs.String("reason", "", "human-readable reason (required)")
	shouldHaveCaught := fs.String("should-have-caught", "", "gate layer that should have caught this (optional)")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if *step == "" || *errType == "" || *reason == "" {
		fmt.Fprintln(os.Stderr, "error: --step, --type, and --reason are required")
		fs.Usage()
		os.Exit(1)
	}

	data := map[string]interface{}{
		"step":   *step,
		"type":   *errType,
		"reason": *reason,
	}
	if *shouldHaveCaught != "" {
		data["should_have_caught"] = *shouldHaveCaught
	}

	logEvent(metrics.EventOverrideError, data)
}

// runLogFalsePositive handles: soul metrics log false-positive --step STEP --reason "REASON"
func runLogFalsePositive(args []string) {
	fs := flag.NewFlagSet("metrics log false-positive", flag.ExitOnError)
	step := fs.String("step", "", "step identifier (required)")
	reason := fs.String("reason", "", "why this was a false positive (required)")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if *step == "" || *reason == "" {
		fmt.Fprintln(os.Stderr, "error: --step and --reason are required")
		fs.Usage()
		os.Exit(1)
	}

	data := map[string]interface{}{
		"step":   *step,
		"type":   metrics.ErrorFalsePositive,
		"reason": *reason,
	}

	logEvent(metrics.EventOverrideError, data)
}

// runLogQuality handles: soul metrics log quality --step STEP --rating N --notes "NOTES"
func runLogQuality(args []string) {
	fs := flag.NewFlagSet("metrics log quality", flag.ExitOnError)
	step := fs.String("step", "", "step identifier (required)")
	rating := fs.String("rating", "", "quality rating 1-5 (required)")
	notes := fs.String("notes", "", "quality notes (required)")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if *step == "" || *rating == "" || *notes == "" {
		fmt.Fprintln(os.Stderr, "error: --step, --rating, and --notes are required")
		fs.Usage()
		os.Exit(1)
	}

	ratingVal, err := strconv.Atoi(*rating)
	if err != nil || ratingVal < 1 || ratingVal > 5 {
		fmt.Fprintln(os.Stderr, "error: --rating must be an integer between 1 and 5")
		os.Exit(1)
	}

	data := map[string]interface{}{
		"step":   *step,
		"rating": ratingVal,
		"notes":  *notes,
	}

	logEvent(metrics.EventOverrideQuality, data)
}

// logEvent creates an EventLogger, logs a single event, and closes the logger.
func logEvent(eventType string, data map[string]interface{}) {
	dataDir := getDataDir()

	logger, err := metrics.NewEventLogger(dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating logger: %v\n", err)
		os.Exit(1)
	}

	if err := logger.Log(eventType, data); err != nil {
		logger.Close()
		fmt.Fprintf(os.Stderr, "error logging event: %v\n", err)
		os.Exit(1)
	}

	if err := logger.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "error closing logger: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("logged %s event\n", eventType)
}

// printEvent formats and prints a single event as: TIMESTAMP EVENT_TYPE DATA_JSON
func printEvent(ev metrics.Event) {
	ts := ev.Timestamp.Format("2006-01-02T15:04:05Z07:00")
	if ev.Data != nil && len(ev.Data) > 0 {
		dataJSON, err := json.Marshal(ev.Data)
		if err != nil {
			fmt.Printf("%s %s {}\n", ts, ev.EventType)
			return
		}
		fmt.Printf("%s %s %s\n", ts, ev.EventType, string(dataJSON))
	} else {
		fmt.Printf("%s %s\n", ts, ev.EventType)
	}
}

// getDataDir returns the data directory from SOUL_V2_DATA_DIR env var or default ~/.soul-v2.
func getDataDir() string {
	if dir := os.Getenv("SOUL_V2_DATA_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting home directory: %v\n", err)
		os.Exit(1)
	}
	return filepath.Join(home, ".soul-v2")
}
