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

func TestNewSampler_SetsInterval(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewEventLogger(dir, "")
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	defer logger.Close()

	interval := 5 * time.Second
	s := NewSampler(logger, interval)

	if s.interval != interval {
		t.Errorf("interval = %v, want %v", s.interval, interval)
	}
	if s.logger != logger {
		t.Error("logger not set correctly")
	}
	if s.stopCh == nil {
		t.Error("stopCh should be initialized")
	}
}

func TestSampler_Sample_CollectsMetrics(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewEventLogger(dir, "")
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	defer logger.Close()

	s := NewSampler(logger, time.Second)
	s.sample()

	// Read back the logged event.
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
		t.Fatalf("invalid JSON: %v", err)
	}

	if parsed["event"] != EventSystemSample {
		t.Errorf("event = %v, want %v", parsed["event"], EventSystemSample)
	}

	d, ok := parsed["data"].(map[string]interface{})
	if !ok {
		t.Fatal("data field missing or not an object")
	}

	goroutines, ok := d["goroutines"].(float64)
	if !ok || goroutines <= 0 {
		t.Errorf("goroutines should be > 0, got %v", d["goroutines"])
	}

	heapMB, ok := d["heap_mb"].(float64)
	if !ok || heapMB <= 0 {
		t.Errorf("heap_mb should be > 0, got %v", d["heap_mb"])
	}

	sysMB, ok := d["sys_mb"].(float64)
	if !ok || sysMB <= 0 {
		t.Errorf("sys_mb should be > 0, got %v", d["sys_mb"])
	}

	if _, ok := d["gc_pause_ns"]; !ok {
		t.Error("gc_pause_ns field missing")
	}

	if _, ok := d["num_gc"]; !ok {
		t.Error("num_gc field missing")
	}
}

func TestSampler_StartEmitsEvents(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewEventLogger(dir, "")
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	defer logger.Close()

	s := NewSampler(logger, 50*time.Millisecond)
	s.Start()

	// Wait long enough for several ticks.
	time.Sleep(280 * time.Millisecond)
	s.Stop()

	data, err := os.ReadFile(filepath.Join(dir, metricsFileName))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	// With 50ms interval and 280ms sleep, expect 4-6 events (first at 50ms, then 100, 150, 200, 250).
	// Allow some timing slack.
	if len(lines) < 3 {
		t.Errorf("expected at least 3 sample events, got %d", len(lines))
	}

	// Verify all lines are valid system.sample events.
	for i, line := range lines {
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			t.Errorf("line %d: invalid JSON: %v", i, err)
			continue
		}
		if parsed["event"] != EventSystemSample {
			t.Errorf("line %d: event = %v, want %v", i, parsed["event"], EventSystemSample)
		}
	}
}

func TestSampler_StopHaltsAndWaits(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewEventLogger(dir, "")
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	defer logger.Close()

	s := NewSampler(logger, 50*time.Millisecond)
	s.Start()
	time.Sleep(120 * time.Millisecond)
	s.Stop()

	// After Stop, count current events.
	data1, err := os.ReadFile(filepath.Join(dir, metricsFileName))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	countBefore := len(strings.Split(strings.TrimSpace(string(data1)), "\n"))

	// Wait and verify no more events are written.
	time.Sleep(150 * time.Millisecond)

	data2, err := os.ReadFile(filepath.Join(dir, metricsFileName))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	countAfter := len(strings.Split(strings.TrimSpace(string(data2)), "\n"))

	if countAfter != countBefore {
		t.Errorf("events after Stop: before=%d, after=%d — sampling should have stopped", countBefore, countAfter)
	}
}

func TestSampler_StopMultipleCalls(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewEventLogger(dir, "")
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	defer logger.Close()

	s := NewSampler(logger, 50*time.Millisecond)
	s.Start()
	time.Sleep(80 * time.Millisecond)

	// Multiple Stop calls should not panic.
	s.Stop()
	s.Stop()
	s.Stop()
}

func TestSampler_RespectsInterval(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewEventLogger(dir, "")
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	defer logger.Close()

	interval := 100 * time.Millisecond
	s := NewSampler(logger, interval)
	s.Start()

	// Run for ~350ms. With 100ms interval, expect 3 ticks (at 100, 200, 300ms).
	time.Sleep(350 * time.Millisecond)
	s.Stop()

	data, err := os.ReadFile(filepath.Join(dir, metricsFileName))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	// Expect 3 events with some timing tolerance (2-4 acceptable).
	if len(lines) < 2 || len(lines) > 5 {
		t.Errorf("expected 2-5 events for 350ms at 100ms interval, got %d", len(lines))
	}
}

func TestSampler_ConcurrentStartStop(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewEventLogger(dir, "")
	if err != nil {
		t.Fatalf("NewEventLogger: %v", err)
	}
	defer logger.Close()

	// Run multiple concurrent Start/Stop cycles. This must not panic or deadlock.
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := NewSampler(logger, 50*time.Millisecond)
			s.Start()
			time.Sleep(60 * time.Millisecond)
			s.Stop()
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent Start/Stop test timed out — possible deadlock")
	}
}
