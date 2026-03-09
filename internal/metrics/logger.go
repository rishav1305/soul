package metrics

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const metricsFileName = "metrics.jsonl"

// EventLogger writes events as JSON Lines to an append-only file.
// All writes are goroutine-safe via sync.Mutex.
// On each Log() call it checks if the UTC date has changed since the last
// write and, if so, rotates the current metrics.jsonl to
// metrics.YYYY-MM-DD.jsonl before writing the new event.
type EventLogger struct {
	file    *os.File
	mu      sync.Mutex
	dataDir string

	// lastDate tracks the UTC date (YYYY-MM-DD) of the most recent write.
	// When this changes, checkRotate triggers a rotation.
	lastDate string

	// nowFunc is an injectable clock for testing rotation without real time changes.
	nowFunc func() time.Time
}

// NewEventLogger creates a new EventLogger that writes to dataDir/metrics.jsonl.
// It creates the data directory (0700) and file (0600) if they do not exist.
func NewEventLogger(dataDir string) (*EventLogger, error) {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	path := filepath.Join(dataDir, metricsFileName)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("open metrics file: %w", err)
	}

	return &EventLogger{
		file:     f,
		dataDir:  dataDir,
		lastDate: time.Now().UTC().Format("2006-01-02"),
		nowFunc:  time.Now,
	}, nil
}

// Log writes an event with the given type and data to the JSONL file.
// It creates the event with the current time, validates it, marshals to JSON,
// writes a single line, and flushes immediately.
// Before writing, it checks if the UTC date has changed and rotates if needed.
func (l *EventLogger) Log(eventType string, data map[string]interface{}) error {
	now := l.nowFunc()
	ev := Event{
		Timestamp: now,
		EventType: eventType,
		Data:      data,
	}

	if err := ev.Validate(); err != nil {
		return fmt.Errorf("invalid event: %w", err)
	}

	buf, err := ev.MarshalJSON()
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	buf = append(buf, '\n')

	l.mu.Lock()
	defer l.mu.Unlock()

	// Check if the date has changed (rotation needed).
	if err := l.checkRotate(now); err != nil {
		// Log rotation failure to stderr but continue writing — losing
		// events is worse than failing to rotate.
		fmt.Fprintf(os.Stderr, "metrics: rotation failed: %v\n", err)
	}

	if _, err := l.file.Write(buf); err != nil {
		return fmt.Errorf("write event: %w", err)
	}
	if err := l.file.Sync(); err != nil {
		return fmt.Errorf("flush event: %w", err)
	}

	return nil
}

// checkRotate rotates the current metrics.jsonl to metrics.YYYY-MM-DD.jsonl
// if the UTC date has changed since the last write. The caller must hold l.mu.
func (l *EventLogger) checkRotate(now time.Time) error {
	currentDate := now.UTC().Format("2006-01-02")
	if currentDate == l.lastDate {
		return nil
	}

	// Date has changed — rotate the file.
	// The rotated file gets the *previous* date (the date of the data it contains).
	previousDate := l.lastDate

	if err := l.file.Sync(); err != nil {
		return fmt.Errorf("flush before rotate: %w", err)
	}

	oldPath := filepath.Join(l.dataDir, metricsFileName)
	newPath := filepath.Join(l.dataDir, fmt.Sprintf("metrics.%s.jsonl", previousDate))

	if err := l.file.Close(); err != nil {
		return fmt.Errorf("close for rotate: %w", err)
	}

	if err := os.Rename(oldPath, newPath); err != nil {
		// Re-open old file as fallback.
		l.file, _ = os.OpenFile(oldPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		return fmt.Errorf("rename metrics file: %w", err)
	}

	f, err := os.OpenFile(oldPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("open new metrics file: %w", err)
	}
	l.file = f
	l.lastDate = currentDate

	return nil
}

// Close flushes any buffered data and closes the underlying file handle.
func (l *EventLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.file.Sync(); err != nil {
		return fmt.Errorf("flush on close: %w", err)
	}
	if err := l.file.Close(); err != nil {
		return fmt.Errorf("close metrics file: %w", err)
	}
	return nil
}

// Rotate renames the current metrics file with a date suffix (metrics.YYYY-MM-DD.jsonl)
// and opens a fresh metrics.jsonl for new writes.
func (l *EventLogger) Rotate() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Flush before rotating.
	if err := l.file.Sync(); err != nil {
		return fmt.Errorf("flush before rotate: %w", err)
	}

	oldPath := filepath.Join(l.dataDir, metricsFileName)
	dateSuffix := l.nowFunc().Format("2006-01-02")
	newPath := filepath.Join(l.dataDir, fmt.Sprintf("metrics.%s.jsonl", dateSuffix))

	// Close current handle before rename.
	if err := l.file.Close(); err != nil {
		return fmt.Errorf("close for rotate: %w", err)
	}

	if err := os.Rename(oldPath, newPath); err != nil {
		// Re-open old file as fallback.
		l.file, _ = os.OpenFile(oldPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		return fmt.Errorf("rename metrics file: %w", err)
	}

	f, err := os.OpenFile(oldPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("open new metrics file: %w", err)
	}
	l.file = f
	l.lastDate = l.nowFunc().UTC().Format("2006-01-02")

	return nil
}
