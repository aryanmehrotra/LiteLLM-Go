package routing

import (
	"sync"
	"time"
)

// UsageTracker tracks windowed token usage per provider/deployment.
type UsageTracker struct {
	mu          sync.RWMutex
	usage       map[string]int64
	windowStart map[string]time.Time
	resetPeriod time.Duration
}

// NewUsageTracker creates a new usage tracker with the given reset period.
func NewUsageTracker(resetPeriod time.Duration) *UsageTracker {
	if resetPeriod <= 0 {
		resetPeriod = 60 * time.Second
	}

	return &UsageTracker{
		usage:       make(map[string]int64),
		windowStart: make(map[string]time.Time),
		resetPeriod: resetPeriod,
	}
}

// Record adds token usage for the given ID. Resets the counter if the window has expired.
func (t *UsageTracker) Record(id string, tokens int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()

	start, exists := t.windowStart[id]
	if !exists || now.Sub(start) > t.resetPeriod {
		t.usage[id] = 0
		t.windowStart[id] = now
	}

	t.usage[id] += int64(tokens)
}

// Usage returns the current windowed token count for the given ID.
func (t *UsageTracker) Usage(id string) int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	start, exists := t.windowStart[id]
	if !exists || time.Since(start) > t.resetPeriod {
		return 0
	}

	return t.usage[id]
}
