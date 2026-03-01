package routing

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCooldownTracker_IsAvailable(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		want     bool
	}{
		{
			name:     "returns true for unknown provider",
			provider: "openai",
			want:     true,
		},
		{
			name:     "returns true for different provider",
			provider: "anthropic",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewCooldownTracker(3, 10*time.Second)
			assert.Equal(t, tt.want, tracker.IsAvailable(tt.provider))
		})
	}
}

func TestCooldownTracker_RecordFailure(t *testing.T) {
	tests := []struct {
		name          string
		threshold     int
		failures      int
		wantCount     int
		wantAvailable bool
	}{
		{
			name:          "single failure below threshold",
			threshold:     3,
			failures:      1,
			wantCount:     1,
			wantAvailable: true,
		},
		{
			name:          "failures at threshold minus one",
			threshold:     3,
			failures:      2,
			wantCount:     2,
			wantAvailable: true,
		},
		{
			name:          "failures at threshold enters cooldown",
			threshold:     3,
			failures:      3,
			wantCount:     3,
			wantAvailable: false,
		},
		{
			name:          "failures beyond threshold stays in cooldown",
			threshold:     3,
			failures:      5,
			wantCount:     5,
			wantAvailable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewCooldownTracker(tt.threshold, 10*time.Second)

			for range tt.failures {
				tracker.RecordFailure("provider-a")
			}

			assert.Equal(t, tt.wantCount, tracker.FailureCount("provider-a"))
			assert.Equal(t, tt.wantAvailable, tracker.IsAvailable("provider-a"))
		})
	}
}

func TestCooldownTracker_RecordSuccess(t *testing.T) {
	t.Run("resets failure count and removes cooldown", func(t *testing.T) {
		tracker := NewCooldownTracker(2, 10*time.Second)

		// Build up failures to enter cooldown
		tracker.RecordFailure("p1")
		tracker.RecordFailure("p1")
		assert.False(t, tracker.IsAvailable("p1"))
		assert.Equal(t, 2, tracker.FailureCount("p1"))

		// Record success resets everything
		tracker.RecordSuccess("p1")
		assert.True(t, tracker.IsAvailable("p1"))
		assert.Equal(t, 0, tracker.FailureCount("p1"))
	})

	t.Run("success on never-failed provider is safe", func(t *testing.T) {
		tracker := NewCooldownTracker(3, 10*time.Second)
		tracker.RecordSuccess("unknown")
		assert.True(t, tracker.IsAvailable("unknown"))
		assert.Equal(t, 0, tracker.FailureCount("unknown"))
	})
}

func TestCooldownTracker_ExitsCooldownAfterPeriod(t *testing.T) {
	// Use a very short cooldown period so the test runs fast
	cooldownPeriod := 50 * time.Millisecond
	tracker := NewCooldownTracker(1, cooldownPeriod)

	// One failure with threshold=1 enters cooldown immediately
	tracker.RecordFailure("p1")
	assert.False(t, tracker.IsAvailable("p1"))

	// Wait for cooldown to expire
	time.Sleep(cooldownPeriod + 10*time.Millisecond)
	assert.True(t, tracker.IsAvailable("p1"))
}

func TestCooldownTracker_IndependentProviders(t *testing.T) {
	tracker := NewCooldownTracker(2, 10*time.Second)

	// Put provider A in cooldown
	tracker.RecordFailure("a")
	tracker.RecordFailure("a")
	assert.False(t, tracker.IsAvailable("a"))

	// Provider B should still be available
	assert.True(t, tracker.IsAvailable("b"))
	assert.Equal(t, 0, tracker.FailureCount("b"))
}

func TestNewCooldownTracker(t *testing.T) {
	tracker := NewCooldownTracker(5, 30*time.Second)

	assert.NotNil(t, tracker)
	assert.NotNil(t, tracker.failures)
	assert.NotNil(t, tracker.cooldownUntil)
	assert.Equal(t, 5, tracker.threshold)
	assert.Equal(t, 30*time.Second, tracker.cooldownPeriod)
}
