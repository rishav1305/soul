package sweep

import (
	"log"
	"sync"
	"time"

	"github.com/rishav1305/soul-v2/internal/scout/store"
)

// Scheduler runs TheirStack sweeps on a configurable interval.
type Scheduler struct {
	interval   time.Duration
	config     *SweepConfig
	store      *store.Store
	scorer     Scorer
	client     *TheirStackClient
	stopCh     chan struct{}
	mu         sync.Mutex
	running    bool
	lastRun    time.Time
	lastResult *SweepResult
	runCounter int64
}

// NewScheduler creates a new sweep scheduler.
// Enforces minimum 1-hour interval to prevent ticker panics and API abuse.
func NewScheduler(cfg *SweepConfig, st *store.Store, scorer Scorer, client *TheirStackClient) *Scheduler {
	interval := time.Duration(cfg.IntervalHours) * time.Hour
	if interval <= 0 {
		interval = 24 * time.Hour
		log.Printf("scout: invalid interval_hours %d, defaulting to 24h", cfg.IntervalHours)
	}
	return &Scheduler{
		interval: interval,
		config:   cfg,
		store:    st,
		scorer:   scorer,
		client:   client,
		stopCh:   make(chan struct{}),
	}
}

// Start launches the scheduler goroutine. It runs a sweep immediately, then on interval.
func (s *Scheduler) Start() {
	go s.loop()
}

// Stop signals the scheduler goroutine to exit.
func (s *Scheduler) Stop() {
	close(s.stopCh)
}

func (s *Scheduler) loop() {
	s.runSweep()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.runSweep()
		case <-s.stopCh:
			return
		}
	}
}

func (s *Scheduler) runSweep() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	log.Println("scout: starting sweep...")
	result, err := RunSweep(s.client, s.store, s.config, s.scorer)
	if err != nil {
		log.Printf("scout: sweep error: %v", err)
		return
	}

	s.mu.Lock()
	s.lastRun = time.Now()
	s.lastResult = result
	s.mu.Unlock()

	log.Printf("scout: sweep complete — %d new, %d dupes, %d scored, %d high matches",
		result.NewLeads, result.Duplicates, result.Scored, result.HighMatches)
}

// RunNow triggers an immediate sweep in a background goroutine.
// Returns a run ID and false if a sweep is already in progress.
func (s *Scheduler) RunNow() (runID int64, started bool) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return 0, false
	}
	s.runCounter++
	id := s.runCounter
	s.mu.Unlock()

	go s.runSweep()
	return id, true
}

// Status returns the current scheduler state.
func (s *Scheduler) Status() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := map[string]interface{}{
		"running":  s.running,
		"interval": s.interval.String(),
		"last_run": "",
		"next_run": "",
	}
	if !s.lastRun.IsZero() {
		status["last_run"] = s.lastRun.Format(time.RFC3339)
		status["next_run"] = s.lastRun.Add(s.interval).Format(time.RFC3339)
	}
	return status
}
