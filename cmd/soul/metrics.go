package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/rishav1305/soul-v2/internal/metrics"
)

func runMetrics(args []string) {
	if len(args) < 1 {
		fmt.Println("usage: soul metrics <subcommand>")
		fmt.Println("subcommands: tail, log")
		os.Exit(1)
	}

	switch args[0] {
	case "tail":
		runMetricsTail(args[1:])
	case "log":
		runMetricsLog(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown metrics subcommand: %s\n", args[0])
		fmt.Println("subcommands: tail, log")
		os.Exit(1)
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
