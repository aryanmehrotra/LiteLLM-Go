package routing

import "sync"

// InFlightTracker counts in-flight requests per provider/deployment.
type InFlightTracker struct {
	mu     sync.RWMutex
	counts map[string]int64
}

// NewInFlightTracker creates a new in-flight request tracker.
func NewInFlightTracker() *InFlightTracker {
	return &InFlightTracker{counts: make(map[string]int64)}
}

// Increment adds one in-flight request for the given ID.
func (t *InFlightTracker) Increment(id string) {
	t.mu.Lock()
	t.counts[id]++
	t.mu.Unlock()
}

// Decrement removes one in-flight request for the given ID.
func (t *InFlightTracker) Decrement(id string) {
	t.mu.Lock()
	t.counts[id]--
	if t.counts[id] < 0 {
		t.counts[id] = 0
	}
	t.mu.Unlock()
}

// Count returns the current in-flight request count for the given ID.
func (t *InFlightTracker) Count(id string) int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.counts[id]
}
