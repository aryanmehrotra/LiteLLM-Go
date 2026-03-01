package routing

import (
	"sync"
	"time"
)

// CooldownTracker tracks consecutive failures per provider and temporarily
// excludes providers that exceed the failure threshold.
// In-memory only — per-instance cooldown is correct for single-node deployments.
type CooldownTracker struct {
	mu             sync.RWMutex
	failures       map[string]int
	cooldownUntil  map[string]time.Time
	threshold      int
	cooldownPeriod time.Duration
}

// NewCooldownTracker creates a tracker with the given threshold and cooldown period.
func NewCooldownTracker(threshold int, cooldownPeriod time.Duration) *CooldownTracker {
	return &CooldownTracker{
		failures:       make(map[string]int),
		cooldownUntil:  make(map[string]time.Time),
		threshold:      threshold,
		cooldownPeriod: cooldownPeriod,
	}
}

// RecordFailure increments consecutive failure count for a provider.
// If the threshold is reached, the provider enters cooldown.
func (c *CooldownTracker) RecordFailure(provider string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.failures[provider]++

	if c.failures[provider] >= c.threshold {
		c.cooldownUntil[provider] = time.Now().Add(c.cooldownPeriod)
	}
}

// RecordSuccess resets consecutive failure count for a provider.
func (c *CooldownTracker) RecordSuccess(provider string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.failures[provider] = 0
	delete(c.cooldownUntil, provider)
}

// IsAvailable returns true if the provider is not in cooldown.
func (c *CooldownTracker) IsAvailable(provider string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	until, ok := c.cooldownUntil[provider]
	if !ok {
		return true
	}

	return time.Now().After(until)
}

// FailureCount returns the current consecutive failure count for a provider.
func (c *CooldownTracker) FailureCount(provider string) int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.failures[provider]
}
