package metrics

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestEvents(t *testing.T, dir string, lines []string) {
	t.Helper()
	path := filepath.Join(dir, metricsFileName)
	var data []byte
	for _, line := range lines {
		data = append(data, []byte(line+"\n")...)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("writeTestEvents: %v", err)
	}
}

func TestReadEvents_ReadsAllEvents(t *testing.T) {
	dir := t.TempDir()
	writeTestEvents(t, dir, []string{
		`{"ts":"2026-03-09T12:00:00Z","event":"system.start","data":{"port":3002}}`,
		`{"ts":"2026-03-09T12:00:01Z","event":"ws.connect","data":{"client":"abc"}}`,
		`{"ts":"2026-03-09T12:00:02Z","event":"api.request","data":{"path":"/chat"}}`,
	})

	events, err := ReadEvents(dir)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	if events[0].EventType != "system.start" {
		t.Errorf("events[0].EventType = %q, want %q", events[0].EventType, "system.start")
	}
	if events[1].EventType != "ws.connect" {
		t.Errorf("events[1].EventType = %q, want %q", events[1].EventType, "ws.connect")
	}
	if events[2].EventType != "api.request" {
		t.Errorf("events[2].EventType = %q, want %q", events[2].EventType, "api.request")
	}
}

func TestReadEventsFiltered_FiltersByTypePrefix(t *testing.T) {
	dir := t.TempDir()
	writeTestEvents(t, dir, []string{
		`{"ts":"2026-03-09T12:00:00Z","event":"system.start","data":{"port":3002}}`,
		`{"ts":"2026-03-09T12:00:01Z","event":"ws.connect","data":{"client":"abc"}}`,
		`{"ts":"2026-03-09T12:00:02Z","event":"ws.disconnect","data":{"client":"abc"}}`,
		`{"ts":"2026-03-09T12:00:03Z","event":"api.request","data":{"path":"/chat"}}`,
	})

	events, err := ReadEventsFiltered(dir, "ws")
	if err != nil {
		t.Fatalf("ReadEventsFiltered: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 ws events, got %d", len(events))
	}
	if events[0].EventType != "ws.connect" {
		t.Errorf("events[0].EventType = %q, want %q", events[0].EventType, "ws.connect")
	}
	if events[1].EventType != "ws.disconnect" {
		t.Errorf("events[1].EventType = %q, want %q", events[1].EventType, "ws.disconnect")
	}
}

func TestReadLastN_ReturnsLastNEvents(t *testing.T) {
	dir := t.TempDir()
	writeTestEvents(t, dir, []string{
		`{"ts":"2026-03-09T12:00:00Z","event":"system.start"}`,
		`{"ts":"2026-03-09T12:00:01Z","event":"ws.connect"}`,
		`{"ts":"2026-03-09T12:00:02Z","event":"api.request"}`,
		`{"ts":"2026-03-09T12:00:03Z","event":"ws.disconnect"}`,
		`{"ts":"2026-03-09T12:00:04Z","event":"system.stop"}`,
	})

	events, err := ReadLastN(dir, 2)
	if err != nil {
		t.Fatalf("ReadLastN: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].EventType != "ws.disconnect" {
		t.Errorf("events[0].EventType = %q, want %q", events[0].EventType, "ws.disconnect")
	}
	if events[1].EventType != "system.stop" {
		t.Errorf("events[1].EventType = %q, want %q", events[1].EventType, "system.stop")
	}
}

func TestReadLastN_ReturnsAllWhenNExceedsCount(t *testing.T) {
	dir := t.TempDir()
	writeTestEvents(t, dir, []string{
		`{"ts":"2026-03-09T12:00:00Z","event":"system.start"}`,
		`{"ts":"2026-03-09T12:00:01Z","event":"ws.connect"}`,
	})

	events, err := ReadLastN(dir, 100)
	if err != nil {
		t.Fatalf("ReadLastN: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
}

func TestReadLastNFiltered_FiltersAndReturnsLastN(t *testing.T) {
	dir := t.TempDir()
	writeTestEvents(t, dir, []string{
		`{"ts":"2026-03-09T12:00:00Z","event":"system.start"}`,
		`{"ts":"2026-03-09T12:00:01Z","event":"ws.connect","data":{"client":"a"}}`,
		`{"ts":"2026-03-09T12:00:02Z","event":"api.request"}`,
		`{"ts":"2026-03-09T12:00:03Z","event":"ws.stream.start","data":{"client":"a"}}`,
		`{"ts":"2026-03-09T12:00:04Z","event":"ws.stream.end","data":{"client":"a"}}`,
		`{"ts":"2026-03-09T12:00:05Z","event":"ws.disconnect","data":{"client":"a"}}`,
	})

	events, err := ReadLastNFiltered(dir, "ws", 2)
	if err != nil {
		t.Fatalf("ReadLastNFiltered: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].EventType != "ws.stream.end" {
		t.Errorf("events[0].EventType = %q, want %q", events[0].EventType, "ws.stream.end")
	}
	if events[1].EventType != "ws.disconnect" {
		t.Errorf("events[1].EventType = %q, want %q", events[1].EventType, "ws.disconnect")
	}
}

func TestReadEvents_EmptyFileReturnsEmptySlice(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, metricsFileName)
	if err := os.WriteFile(path, []byte(""), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	events, err := ReadEvents(dir)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}

func TestReadEvents_MissingFileReturnsEmptySlice(t *testing.T) {
	dir := t.TempDir()
	// Don't create any file.

	events, err := ReadEvents(dir)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}

func TestReadEvents_MalformedLinesAreSkipped(t *testing.T) {
	dir := t.TempDir()
	writeTestEvents(t, dir, []string{
		`{"ts":"2026-03-09T12:00:00Z","event":"system.start"}`,
		`this is not json`,
		`{"ts":"2026-03-09T12:00:02Z","event":"api.request"}`,
		`{"ts":"bad-ts","event":"ws.connect"}`,
		`{"ts":"2026-03-09T12:00:04Z","event":"system.stop"}`,
	})

	events, err := ReadEvents(dir)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	// Lines 2 (not json) and 4 (bad timestamp) should be skipped.
	if len(events) != 3 {
		t.Fatalf("expected 3 events (2 malformed skipped), got %d", len(events))
	}
	if events[0].EventType != "system.start" {
		t.Errorf("events[0].EventType = %q, want %q", events[0].EventType, "system.start")
	}
	if events[1].EventType != "api.request" {
		t.Errorf("events[1].EventType = %q, want %q", events[1].EventType, "api.request")
	}
	if events[2].EventType != "system.stop" {
		t.Errorf("events[2].EventType = %q, want %q", events[2].EventType, "system.stop")
	}
}

func TestReadLastN_ZeroReturnsEmptySlice(t *testing.T) {
	dir := t.TempDir()
	writeTestEvents(t, dir, []string{
		`{"ts":"2026-03-09T12:00:00Z","event":"system.start"}`,
	})

	events, err := ReadLastN(dir, 0)
	if err != nil {
		t.Fatalf("ReadLastN: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}

func TestReadEvents_BlankLinesAreSkipped(t *testing.T) {
	dir := t.TempDir()
	writeTestEvents(t, dir, []string{
		`{"ts":"2026-03-09T12:00:00Z","event":"system.start"}`,
		``,
		`   `,
		`{"ts":"2026-03-09T12:00:01Z","event":"system.stop"}`,
	})

	events, err := ReadEvents(dir)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
}

func TestReadEventsFiltered_NoMatchReturnsEmptySlice(t *testing.T) {
	dir := t.TempDir()
	writeTestEvents(t, dir, []string{
		`{"ts":"2026-03-09T12:00:00Z","event":"system.start"}`,
		`{"ts":"2026-03-09T12:00:01Z","event":"system.stop"}`,
	})

	events, err := ReadEventsFiltered(dir, "ws")
	if err != nil {
		t.Fatalf("ReadEventsFiltered: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}
