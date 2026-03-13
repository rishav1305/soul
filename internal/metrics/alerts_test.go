package metrics

import (
	"os"
	"testing"
)

// makeTestLogger creates a temporary EventLogger for use in tests.
func makeTestLogger(t *testing.T) (*EventLogger, string) {
	t.Helper()
	dir := t.TempDir()
	logger, err := NewEventLogger(dir)
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	t.Cleanup(func() { logger.Close() })
	return logger, dir
}

// TestAlertChecker_TriggersOnBreach verifies that when a field value exceeds
// the configured threshold, an alert.threshold event is written to the log.
func TestAlertChecker_TriggersOnBreach(t *testing.T) {
	logger, dir := makeTestLogger(t)

	ac := NewAlertChecker(logger)
	ac.AddThreshold(Threshold{
		Metric:   EventDBQuery,
		Field:    "duration_ms",
		MaxValue: 100,
		Severity: "warning",
	})

	// Value exceeds threshold — should log an alert.
	ac.Check(EventDBQuery, map[string]interface{}{
		"duration_ms": float64(250),
		"path":        "/api/sessions",
	})

	// Read back the events and look for an alert.threshold entry.
	events, err := ReadEvents(dir)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}

	var found bool
	for _, ev := range events {
		if ev.EventType != EventAlertThreshold {
			continue
		}
		found = true
		if v, ok := ev.Data["metric"]; !ok || v != EventDBQuery {
			t.Errorf("alert metric = %v, want %s", v, EventDBQuery)
		}
		if v, ok := ev.Data["field"]; !ok || v != "duration_ms" {
			t.Errorf("alert field = %v, want duration_ms", v)
		}
		if v, ok := ev.Data["severity"]; !ok || v != "warning" {
			t.Errorf("alert severity = %v, want warning", v)
		}
		if v, ok := ev.Data["value"]; !ok || v.(float64) != 250 {
			t.Errorf("alert value = %v, want 250", v)
		}
		if v, ok := ev.Data["threshold"]; !ok || v.(float64) != 100 {
			t.Errorf("alert threshold = %v, want 100", v)
		}
		// context field "path" should be propagated
		if v, ok := ev.Data["path"]; !ok || v != "/api/sessions" {
			t.Errorf("alert path = %v, want /api/sessions", v)
		}
	}

	if !found {
		t.Error("expected alert.threshold event to be logged, but none found")
	}
}

// TestAlertChecker_NoAlertBelowThreshold verifies that no alert event is
// written when a field value is at or below the configured threshold.
func TestAlertChecker_NoAlertBelowThreshold(t *testing.T) {
	logger, dir := makeTestLogger(t)

	ac := NewAlertChecker(logger)
	ac.AddThreshold(Threshold{
		Metric:   EventDBQuery,
		Field:    "duration_ms",
		MaxValue: 100,
		Severity: "warning",
	})

	// Exactly at threshold — should NOT trigger.
	ac.Check(EventDBQuery, map[string]interface{}{
		"duration_ms": float64(100),
	})

	// Below threshold — should NOT trigger.
	ac.Check(EventDBQuery, map[string]interface{}{
		"duration_ms": float64(50),
	})

	events, err := ReadEvents(dir)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}

	for _, ev := range events {
		if ev.EventType == EventAlertThreshold {
			t.Errorf("unexpected alert.threshold event logged for value at/below threshold: %+v", ev.Data)
		}
	}
}

// TestDefaultThresholds verifies that NewAlertCheckerWithDefaults returns a
// checker with a non-empty set of pre-configured thresholds.
func TestDefaultThresholds(t *testing.T) {
	// We need a real logger — use /dev/null as data dir stand-in via TempDir.
	dir := t.TempDir()
	logger, err := NewEventLogger(dir)
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	defer logger.Close()

	ac := NewAlertCheckerWithDefaults(logger)
	if len(ac.thresholds) == 0 {
		t.Error("NewAlertCheckerWithDefaults returned checker with no thresholds")
	}

	// Spot-check: at least one threshold for each major event type.
	checks := map[string]bool{
		EventDBQuery:      false,
		EventAPIRequest:   false,
		EventWSStreamEnd:  false,
		EventSystemSample: false,
	}
	for _, th := range ac.thresholds {
		if _, ok := checks[th.Metric]; ok {
			checks[th.Metric] = true
		}
	}
	for metric, found := range checks {
		if !found {
			t.Errorf("no default threshold configured for metric %q", metric)
		}
	}

	// Ensure /dev/null write didn't leave stray files we care about.
	_ = os.RemoveAll(dir)
}
