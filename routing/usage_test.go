package routing

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUsageTracker_Record(t *testing.T) {
	t.Run("accumulates tokens within window", func(t *testing.T) {
		tracker := NewUsageTracker(10 * time.Second)

		tracker.Record("p1", 100)
		assert.Equal(t, int64(100), tracker.Usage("p1"))

		tracker.Record("p1", 200)
		assert.Equal(t, int64(300), tracker.Usage("p1"))

		tracker.Record("p1", 50)
		assert.Equal(t, int64(350), tracker.Usage("p1"))
	})

	t.Run("independent tracking per ID", func(t *testing.T) {
		tracker := NewUsageTracker(10 * time.Second)

		tracker.Record("a", 100)
		tracker.Record("b", 200)

		assert.Equal(t, int64(100), tracker.Usage("a"))
		assert.Equal(t, int64(200), tracker.Usage("b"))
	})
}

func TestUsageTracker_Usage(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want int64
	}{
		{
			name: "returns zero for unknown ID",
			id:   "unknown",
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewUsageTracker(10 * time.Second)
			assert.Equal(t, tt.want, tracker.Usage(tt.id))
		})
	}
}

func TestUsageTracker_WindowReset(t *testing.T) {
	// Use a very short reset period
	resetPeriod := 50 * time.Millisecond
	tracker := NewUsageTracker(resetPeriod)

	// Record some usage
	tracker.Record("p1", 500)
	assert.Equal(t, int64(500), tracker.Usage("p1"))

	// Wait for the window to expire
	time.Sleep(resetPeriod + 10*time.Millisecond)

	// Usage should return 0 after window expires
	assert.Equal(t, int64(0), tracker.Usage("p1"))
}

func TestUsageTracker_WindowResetOnRecord(t *testing.T) {
	// Use a very short reset period
	resetPeriod := 50 * time.Millisecond
	tracker := NewUsageTracker(resetPeriod)

	// Record initial usage
	tracker.Record("p1", 1000)
	assert.Equal(t, int64(1000), tracker.Usage("p1"))

	// Wait for the window to expire
	time.Sleep(resetPeriod + 10*time.Millisecond)

	// Record new usage — should reset counter and start fresh
	tracker.Record("p1", 200)
	assert.Equal(t, int64(200), tracker.Usage("p1"))
}

func TestNewUsageTracker(t *testing.T) {
	tests := []struct {
		name            string
		resetPeriod     time.Duration
		wantResetPeriod time.Duration
	}{
		{
			name:            "valid reset period",
			resetPeriod:     30 * time.Second,
			wantResetPeriod: 30 * time.Second,
		},
		{
			name:            "zero defaults to 60s",
			resetPeriod:     0,
			wantResetPeriod: 60 * time.Second,
		},
		{
			name:            "negative defaults to 60s",
			resetPeriod:     -1 * time.Second,
			wantResetPeriod: 60 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewUsageTracker(tt.resetPeriod)
			assert.NotNil(t, tracker)
			assert.NotNil(t, tracker.usage)
			assert.NotNil(t, tracker.windowStart)
			assert.Equal(t, tt.wantResetPeriod, tracker.resetPeriod)
		})
	}
}

func TestUsageTracker_MultipleProviders(t *testing.T) {
	tracker := NewUsageTracker(10 * time.Second)

	tracker.Record("openai", 1000)
	tracker.Record("anthropic", 500)
	tracker.Record("openai", 2000)
	tracker.Record("groq", 100)

	assert.Equal(t, int64(3000), tracker.Usage("openai"))
	assert.Equal(t, int64(500), tracker.Usage("anthropic"))
	assert.Equal(t, int64(100), tracker.Usage("groq"))
	assert.Equal(t, int64(0), tracker.Usage("deepseek"))
}
