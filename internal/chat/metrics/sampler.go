package metrics

import (
	"runtime"
	"sync"
	"time"
)

// Sampler periodically collects system runtime metrics and logs them
// via an EventLogger as system.sample events.
type Sampler struct {
	logger   *EventLogger
	interval time.Duration
	stopCh   chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex
	running  bool
}

// NewSampler creates a Sampler that will emit system.sample events at the
// given interval using the provided EventLogger.
func NewSampler(logger *EventLogger, interval time.Duration) *Sampler {
	return &Sampler{
		logger:   logger,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins periodic sampling in a background goroutine. It is safe to
// call Start only once; subsequent calls before Stop are no-ops.
func (s *Sampler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	s.wg.Add(1)
	go s.loop()
}

// Stop signals the sampling goroutine to exit and blocks until it has
// finished. Stop is safe to call multiple times; only the first call
// performs the shutdown.
func (s *Sampler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.mu.Unlock()

	close(s.stopCh)
	s.wg.Wait()
}

// loop runs in a goroutine, sampling at each tick until stopCh is closed.
func (s *Sampler) loop() {
	defer s.wg.Done()
	defer func() {
		// Recover from any panic (e.g. runtime.ReadMemStats) so the
		// server keeps running.
		_ = recover()
	}()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.sample()
		}
	}
}

// sample collects current runtime metrics and logs a system.sample event.
func (s *Sampler) sample() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	data := map[string]interface{}{
		"goroutines":  runtime.NumGoroutine(),
		"heap_mb":     float64(memStats.HeapAlloc) / 1024 / 1024,
		"sys_mb":      float64(memStats.Sys) / 1024 / 1024,
		"gc_pause_ns": memStats.PauseNs[(memStats.NumGC+255)%256],
		"num_gc":      memStats.NumGC,
	}

	// Best-effort logging — don't propagate errors from the background sampler.
	_ = s.logger.Log(EventSystemSample, data)
}
