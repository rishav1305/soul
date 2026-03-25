package runner

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/rishav1305/soul/internal/scout/store"
)

// PhaseFunc is a function that processes a pipeline phase.
// It receives the store and returns the number of leads processed and any error.
type PhaseFunc func(s *store.Store) (int, error)

// Runner polls the scout pipeline on a configurable interval,
// executing registered phase functions in sequence each cycle.
type Runner struct {
	store    *store.Store
	interval time.Duration
	phases   []namedPhase

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
	done    chan struct{}
	cycles  int
}

// namedPhase pairs a phase function with a name for logging.
type namedPhase struct {
	name string
	fn   PhaseFunc
}

// New creates a Runner with the given store and polling interval.
func New(s *store.Store, interval time.Duration) *Runner {
	return &Runner{
		store:    s,
		interval: interval,
	}
}

// Register adds a named phase function to the runner.
// Phases execute in registration order each cycle.
func (r *Runner) Register(name string, fn PhaseFunc) {
	r.phases = append(r.phases, namedPhase{name: name, fn: fn})
}

// Start begins the polling loop in a background goroutine.
// It runs one cycle immediately, then repeats at the configured interval.
// The loop stops when the context is cancelled or Stop is called.
func (r *Runner) Start(ctx context.Context) {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return
	}
	r.running = true
	r.cycles = 0
	r.done = make(chan struct{})
	ctx, r.cancel = context.WithCancel(ctx)
	r.mu.Unlock()

	go r.loop(ctx)
}

// Stop cancels the polling loop and waits for it to finish.
func (r *Runner) Stop() {
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return
	}
	r.cancel()
	done := r.done
	r.mu.Unlock()

	<-done
}

// Cycles returns the number of completed polling cycles.
func (r *Runner) Cycles() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cycles
}

func (r *Runner) loop(ctx context.Context) {
	defer func() {
		r.mu.Lock()
		r.running = false
		close(r.done)
		r.mu.Unlock()
	}()

	// Run one cycle immediately.
	r.runCycle()

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.runCycle()
		}
	}
}

func (r *Runner) runCycle() {
	for _, p := range r.phases {
		n, err := p.fn(r.store)
		if err != nil {
			log.Printf("runner: phase %s error: %v", p.name, err)
			continue
		}
		if n > 0 {
			log.Printf("runner: phase %s processed %d leads", p.name, n)
		}
	}

	r.mu.Lock()
	r.cycles++
	r.mu.Unlock()
}
