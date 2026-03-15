package metrics

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rishav1305/soul-v2/pkg/events"
)

// Compile-time check: EventLogger satisfies events.Logger.
var _ events.Logger = (*EventLogger)(nil)

const metricsFileName = "metrics.jsonl"

// metricsFileNameForProduct returns the JSONL filename for a given product.
// If product is empty, it returns the legacy "metrics.jsonl".
func metricsFileNameForProduct(product string) string {
	if product == "" {
		return metricsFileName
	}
	return fmt.Sprintf("metrics-%s.jsonl", product)
}

// EventLogger writes events as JSON Lines to an append-only file.
// All writes are goroutine-safe via sync.Mutex.
// On each Log() call it checks if the UTC date has changed since the last
// write and, if so, rotates the current metrics file to a dated variant
// before writing the new event.
type EventLogger struct {
	file    *os.File
	mu      sync.Mutex
	dataDir string
	product string // product tag (e.g. "chat"); empty = legacy mode

	// lastDate tracks the UTC date (YYYY-MM-DD) of the most recent write.
	// When this changes, checkRotate triggers a rotation.
	lastDate string

	// nowFunc is an injectable clock for testing rotation without real time changes.
	nowFunc func() time.Time

	// alertChecker is called after each event write (outside the mutex) to
	// evaluate threshold rules. It is optional; nil means no alerting.
	alertChecker *AlertChecker
}

// NewEventLogger creates a new EventLogger that writes to a product-specific
// JSONL file in dataDir. If product is non-empty, writes go to
// metrics-{product}.jsonl; otherwise the legacy metrics.jsonl is used.
// It creates the data directory (0700) and file (0600) if they do not exist.
// On first use with a product, if the legacy metrics.jsonl exists and the
// product file does not, the legacy file is renamed (one-time migration).
func NewEventLogger(dataDir string, product string) (*EventLogger, error) {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	fileName := metricsFileNameForProduct(product)

	// One-time migration: rename legacy metrics.jsonl → metrics-{product}.jsonl
	// if product is set and the product file doesn't exist yet.
	if product != "" {
		legacyPath := filepath.Join(dataDir, metricsFileName)
		productPath := filepath.Join(dataDir, fileName)
		if _, err := os.Stat(productPath); os.IsNotExist(err) {
			if _, err := os.Stat(legacyPath); err == nil {
				_ = os.Rename(legacyPath, productPath)
			}
		}
	}

	path := filepath.Join(dataDir, fileName)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("open metrics file: %w", err)
	}

	return &EventLogger{
		file:     f,
		dataDir:  dataDir,
		product:  product,
		lastDate: time.Now().UTC().Format("2006-01-02"),
		nowFunc:  time.Now,
	}, nil
}

// SetAlertChecker attaches an AlertChecker that will be invoked after each
// event is written. The checker runs outside the EventLogger mutex to avoid
// recursive deadlock when it logs its own alert.threshold events.
func (l *EventLogger) SetAlertChecker(ac *AlertChecker) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.alertChecker = ac
}

// Log writes an event with the given type and data to the JSONL file.
// It creates the event with the current time, validates it, marshals to JSON,
// writes a single line, and flushes immediately.
// Before writing, it checks if the UTC date has changed and rotates if needed.
// After releasing the mutex it invokes the AlertChecker (if set) so that
// alerts can themselves call Log() without deadlocking. alert.threshold events
// are never re-checked to prevent infinite recursion.
func (l *EventLogger) Log(eventType string, data map[string]interface{}) error {
	now := l.nowFunc()

	// Inject product tag into event data if set.
	if l.product != "" {
		if data == nil {
			data = make(map[string]interface{})
		}
		data["product"] = l.product
	}

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

	// Check if the date has changed (rotation needed).
	if err := l.checkRotate(now); err != nil {
		// Log rotation failure to stderr but continue writing — losing
		// events is worse than failing to rotate.
		fmt.Fprintf(os.Stderr, "metrics: rotation failed: %v\n", err)
	}

	if _, err := l.file.Write(buf); err != nil {
		l.mu.Unlock()
		return fmt.Errorf("write event: %w", err)
	}
	if err := l.file.Sync(); err != nil {
		l.mu.Unlock()
		return fmt.Errorf("flush event: %w", err)
	}

	// Capture checker while still holding the lock, then release before calling
	// it so that alert.threshold events written by the checker can acquire the
	// lock without deadlocking.
	checker := l.alertChecker
	l.mu.Unlock()

	// Skip alert.threshold events to prevent infinite recursion.
	if checker != nil && eventType != EventAlertThreshold {
		checker.Check(eventType, data)
	}

	return nil
}

// checkRotate rotates the current metrics file to a dated variant
// if the UTC date has changed since the last write. The caller must hold l.mu.
// For product "chat": metrics-chat.jsonl → metrics-chat-2026-03-15.jsonl
// For legacy (no product): metrics.jsonl → metrics.2026-03-15.jsonl
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

	fileName := metricsFileNameForProduct(l.product)
	oldPath := filepath.Join(l.dataDir, fileName)

	var newPath string
	if l.product != "" {
		newPath = filepath.Join(l.dataDir, fmt.Sprintf("metrics-%s-%s.jsonl", l.product, previousDate))
	} else {
		newPath = filepath.Join(l.dataDir, fmt.Sprintf("metrics.%s.jsonl", previousDate))
	}

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

// Rotate renames the current metrics file with a date suffix and opens a
// fresh file for new writes.
func (l *EventLogger) Rotate() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Flush before rotating.
	if err := l.file.Sync(); err != nil {
		return fmt.Errorf("flush before rotate: %w", err)
	}

	fileName := metricsFileNameForProduct(l.product)
	oldPath := filepath.Join(l.dataDir, fileName)
	dateSuffix := l.nowFunc().Format("2006-01-02")

	var newPath string
	if l.product != "" {
		newPath = filepath.Join(l.dataDir, fmt.Sprintf("metrics-%s-%s.jsonl", l.product, dateSuffix))
	} else {
		newPath = filepath.Join(l.dataDir, fmt.Sprintf("metrics.%s.jsonl", dateSuffix))
	}

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
