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
type EventLogger struct {
	file    *os.File
	mu      sync.Mutex
	dataDir string
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
		file:    f,
		dataDir: dataDir,
	}, nil
}

// Log writes an event with the given type and data to the JSONL file.
// It creates the event with the current time, validates it, marshals to JSON,
// writes a single line, and flushes immediately.
func (l *EventLogger) Log(eventType string, data map[string]interface{}) error {
	ev := Event{
		Timestamp: time.Now(),
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

	if _, err := l.file.Write(buf); err != nil {
		return fmt.Errorf("write event: %w", err)
	}
	if err := l.file.Sync(); err != nil {
		return fmt.Errorf("flush event: %w", err)
	}

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
	dateSuffix := time.Now().Format("2006-01-02")
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

	return nil
}
