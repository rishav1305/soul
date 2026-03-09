package metrics

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

// readEvents reads events from metrics.jsonl, optionally filtering by type prefix.
func readEvents(dataDir string, typePrefix string) ([]Event, error) {
	path := filepath.Join(dataDir, metricsFileName)

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Event{}, nil
		}
		return nil, fmt.Errorf("open metrics file: %w", err)
	}
	defer f.Close()

	var events []Event
	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		ev, err := parseEventLine(line)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: metrics.jsonl line %d: %v\n", lineNum, err)
			continue
		}

		if typePrefix != "" && !strings.HasPrefix(ev.EventType, typePrefix) {
			continue
		}

		events = append(events, ev)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read metrics file: %w", err)
	}

	if events == nil {
		events = []Event{}
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
