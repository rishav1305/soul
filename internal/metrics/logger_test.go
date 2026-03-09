package metrics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestEvent_Validate_RejectsEmptyType(t *testing.T) {
	ev := Event{
		Timestamp: time.Now(),
		EventType: "",
	}
	if err := ev.Validate(); err == nil {
		t.Fatal("expected error for empty event type, got nil")
	}
}

func TestEvent_Validate_RejectsZeroTimestamp(t *testing.T) {
	ev := Event{
		EventType: EventSystemStart,
	}
	if err := ev.Validate(); err == nil {
		t.Fatal("expected error for zero timestamp, got nil")
	}
}

func TestEvent_Validate_AcceptsValid(t *testing.T) {
	ev := Event{
		Timestamp: time.Now(),
		EventType: EventSystemStart,
	}
	if err := ev.Validate(); err != nil {
		t.Fatalf("expected nil error for valid event, got %v", err)
	}
}

func TestEvent_MarshalJSON_ProducesValidJSON(t *testing.T) {
	now := time.Now()
	ev := Event{
		Timestamp: now,
		EventType: EventWSConnect,
		Data: map[string]interface{}{
			"client_id": "abc123",
			"origin":    "http://localhost",
		},
	}

	buf, err := ev.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	// Verify it is valid JSON.
	var parsed map[string]interface{}
	if err := json.Unmarshal(buf, &parsed); err != nil {
		t.Fatalf("produced invalid JSON: %v\nraw: %s", err, string(buf))
	}

	// Check required fields.
	if _, ok := parsed["ts"]; !ok {
		t.Error("missing 'ts' field")
	}
	if _, ok := parsed["event"]; !ok {
		t.Error("missing 'event' field")
	}
	if parsed["event"] != EventWSConnect {
		t.Errorf("event = %v, want %v", parsed["event"], EventWSConnect)
	}

	// Verify timestamp format is RFC3339Nano.
	ts := parsed["ts"].(string)
	if _, err := time.Parse(time.RFC3339Nano, ts); err != nil {
		t.Errorf("timestamp not RFC3339Nano: %v", err)
	}
}

func TestEvent_MarshalJSON_OmitsEmptyData(t *testing.T) {
	ev := Event{
		Timestamp: time.Now(),
		EventType: EventSystemStop,
	}

	buf, err := ev.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	if strings.Contains(string(buf), `"data"`) {
		t.Error("expected data field to be omitted when nil")
	}
}

func TestEventLogger_Log_WritesJSONLine(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewEventLogger(dir)
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	defer logger.Close()

	err = logger.Log(EventSystemStart, map[string]interface{}{
		"version": "0.1.0",
		"port":    3002,
	})
	if err != nil {
		t.Fatalf("Log: %v", err)
	}

	// Read back the file.
	data, err := os.ReadFile(filepath.Join(dir, metricsFileName))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
		t.Fatalf("line is not valid JSON: %v", err)
	}

	if parsed["event"] != EventSystemStart {
		t.Errorf("event = %v, want %v", parsed["event"], EventSystemStart)
	}
}

func TestEventLogger_Log_GoroutineSafe(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewEventLogger(dir)
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	defer logger.Close()

	const numGoroutines = 10
	const eventsPerGoroutine = 50
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				err := logger.Log(EventAPIRequest, map[string]interface{}{
					"goroutine": id,
					"index":     j,
				})
				if err != nil {
					t.Errorf("Log from goroutine %d: %v", id, err)
				}
			}
		}(i)
	}
	wg.Wait()

	// Read back and verify all lines are valid JSON.
	data, err := os.ReadFile(filepath.Join(dir, metricsFileName))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	expectedLines := numGoroutines * eventsPerGoroutine
	if len(lines) != expectedLines {
		t.Fatalf("expected %d lines, got %d", expectedLines, len(lines))
	}

	for i, line := range lines {
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			t.Errorf("line %d is not valid JSON: %v\nline: %s", i, err, line)
		}
	}
}

func TestEventLogger_Close_FlushesAndCloses(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewEventLogger(dir)
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}

	_ = logger.Log(EventSystemStart, nil)
	err = logger.Close()
	if err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Verify file exists and has content.
	data, err := os.ReadFile(filepath.Join(dir, metricsFileName))
	if err != nil {
		t.Fatalf("ReadFile after close: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected data in file after close, got empty")
	}

	// Writing after close should fail.
	err = logger.Log(EventSystemStop, nil)
	if err == nil {
		t.Error("expected error writing after close, got nil")
	}
}

func TestEventLogger_CreatesFileIfNotExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, metricsFileName)

	// Confirm file does not exist yet.
	if _, err := os.Stat(path); err == nil {
		t.Fatal("file should not exist before NewEventLogger")
	}

	logger, err := NewEventLogger(dir)
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	defer logger.Close()

	// Confirm file now exists.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("file should exist after NewEventLogger: %v", err)
	}

	// Check permissions (0600).
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}
}

func TestEventLogger_CreatesDataDirIfNotExists(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "data")

	// Confirm dir does not exist.
	if _, err := os.Stat(dir); err == nil {
		t.Fatal("dir should not exist before NewEventLogger")
	}

	logger, err := NewEventLogger(dir)
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	defer logger.Close()

	// Confirm dir now exists.
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("dir should exist after NewEventLogger: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory, got file")
	}

	// Check permissions (0700).
	perm := info.Mode().Perm()
	if perm != 0700 {
		t.Errorf("dir permissions = %o, want 0700", perm)
	}
}

func TestEventLogger_Rotate(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewEventLogger(dir)
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	defer logger.Close()

	// Write an event before rotation.
	_ = logger.Log(EventSystemStart, map[string]interface{}{"phase": "before"})

	// Rotate.
	err = logger.Rotate()
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	// The rotated file should exist with today's date.
	dateSuffix := time.Now().Format("2006-01-02")
	rotatedPath := filepath.Join(dir, "metrics."+dateSuffix+".jsonl")
	if _, err := os.Stat(rotatedPath); err != nil {
		t.Fatalf("rotated file should exist: %v", err)
	}

	// Rotated file should contain the pre-rotation event.
	data, err := os.ReadFile(rotatedPath)
	if err != nil {
		t.Fatalf("ReadFile rotated: %v", err)
	}
	if !strings.Contains(string(data), `"phase":"before"`) {
		t.Error("rotated file should contain pre-rotation event")
	}

	// New metrics.jsonl should exist and be empty (or writable).
	_ = logger.Log(EventSystemStart, map[string]interface{}{"phase": "after"})

	newData, err := os.ReadFile(filepath.Join(dir, metricsFileName))
	if err != nil {
		t.Fatalf("ReadFile new: %v", err)
	}
	if !strings.Contains(string(newData), `"phase":"after"`) {
		t.Error("new file should contain post-rotation event")
	}

	// New file should NOT contain the old event.
	if strings.Contains(string(newData), `"phase":"before"`) {
		t.Error("new file should not contain pre-rotation event")
	}
}

func TestLogger_Rotation_AutoRotatesOnDateChange(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewEventLogger(dir)
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	defer logger.Close()

	// Start on day 1.
	day1 := time.Date(2026, 3, 8, 23, 59, 0, 0, time.UTC)
	day2 := time.Date(2026, 3, 9, 0, 1, 0, 0, time.UTC)

	logger.nowFunc = func() time.Time { return day1 }
	logger.lastDate = day1.UTC().Format("2006-01-02")

	// Log an event on day 1.
	if err := logger.Log(EventSystemStart, map[string]interface{}{"day": "1"}); err != nil {
		t.Fatalf("Log day1: %v", err)
	}

	// Advance the clock to day 2.
	logger.nowFunc = func() time.Time { return day2 }

	// Log an event on day 2 — should trigger auto-rotation.
	if err := logger.Log(EventSystemStop, map[string]interface{}{"day": "2"}); err != nil {
		t.Fatalf("Log day2: %v", err)
	}

	// Verify rotated file exists with day 1's date.
	rotatedPath := filepath.Join(dir, "metrics.2026-03-08.jsonl")
	data, err := os.ReadFile(rotatedPath)
	if err != nil {
		t.Fatalf("rotated file should exist: %v", err)
	}
	if !strings.Contains(string(data), `"day":"1"`) {
		t.Error("rotated file should contain day 1 event")
	}
	if strings.Contains(string(data), `"day":"2"`) {
		t.Error("rotated file should NOT contain day 2 event")
	}

	// Verify current file has only day 2 event.
	currentData, err := os.ReadFile(filepath.Join(dir, metricsFileName))
	if err != nil {
		t.Fatalf("ReadFile current: %v", err)
	}
	if !strings.Contains(string(currentData), `"day":"2"`) {
		t.Error("current file should contain day 2 event")
	}
	if strings.Contains(string(currentData), `"day":"1"`) {
		t.Error("current file should NOT contain day 1 event")
	}
}

func TestLogger_Rotation_NoRotateOnSameDate(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewEventLogger(dir)
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	defer logger.Close()

	fixedTime := time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC)
	logger.nowFunc = func() time.Time { return fixedTime }
	logger.lastDate = fixedTime.UTC().Format("2006-01-02")

	// Log two events on the same date.
	if err := logger.Log(EventSystemStart, nil); err != nil {
		t.Fatalf("Log 1: %v", err)
	}
	if err := logger.Log(EventSystemStop, nil); err != nil {
		t.Fatalf("Log 2: %v", err)
	}

	// Verify no rotated file was created.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, entry := range entries {
		if entry.Name() != metricsFileName {
			t.Errorf("unexpected rotated file: %s", entry.Name())
		}
	}

	// Verify both events are in the current file.
	data, err := os.ReadFile(filepath.Join(dir, metricsFileName))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
}

func TestLogger_Rotation_MultiDayRotation(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewEventLogger(dir)
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	defer logger.Close()

	day1 := time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	day3 := time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC)

	logger.nowFunc = func() time.Time { return day1 }
	logger.lastDate = day1.UTC().Format("2006-01-02")
	_ = logger.Log(EventSystemStart, map[string]interface{}{"day": "1"})

	logger.nowFunc = func() time.Time { return day2 }
	_ = logger.Log(EventWSConnect, map[string]interface{}{"day": "2"})

	logger.nowFunc = func() time.Time { return day3 }
	_ = logger.Log(EventAPIRequest, map[string]interface{}{"day": "3"})

	// Should have two rotated files and one current file.
	if _, err := os.Stat(filepath.Join(dir, "metrics.2026-03-07.jsonl")); err != nil {
		t.Errorf("expected metrics.2026-03-07.jsonl: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "metrics.2026-03-08.jsonl")); err != nil {
		t.Errorf("expected metrics.2026-03-08.jsonl: %v", err)
	}

	// Current file should only have day 3 event.
	currentData, err := os.ReadFile(filepath.Join(dir, metricsFileName))
	if err != nil {
		t.Fatalf("ReadFile current: %v", err)
	}
	if !strings.Contains(string(currentData), `"day":"3"`) {
		t.Error("current file should contain day 3 event")
	}
	lines := strings.Split(strings.TrimSpace(string(currentData)), "\n")
	if len(lines) != 1 {
		t.Errorf("current file should have 1 line, got %d", len(lines))
	}
}

func TestEventLogger_Log_RejectsEmptyType(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewEventLogger(dir)
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	defer logger.Close()

	err = logger.Log("", nil)
	if err == nil {
		t.Error("expected error for empty event type, got nil")
	}
}
