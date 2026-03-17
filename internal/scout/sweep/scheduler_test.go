package sweep

import (
	"net/http"
	"testing"
)

func TestScheduler_RunNowRejectsWhileRunning(t *testing.T) {
	st := newTestSweepStore(t)
	transport := &mockTransport{status: 200, response: emptyJobsResponse}
	client := NewTheirStackClient("key", &http.Client{Transport: transport})
	cfg := DefaultConfig()
	cfg.IntervalHours = 999

	sched := NewScheduler(cfg, st, nil, client)

	sched.mu.Lock()
	sched.running = true
	sched.mu.Unlock()

	_, started := sched.RunNow()
	if started {
		t.Error("RunNow should return false while already running")
	}
}

func TestScheduler_StatusEmptyBeforeFirstRun(t *testing.T) {
	st := newTestSweepStore(t)
	transport := &mockTransport{status: 200, response: emptyJobsResponse}
	client := NewTheirStackClient("key", &http.Client{Transport: transport})
	cfg := DefaultConfig()

	sched := NewScheduler(cfg, st, nil, client)
	status := sched.Status()

	if status["last_run"] != "" {
		t.Errorf("last_run = %q, want empty", status["last_run"])
	}
	if status["next_run"] != "" {
		t.Errorf("next_run = %q, want empty", status["next_run"])
	}
}
