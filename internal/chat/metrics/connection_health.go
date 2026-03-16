package metrics

import (
	"math"
	"sort"
	"sync"
	"time"
)

// ConnectionHealth tracks sliding-window connection health metrics.
type ConnectionHealth struct {
	mu     sync.Mutex
	window time.Duration

	connects    []time.Time
	disconnects []disconnectRecord
	reconnects  []reconnectRecord
	latencies   []latencyRecord
}

type disconnectRecord struct {
	at     time.Time
	reason string
}

type reconnectRecord struct {
	at      time.Time
	success bool
}

type latencyRecord struct {
	at      time.Time
	latency time.Duration
}

func NewConnectionHealth(window time.Duration) *ConnectionHealth {
	return &ConnectionHealth{window: window}
}

func (ch *ConnectionHealth) RecordConnect() {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.connects = append(ch.connects, time.Now())
}

func (ch *ConnectionHealth) RecordDisconnect(reason string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.disconnects = append(ch.disconnects, disconnectRecord{at: time.Now(), reason: reason})
}

func (ch *ConnectionHealth) RecordReconnectAttempt(success bool) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.reconnects = append(ch.reconnects, reconnectRecord{at: time.Now(), success: success})
}

func (ch *ConnectionHealth) RecordReconnectLatency(d time.Duration) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.latencies = append(ch.latencies, latencyRecord{at: time.Now(), latency: d})
}

// DropRate returns the fraction of connections that ended abnormally within the window.
func (ch *ConnectionHealth) DropRate() float64 {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	cutoff := time.Now().Add(-ch.window)

	total := 0
	for _, c := range ch.connects {
		if !c.Before(cutoff) {
			total++
		}
	}
	if total == 0 {
		return 0
	}

	abnormal := 0
	for _, d := range ch.disconnects {
		if d.at.Before(cutoff) {
			continue
		}
		if d.reason != "normal" && d.reason != "client_nav" {
			abnormal++
		}
	}
	return float64(abnormal) / float64(total)
}

// ReconnectSuccessRate returns the fraction of reconnect attempts that succeeded.
func (ch *ConnectionHealth) ReconnectSuccessRate() float64 {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	cutoff := time.Now().Add(-ch.window)

	var total, success int
	for _, r := range ch.reconnects {
		if r.at.Before(cutoff) {
			continue
		}
		total++
		if r.success {
			success++
		}
	}
	if total == 0 {
		return 1.0
	}
	return float64(success) / float64(total)
}

// ReconnectP50 returns the median reconnect latency within the window.
func (ch *ConnectionHealth) ReconnectP50() time.Duration {
	return ch.reconnectPercentile(0.50)
}

// ReconnectP95 returns the 95th percentile reconnect latency.
func (ch *ConnectionHealth) ReconnectP95() time.Duration {
	return ch.reconnectPercentile(0.95)
}

func (ch *ConnectionHealth) reconnectPercentile(p float64) time.Duration {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	cutoff := time.Now().Add(-ch.window)

	var durations []time.Duration
	for _, l := range ch.latencies {
		if l.at.Before(cutoff) {
			continue
		}
		durations = append(durations, l.latency)
	}
	if len(durations) == 0 {
		return 0
	}

	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	idx := int(math.Ceil(float64(len(durations))*p)) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(durations) {
		idx = len(durations) - 1
	}
	return durations[idx]
}

// Prune removes records older than 2x the window to bound memory.
func (ch *ConnectionHealth) Prune() {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	cutoff := time.Now().Add(-2 * ch.window)

	pruned := ch.connects[:0]
	for _, c := range ch.connects {
		if !c.Before(cutoff) {
			pruned = append(pruned, c)
		}
	}
	ch.connects = pruned

	prunedD := ch.disconnects[:0]
	for _, d := range ch.disconnects {
		if !d.at.Before(cutoff) {
			prunedD = append(prunedD, d)
		}
	}
	ch.disconnects = prunedD

	prunedR := ch.reconnects[:0]
	for _, r := range ch.reconnects {
		if !r.at.Before(cutoff) {
			prunedR = append(prunedR, r)
		}
	}
	ch.reconnects = prunedR

	prunedL := ch.latencies[:0]
	for _, l := range ch.latencies {
		if !l.at.Before(cutoff) {
			prunedL = append(prunedL, l)
		}
	}
	ch.latencies = prunedL
}
