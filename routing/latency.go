package routing

import (
	"sync"
	"time"
)

// LatencyTracker tracks exponential moving average (EMA) latency per provider/deployment.
type LatencyTracker struct {
	mu      sync.RWMutex
	latency map[string]float64
	alpha   float64
}

// NewLatencyTracker creates a new latency tracker with the given EMA alpha (0-1).
// Higher alpha = more weight on recent observations.
func NewLatencyTracker(alpha float64) *LatencyTracker {
	if alpha <= 0 || alpha > 1 {
		alpha = 0.2
	}

	return &LatencyTracker{
		latency: make(map[string]float64),
		alpha:   alpha,
	}
}

// Record updates the EMA latency for the given ID.
func (t *LatencyTracker) Record(id string, duration time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	ms := float64(duration.Milliseconds())

	prev, exists := t.latency[id]
	if !exists {
		t.latency[id] = ms
		return
	}

	t.latency[id] = t.alpha*ms + (1-t.alpha)*prev
}

// Latency returns the current EMA latency in milliseconds for the given ID.
// Returns 0 if no data exists.
func (t *LatencyTracker) Latency(id string) float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.latency[id]
}
