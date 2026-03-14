package metrics

import (
	"os"
	"path/filepath"
	"strings"
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

func TestReader_MultipleFiles_ReadsAcrossRotatedFiles(t *testing.T) {
	dir := t.TempDir()

	// Create rotated files for two previous days.
	day1Lines := []string{
		`{"ts":"2026-03-07T10:00:00Z","event":"system.start","data":{"day":"1"}}`,
		`{"ts":"2026-03-07T18:00:00Z","event":"ws.connect","data":{"day":"1"}}`,
	}
	day2Lines := []string{
		`{"ts":"2026-03-08T10:00:00Z","event":"api.request","data":{"day":"2"}}`,
	}
	currentLines := []string{
		`{"ts":"2026-03-09T10:00:00Z","event":"system.stop","data":{"day":"3"}}`,
	}

	// Write rotated files.
	os.WriteFile(filepath.Join(dir, "metrics.2026-03-07.jsonl"),
		[]byte(strings.Join(day1Lines, "\n")+"\n"), 0600)
	os.WriteFile(filepath.Join(dir, "metrics.2026-03-08.jsonl"),
		[]byte(strings.Join(day2Lines, "\n")+"\n"), 0600)
	// Write current file.
	os.WriteFile(filepath.Join(dir, metricsFileName),
		[]byte(strings.Join(currentLines, "\n")+"\n"), 0600)

	events, err := ReadEvents(dir)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}

	if len(events) != 4 {
		t.Fatalf("expected 4 events across 3 files, got %d", len(events))
	}

	// Verify chronological order: day1 events first, then day2, then current.
	if events[0].EventType != "system.start" {
		t.Errorf("events[0].EventType = %q, want system.start", events[0].EventType)
	}
	if events[1].EventType != "ws.connect" {
		t.Errorf("events[1].EventType = %q, want ws.connect", events[1].EventType)
	}
	if events[2].EventType != "api.request" {
		t.Errorf("events[2].EventType = %q, want api.request", events[2].EventType)
	}
	if events[3].EventType != "system.stop" {
		t.Errorf("events[3].EventType = %q, want system.stop", events[3].EventType)
	}
}

func TestReader_MultipleFiles_FilteredReadsAcrossFiles(t *testing.T) {
	dir := t.TempDir()

	// Create rotated file with mixed events.
	os.WriteFile(filepath.Join(dir, "metrics.2026-03-08.jsonl"), []byte(
		`{"ts":"2026-03-08T10:00:00Z","event":"ws.connect","data":{"client":"a"}}`+"\n"+
			`{"ts":"2026-03-08T10:00:01Z","event":"system.start"}`+"\n",
	), 0600)

	// Current file with mixed events.
	os.WriteFile(filepath.Join(dir, metricsFileName), []byte(
		`{"ts":"2026-03-09T10:00:00Z","event":"ws.disconnect","data":{"client":"a"}}`+"\n"+
			`{"ts":"2026-03-09T10:00:01Z","event":"api.request"}`+"\n",
	), 0600)

	events, err := ReadEventsFiltered(dir, "ws")
	if err != nil {
		t.Fatalf("ReadEventsFiltered: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 ws events across files, got %d", len(events))
	}
	if events[0].EventType != "ws.connect" {
		t.Errorf("events[0].EventType = %q, want ws.connect", events[0].EventType)
	}
	if events[1].EventType != "ws.disconnect" {
		t.Errorf("events[1].EventType = %q, want ws.disconnect", events[1].EventType)
	}
}

func TestReader_MultipleFiles_LastNAcrossFiles(t *testing.T) {
	dir := t.TempDir()

	// Rotated file: 2 events.
	os.WriteFile(filepath.Join(dir, "metrics.2026-03-08.jsonl"), []byte(
		`{"ts":"2026-03-08T10:00:00Z","event":"system.start"}`+"\n"+
			`{"ts":"2026-03-08T18:00:00Z","event":"ws.connect"}`+"\n",
	), 0600)

	// Current file: 1 event.
	os.WriteFile(filepath.Join(dir, metricsFileName), []byte(
		`{"ts":"2026-03-09T10:00:00Z","event":"system.stop"}`+"\n",
	), 0600)

	events, err := ReadLastN(dir, 2)
	if err != nil {
		t.Fatalf("ReadLastN: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	// Last 2 of 3 total: ws.connect, system.stop
	if events[0].EventType != "ws.connect" {
		t.Errorf("events[0].EventType = %q, want ws.connect", events[0].EventType)
	}
	if events[1].EventType != "system.stop" {
		t.Errorf("events[1].EventType = %q, want system.stop", events[1].EventType)
	}
}

func TestReader_MultipleFiles_EmptyDirReturnsEmptySlice(t *testing.T) {
	dir := t.TempDir()
	events, err := ReadEvents(dir)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}

func TestReader_MultipleFiles_OnlyRotatedNoCurrentFile(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "metrics.2026-03-07.jsonl"), []byte(
		`{"ts":"2026-03-07T10:00:00Z","event":"system.start"}`+"\n",
	), 0600)

	events, err := ReadEvents(dir)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event from rotated file, got %d", len(events))
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
