package routing

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInFlightTracker_Increment(t *testing.T) {
	tests := []struct {
		name       string
		increments int
		wantCount  int64
	}{
		{name: "single increment", increments: 1, wantCount: 1},
		{name: "multiple increments", increments: 5, wantCount: 5},
		{name: "many increments", increments: 100, wantCount: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewInFlightTracker()

			for range tt.increments {
				tracker.Increment("provider-a")
			}

			assert.Equal(t, tt.wantCount, tracker.Count("provider-a"))
		})
	}
}

func TestInFlightTracker_Decrement(t *testing.T) {
	tests := []struct {
		name       string
		increments int
		decrements int
		wantCount  int64
	}{
		{name: "decrement to zero", increments: 3, decrements: 3, wantCount: 0},
		{name: "partial decrement", increments: 5, decrements: 2, wantCount: 3},
		{name: "decrement below zero clamps to zero", increments: 1, decrements: 3, wantCount: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewInFlightTracker()

			for range tt.increments {
				tracker.Increment("p1")
			}

			for range tt.decrements {
				tracker.Decrement("p1")
			}

			assert.Equal(t, tt.wantCount, tracker.Count("p1"))
		})
	}
}

func TestInFlightTracker_Count(t *testing.T) {
	t.Run("returns zero for unknown ID", func(t *testing.T) {
		tracker := NewInFlightTracker()
		assert.Equal(t, int64(0), tracker.Count("unknown"))
	})

	t.Run("independent tracking per ID", func(t *testing.T) {
		tracker := NewInFlightTracker()

		tracker.Increment("a")
		tracker.Increment("a")
		tracker.Increment("b")

		assert.Equal(t, int64(2), tracker.Count("a"))
		assert.Equal(t, int64(1), tracker.Count("b"))
		assert.Equal(t, int64(0), tracker.Count("c"))
	})
}

func TestInFlightTracker_DecrementOnEmpty(t *testing.T) {
	tracker := NewInFlightTracker()

	// Decrement without any increments should clamp to 0
	tracker.Decrement("empty")
	assert.Equal(t, int64(0), tracker.Count("empty"))
}

func TestNewInFlightTracker(t *testing.T) {
	tracker := NewInFlightTracker()
	assert.NotNil(t, tracker)
	assert.NotNil(t, tracker.counts)
}
