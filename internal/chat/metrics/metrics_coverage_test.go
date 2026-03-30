package metrics

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- isCurrentProductFile ---

func TestIsCurrentProductFile(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		{"current product file", "metrics-chat.jsonl", true},
		{"current product file 2", "metrics-tutor.jsonl", true},
		{"rotated product file", "metrics-chat-2026-03-15.jsonl", false},
		{"rotated product file 2", "metrics-tutor-2026-01-01.jsonl", false},
		{"legacy current file", "metrics.jsonl", false}, // no "metrics-" prefix
		{"legacy rotated file", "metrics.2026-03-15.jsonl", false},
		{"non-metrics prefix", "events-chat.jsonl", false},
		{"non-jsonl suffix", "metrics-chat.log", false},
		{"short product name", "metrics-x.jsonl", true},
		{"empty middle", "metrics-.jsonl", true},
		{"product with hyphens", "metrics-my-app.jsonl", true},
		{"product with hyphens and date-like suffix", "metrics-my-app-2026-03-15.jsonl", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isCurrentProductFile(tt.input)
			if got != tt.expect {
				t.Errorf("isCurrentProductFile(%q) = %v, want %v", tt.input, got, tt.expect)
			}
		})
	}
}

// --- metricsFiles ---

func TestMetricsFiles_ProductSpecific(t *testing.T) {
	dir := t.TempDir()

	// Create product-specific files.
	os.WriteFile(filepath.Join(dir, "metrics-chat.jsonl"), []byte{}, 0600)
	os.WriteFile(filepath.Join(dir, "metrics-chat-2026-03-07.jsonl"), []byte{}, 0600)
	os.WriteFile(filepath.Join(dir, "metrics-chat-2026-03-08.jsonl"), []byte{}, 0600)
	// Other product files should NOT be included.
	os.WriteFile(filepath.Join(dir, "metrics-tutor.jsonl"), []byte{}, 0600)
	os.WriteFile(filepath.Join(dir, "metrics.jsonl"), []byte{}, 0600)
	// Non-metrics file.
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte{}, 0600)

	files, err := metricsFiles(dir, "chat")
	if err != nil {
		t.Fatalf("metricsFiles: %v", err)
	}

	// Should include: rotated 03-07, rotated 03-08, current chat.
	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d: %v", len(files), files)
	}
	// Rotated first (sorted), then current.
	if !strings.HasSuffix(files[0], "metrics-chat-2026-03-07.jsonl") {
		t.Errorf("files[0] = %q, want rotated 03-07", files[0])
	}
	if !strings.HasSuffix(files[1], "metrics-chat-2026-03-08.jsonl") {
		t.Errorf("files[1] = %q, want rotated 03-08", files[1])
	}
	if !strings.HasSuffix(files[2], "metrics-chat.jsonl") {
		t.Errorf("files[2] = %q, want current chat", files[2])
	}
}

func TestMetricsFiles_NonExistentDir(t *testing.T) {
	files, err := metricsFiles("/nonexistent/path/to/dir", "")
	if err != nil {
		t.Fatalf("expected nil error for nonexistent dir, got %v", err)
	}
	if files != nil {
		t.Errorf("expected nil files, got %v", files)
	}
}

func TestMetricsFiles_AllProducts(t *testing.T) {
	dir := t.TempDir()

	// Mix of legacy, product current, product rotated, and non-metrics files.
	os.WriteFile(filepath.Join(dir, "metrics.jsonl"), []byte{}, 0600)
	os.WriteFile(filepath.Join(dir, "metrics.2026-03-07.jsonl"), []byte{}, 0600)
	os.WriteFile(filepath.Join(dir, "metrics-chat.jsonl"), []byte{}, 0600)
	os.WriteFile(filepath.Join(dir, "metrics-chat-2026-03-06.jsonl"), []byte{}, 0600)
	os.WriteFile(filepath.Join(dir, "data.json"), []byte{}, 0600) // ignored
	os.Mkdir(filepath.Join(dir, "subdir"), 0700)                  // ignored

	files, err := metricsFiles(dir, "")
	if err != nil {
		t.Fatalf("metricsFiles: %v", err)
	}

	// Should include all 4 metrics files (not data.json or subdir).
	if len(files) != 4 {
		t.Fatalf("expected 4 files, got %d: %v", len(files), files)
	}
	// Rotated files first (sorted), then current files.
	if !strings.Contains(files[0], "2026-03-06") {
		t.Errorf("files[0] should be earliest rotated, got %q", files[0])
	}
}

// --- NewEventLogger product migration ---

func TestNewEventLogger_ProductMigration(t *testing.T) {
	dir := t.TempDir()

	// Create a legacy metrics.jsonl file.
	legacyPath := filepath.Join(dir, "metrics.jsonl")
	os.WriteFile(legacyPath, []byte(`{"ts":"2026-03-09T12:00:00Z","event":"system.start"}`+"\n"), 0600)

	// Create logger with product — should migrate legacy → metrics-chat.jsonl.
	logger, err := NewEventLogger(dir, "chat")
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	defer logger.Close()

	// Legacy file should be gone (renamed).
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Error("legacy metrics.jsonl should be renamed after migration")
	}

	// Product file should exist.
	productPath := filepath.Join(dir, "metrics-chat.jsonl")
	if _, err := os.Stat(productPath); err != nil {
		t.Fatalf("product file should exist: %v", err)
	}

	// Product file should contain the old event.
	data, err := os.ReadFile(productPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "system.start") {
		t.Error("migrated file should contain legacy event")
	}
}

func TestNewEventLogger_ProductNoMigrationIfProductFileExists(t *testing.T) {
	dir := t.TempDir()

	// Both legacy and product files exist.
	os.WriteFile(filepath.Join(dir, "metrics.jsonl"), []byte("legacy\n"), 0600)
	os.WriteFile(filepath.Join(dir, "metrics-chat.jsonl"), []byte("product\n"), 0600)

	logger, err := NewEventLogger(dir, "chat")
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	defer logger.Close()

	// Legacy file should still exist (not renamed since product file already existed).
	if _, err := os.Stat(filepath.Join(dir, "metrics.jsonl")); err != nil {
		t.Error("legacy file should still exist when product file already exists")
	}
}

func TestNewEventLogger_InvalidPath(t *testing.T) {
	_, err := NewEventLogger("/proc/self/fd/999/nested/invalid", "")
	if err == nil {
		t.Error("expected error for invalid data dir path")
	}
}

// --- Logger product mode ---

func TestEventLogger_Log_ProductTagInjected(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewEventLogger(dir, "tutor")
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	defer logger.Close()

	// Log with nil data — should create data map and inject product.
	if err := logger.Log(EventSystemStart, nil); err != nil {
		t.Fatalf("Log: %v", err)
	}

	// Log with existing data — should add product to it.
	if err := logger.Log(EventAPIRequest, map[string]interface{}{"path": "/api"}); err != nil {
		t.Fatalf("Log: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "metrics-tutor.jsonl"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	for i, line := range lines {
		if !strings.Contains(line, `"product":"tutor"`) {
			t.Errorf("line %d missing product tag: %s", i, line)
		}
	}
}

func TestEventLogger_Rotate_ProductMode(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewEventLogger(dir, "chat")
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	defer logger.Close()

	_ = logger.Log(EventSystemStart, nil)

	if err := logger.Rotate(); err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	dateSuffix := time.Now().Format("2006-01-02")
	rotatedPath := filepath.Join(dir, "metrics-chat-"+dateSuffix+".jsonl")
	if _, err := os.Stat(rotatedPath); err != nil {
		t.Fatalf("rotated product file should exist: %v", err)
	}
}

func TestEventLogger_AutoRotate_ProductMode(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewEventLogger(dir, "tasks")
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	defer logger.Close()

	day1 := time.Date(2026, 3, 10, 23, 59, 0, 0, time.UTC)
	day2 := time.Date(2026, 3, 11, 0, 1, 0, 0, time.UTC)

	logger.nowFunc = func() time.Time { return day1 }
	logger.lastDate = day1.UTC().Format("2006-01-02")

	_ = logger.Log(EventSystemStart, nil)

	logger.nowFunc = func() time.Time { return day2 }
	_ = logger.Log(EventSystemStop, nil)

	// Rotated file should have day1's date in product format.
	rotatedPath := filepath.Join(dir, "metrics-tasks-2026-03-10.jsonl")
	if _, err := os.Stat(rotatedPath); err != nil {
		t.Fatalf("rotated product file should exist: %v", err)
	}
}

// --- Close error paths ---

func TestEventLogger_Close_DoubleClose(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewEventLogger(dir, "")
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}

	if err := logger.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	// Second close should return error (file already closed).
	err = logger.Close()
	if err == nil {
		t.Error("expected error on double close")
	}
}

// --- ReadAllProducts product filter ---

func TestReadAllProducts_SpecificProduct(t *testing.T) {
	dir := t.TempDir()

	// Write events to product-specific files.
	os.WriteFile(filepath.Join(dir, "metrics-chat.jsonl"), []byte(
		`{"ts":"2026-03-09T12:00:00Z","event":"system.start","data":{"product":"chat"}}`+"\n",
	), 0600)
	os.WriteFile(filepath.Join(dir, "metrics-chat-2026-03-08.jsonl"), []byte(
		`{"ts":"2026-03-08T12:00:00Z","event":"system.start","data":{"product":"chat"}}`+"\n",
	), 0600)
	// Tutor events should NOT be included.
	os.WriteFile(filepath.Join(dir, "metrics-tutor.jsonl"), []byte(
		`{"ts":"2026-03-09T12:00:00Z","event":"system.start","data":{"product":"tutor"}}`+"\n",
	), 0600)

	events, err := ReadAllProducts(dir, "chat")
	if err != nil {
		t.Fatalf("ReadAllProducts: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 chat events, got %d", len(events))
	}
	// Should be sorted by timestamp.
	if events[0].Timestamp.After(events[1].Timestamp) {
		t.Error("events should be sorted chronologically")
	}
}

func TestReadAllProducts_AllProducts(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "metrics-chat.jsonl"), []byte(
		`{"ts":"2026-03-09T12:00:01Z","event":"ws.connect"}`+"\n",
	), 0600)
	os.WriteFile(filepath.Join(dir, "metrics-tutor.jsonl"), []byte(
		`{"ts":"2026-03-09T12:00:00Z","event":"api.request"}`+"\n",
	), 0600)

	events, err := ReadAllProducts(dir, "")
	if err != nil {
		t.Fatalf("ReadAllProducts: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events from all products, got %d", len(events))
	}
	// Sorted by timestamp.
	if events[0].EventType != "api.request" {
		t.Errorf("events[0] = %q, want api.request (earlier timestamp)", events[0].EventType)
	}
}

func TestReadAllProducts_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	events, err := ReadAllProducts(dir, "chat")
	if err != nil {
		t.Fatalf("ReadAllProducts: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

// --- parseEventLine edge cases ---

func TestParseEventLine_MissingEventType(t *testing.T) {
	_, err := parseEventLine(`{"ts":"2026-03-09T12:00:00Z","data":{}}`)
	if err == nil {
		t.Error("expected error for missing event type")
	}
	if !strings.Contains(err.Error(), "missing event type") {
		t.Errorf("error = %q, want 'missing event type'", err.Error())
	}
}

func TestParseEventLine_RFC3339Fallback(t *testing.T) {
	// RFC3339 without nanoseconds — should fall back to RFC3339 parsing.
	ev, err := parseEventLine(`{"ts":"2026-03-09T12:00:00Z","event":"test"}`)
	if err != nil {
		t.Fatalf("parseEventLine: %v", err)
	}
	if ev.EventType != "test" {
		t.Errorf("EventType = %q, want test", ev.EventType)
	}
}

func TestParseEventLine_InvalidTimestamp(t *testing.T) {
	_, err := parseEventLine(`{"ts":"not-a-timestamp","event":"test"}`)
	if err == nil {
		t.Error("expected error for invalid timestamp")
	}
}

// --- metricsFileNameForProduct ---

func TestMetricsFileNameForProduct(t *testing.T) {
	if got := metricsFileNameForProduct(""); got != "metrics.jsonl" {
		t.Errorf("empty product = %q, want metrics.jsonl", got)
	}
	if got := metricsFileNameForProduct("chat"); got != "metrics-chat.jsonl" {
		t.Errorf("chat product = %q, want metrics-chat.jsonl", got)
	}
	if got := metricsFileNameForProduct("tutor"); got != "metrics-tutor.jsonl" {
		t.Errorf("tutor product = %q, want metrics-tutor.jsonl", got)
	}
}

// --- AlertChecker edge cases ---

func TestAlertChecker_NoThresholds(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewEventLogger(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	ac := NewAlertChecker(logger)
	logger.SetAlertChecker(ac)

	// Should not panic or error with no thresholds.
	if err := logger.Log("some.event", map[string]interface{}{"value": 100}); err != nil {
		t.Fatalf("Log: %v", err)
	}
}

func TestAlertChecker_BelowThreshold(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewEventLogger(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	ac := NewAlertChecker(logger)
	ac.AddThreshold(Threshold{Metric: "latency", Field: "p95", MaxValue: 1000.0, Severity: "warning"})
	logger.SetAlertChecker(ac)

	if err := logger.Log("latency", map[string]interface{}{"p95": 500.0}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, metricsFileName))
	if err != nil {
		t.Fatal(err)
	}
	// Should NOT contain alert.threshold since value is below max.
	if strings.Contains(string(data), EventAlertThreshold) {
		t.Error("should not fire alert when value is below threshold")
	}
}

func TestAlertChecker_WithDefaults(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewEventLogger(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	ac := NewAlertCheckerWithDefaults(logger)
	logger.SetAlertChecker(ac)

	// Trigger the db.query warning threshold (>100ms).
	if err := logger.Log(EventDBQuery, map[string]interface{}{"duration_ms": 200.0}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, metricsFileName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), EventAlertThreshold) {
		t.Error("expected alert.threshold from default db.query threshold")
	}
}

func TestAlertChecker_CopiesContextFields(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewEventLogger(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	ac := NewAlertChecker(logger)
	ac.AddThreshold(Threshold{Metric: EventAPIRequest, Field: "duration_ms", MaxValue: 100.0, Severity: "critical"})
	logger.SetAlertChecker(ac)

	// Include context fields that should be copied to the alert.
	if err := logger.Log(EventAPIRequest, map[string]interface{}{
		"duration_ms": 500.0,
		"method":      "GET",
		"path":        "/api/chat",
		"session_id":  "sess-123",
	}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, metricsFileName))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, `"method":"GET"`) {
		t.Error("alert should contain method context field")
	}
	if !strings.Contains(content, `"path":"/api/chat"`) {
		t.Error("alert should contain path context field")
	}
	if !strings.Contains(content, `"session_id":"sess-123"`) {
		t.Error("alert should contain session_id context field")
	}
}

// --- Log after alert.threshold should not re-check (prevent recursion) ---

func TestAlertChecker_SkipsAlertThresholdEvents(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewEventLogger(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	ac := NewAlertChecker(logger)
	// Add threshold that matches alert.threshold events — should be skipped.
	ac.AddThreshold(Threshold{Metric: EventAlertThreshold, Field: "value", MaxValue: 0, Severity: "critical"})
	logger.SetAlertChecker(ac)

	// Log an alert.threshold event directly — should NOT trigger re-check.
	if err := logger.Log(EventAlertThreshold, map[string]interface{}{"value": 100.0}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, metricsFileName))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	// Should have exactly 1 line (the original event), not 2 (no recursive alert).
	if len(lines) != 1 {
		t.Errorf("expected 1 line (no recursive alert), got %d", len(lines))
	}
}

// --- readEventsFromFile nonexistent ---

func TestReadEventsFromFile_Nonexistent(t *testing.T) {
	events, err := readEventsFromFile("/nonexistent/file.jsonl", "")
	if err != nil {
		t.Fatalf("expected nil error for nonexistent file, got %v", err)
	}
	if events != nil {
		t.Errorf("expected nil events, got %v", events)
	}
}

// --- lastN edge cases ---

func TestLastN_NegativeN(t *testing.T) {
	events := lastN([]Event{{EventType: "a"}}, -1)
	if len(events) != 0 {
		t.Errorf("expected 0 for negative n, got %d", len(events))
	}
}

func TestLastN_EmptySlice(t *testing.T) {
	events := lastN([]Event{}, 5)
	if len(events) != 0 {
		t.Errorf("expected 0 for empty slice, got %d", len(events))
	}
}
