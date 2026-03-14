package metrics

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ReadEvents reads all events from the metrics.jsonl file.
// Returns an empty slice (not an error) if the file does not exist or is empty.
// Malformed JSON lines are skipped with a warning to stderr.
func ReadEvents(dataDir string) ([]Event, error) {
	return readEvents(dataDir, "")
}

// ReadEventsFiltered reads events matching a type prefix.
// For example, prefix "ws" matches "ws.connect", "ws.disconnect", etc.
func ReadEventsFiltered(dataDir string, typePrefix string) ([]Event, error) {
	return readEvents(dataDir, typePrefix)
}

// ReadLastN reads the last N events from the file.
func ReadLastN(dataDir string, n int) ([]Event, error) {
	events, err := readEvents(dataDir, "")
	if err != nil {
		return nil, err
	}
	return lastN(events, n), nil
}

// ReadLastNFiltered reads the last N events matching a type prefix.
func ReadLastNFiltered(dataDir string, typePrefix string, n int) ([]Event, error) {
	events, err := readEvents(dataDir, typePrefix)
	if err != nil {
		return nil, err
	}
	return lastN(events, n), nil
}

// readEvents reads events from all metrics JSONL files in dataDir, optionally
// filtering by type prefix. It reads rotated files (metrics.YYYY-MM-DD.jsonl)
// in chronological order first, then the current metrics.jsonl, so that events
// are returned in temporal order.
func readEvents(dataDir string, typePrefix string) ([]Event, error) {
	files, err := metricsFiles(dataDir)
	if err != nil {
		return nil, err
	}

	var events []Event
	for _, path := range files {
		fileEvents, err := readEventsFromFile(path, typePrefix)
		if err != nil {
			return nil, err
		}
		events = append(events, fileEvents...)
	}

	if events == nil {
		events = []Event{}
	}
	return events, nil
}

// metricsFiles returns all metrics JSONL file paths in dataDir, sorted so that
// rotated files (metrics.YYYY-MM-DD.jsonl) come first in chronological order,
// followed by the current metrics.jsonl.
func metricsFiles(dataDir string) ([]string, error) {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read data dir: %w", err)
	}

	var rotated []string
	hasCurrentFile := false

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == metricsFileName {
			hasCurrentFile = true
			continue
		}
		// Match rotated files: metrics.YYYY-MM-DD.jsonl
		if strings.HasPrefix(name, "metrics.") && strings.HasSuffix(name, ".jsonl") {
			rotated = append(rotated, filepath.Join(dataDir, name))
		}
	}

	// Sort rotated files chronologically (the date is embedded in the filename).
	sort.Strings(rotated)

	// Append current file last (it contains the most recent events).
	if hasCurrentFile {
		rotated = append(rotated, filepath.Join(dataDir, metricsFileName))
	}

	return rotated, nil
}

// readEventsFromFile reads events from a single JSONL file, optionally filtering
// by type prefix. Returns empty slice (not error) if the file does not exist.
func readEventsFromFile(path string, typePrefix string) ([]Event, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open metrics file %s: %w", filepath.Base(path), err)
	}
	defer f.Close()

	var events []Event
	scanner := bufio.NewScanner(f)
	fileName := filepath.Base(path)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		ev, err := parseEventLine(line)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: %s line %d: %v\n", fileName, lineNum, err)
			continue
		}

		if typePrefix != "" && !strings.HasPrefix(ev.EventType, typePrefix) {
			continue
		}

		events = append(events, ev)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", fileName, err)
	}

	return events, nil
}

// parseEventLine parses a single JSON line into an Event.
func parseEventLine(line string) (Event, error) {
	// Parse into raw map first to handle the timestamp string.
	var raw struct {
		Timestamp string                 `json:"ts"`
		EventType string                 `json:"event"`
		Data      map[string]interface{} `json:"data,omitempty"`
	}

	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return Event{}, fmt.Errorf("invalid JSON: %w", err)
	}

	if raw.EventType == "" {
		return Event{}, fmt.Errorf("missing event type")
	}

	ts, err := time.Parse(time.RFC3339Nano, raw.Timestamp)
	if err != nil {
		// Try RFC3339 as fallback.
		ts, err = time.Parse(time.RFC3339, raw.Timestamp)
		if err != nil {
			return Event{}, fmt.Errorf("invalid timestamp %q: %w", raw.Timestamp, err)
		}
	}

	return Event{
		Timestamp: ts,
		EventType: raw.EventType,
		Data:      raw.Data,
	}, nil
}

// lastN returns the last n elements of events, or all if len < n.
func lastN(events []Event, n int) []Event {
	if n <= 0 || len(events) == 0 {
		return []Event{}
	}
	if n >= len(events) {
		return events
	}
	return events[len(events)-n:]
}
