package routing

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLatencyTracker_Record(t *testing.T) {
	t.Run("first recording sets latency directly", func(t *testing.T) {
		tracker := NewLatencyTracker(0.5)
		tracker.Record("p1", 100*time.Millisecond)

		assert.Equal(t, 100.0, tracker.Latency("p1"))
	})

	t.Run("EMA calculation with alpha 0.5", func(t *testing.T) {
		tracker := NewLatencyTracker(0.5)

		// First recording: EMA = 100
		tracker.Record("p1", 100*time.Millisecond)
		assert.Equal(t, 100.0, tracker.Latency("p1"))

		// Second recording: EMA = 0.5*200 + 0.5*100 = 150
		tracker.Record("p1", 200*time.Millisecond)
		assert.Equal(t, 150.0, tracker.Latency("p1"))

		// Third recording: EMA = 0.5*300 + 0.5*150 = 225
		tracker.Record("p1", 300*time.Millisecond)
		assert.Equal(t, 225.0, tracker.Latency("p1"))
	})

	t.Run("EMA calculation with alpha 1.0 tracks latest exactly", func(t *testing.T) {
		tracker := NewLatencyTracker(1.0)

		tracker.Record("p1", 100*time.Millisecond)
		assert.Equal(t, 100.0, tracker.Latency("p1"))

		tracker.Record("p1", 500*time.Millisecond)
		assert.Equal(t, 500.0, tracker.Latency("p1"))
	})

	t.Run("EMA with small alpha is more stable", func(t *testing.T) {
		tracker := NewLatencyTracker(0.1)

		tracker.Record("p1", 100*time.Millisecond)
		assert.Equal(t, 100.0, tracker.Latency("p1"))

		// A spike should be smoothed: EMA = 0.1*1000 + 0.9*100 = 190
		tracker.Record("p1", 1000*time.Millisecond)
		assert.InDelta(t, 190.0, tracker.Latency("p1"), 0.01)
	})

	t.Run("independent tracking per ID", func(t *testing.T) {
		tracker := NewLatencyTracker(0.5)

		tracker.Record("a", 100*time.Millisecond)
		tracker.Record("b", 200*time.Millisecond)

		assert.Equal(t, 100.0, tracker.Latency("a"))
		assert.Equal(t, 200.0, tracker.Latency("b"))
	})
}

func TestLatencyTracker_Latency(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want float64
	}{
		{
			name: "returns zero for unknown ID",
			id:   "unknown",
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewLatencyTracker(0.2)
			assert.Equal(t, tt.want, tracker.Latency(tt.id))
		})
	}
}

func TestNewLatencyTracker(t *testing.T) {
	tests := []struct {
		name      string
		alpha     float64
		wantAlpha float64
	}{
		{name: "valid alpha 0.2", alpha: 0.2, wantAlpha: 0.2},
		{name: "valid alpha 1.0", alpha: 1.0, wantAlpha: 1.0},
		{name: "zero alpha defaults to 0.2", alpha: 0, wantAlpha: 0.2},
		{name: "negative alpha defaults to 0.2", alpha: -1, wantAlpha: 0.2},
		{name: "alpha greater than 1 defaults to 0.2", alpha: 1.5, wantAlpha: 0.2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewLatencyTracker(tt.alpha)
			assert.NotNil(t, tracker)
			assert.NotNil(t, tracker.latency)
			assert.True(t, math.Abs(tt.wantAlpha-tracker.alpha) < 0.001)
		})
	}
}

func TestLatencyTracker_EMAConvergence(t *testing.T) {
	// Verify EMA converges toward a constant input
	tracker := NewLatencyTracker(0.3)

	// Start at 100ms, then keep recording 200ms
	tracker.Record("p1", 100*time.Millisecond)

	for range 50 {
		tracker.Record("p1", 200*time.Millisecond)
	}

	// After many 200ms recordings, EMA should converge close to 200
	assert.InDelta(t, 200.0, tracker.Latency("p1"), 1.0)
}
