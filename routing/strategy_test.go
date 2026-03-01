package routing

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func testDeployments(ids ...string) []Deployment {
	deps := make([]Deployment, len(ids))
	for i, id := range ids {
		deps[i] = Deployment{ID: id}
	}

	return deps
}

func TestSimpleStrategy_Select(t *testing.T) {
	s := &SimpleStrategy{}

	tests := []struct {
		name        string
		deployments []Deployment
		wantNil     bool
		wantID      string
	}{
		{
			name:        "empty list returns nil",
			deployments: nil,
			wantNil:     true,
		},
		{
			name:        "single deployment returns it",
			deployments: testDeployments("a"),
			wantID:      "a",
		},
		{
			name:        "multiple deployments returns first",
			deployments: testDeployments("a", "b", "c"),
			wantID:      "a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.Select(tt.deployments, "model")

			if tt.wantNil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				assert.Equal(t, tt.wantID, got.ID)
			}
		})
	}
}

func TestRoundRobinStrategy_Select(t *testing.T) {
	tests := []struct {
		name        string
		deployments []Deployment
		calls       int
		wantIDs     []string
	}{
		{
			name:        "empty list returns nil",
			deployments: nil,
			calls:       1,
			wantIDs:     []string{""},
		},
		{
			name:        "rotates through two deployments",
			deployments: testDeployments("a", "b"),
			calls:       4,
			wantIDs:     []string{"a", "b", "a", "b"},
		},
		{
			name:        "rotates through three deployments",
			deployments: testDeployments("x", "y", "z"),
			calls:       6,
			wantIDs:     []string{"x", "y", "z", "x", "y", "z"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &RoundRobinStrategy{}

			for i := range tt.calls {
				got := s.Select(tt.deployments, "model")

				if tt.wantIDs[i] == "" {
					assert.Nil(t, got)
				} else {
					assert.NotNil(t, got)
					assert.Equal(t, tt.wantIDs[i], got.ID)
				}
			}
		})
	}
}

func TestWeightedStrategy_Select(t *testing.T) {
	t.Run("empty list returns nil", func(t *testing.T) {
		s := &WeightedStrategy{Weights: map[string]int{}}
		got := s.Select(nil, "model")
		assert.Nil(t, got)
	})

	t.Run("respects deployment weight field", func(t *testing.T) {
		deployments := []Deployment{
			{ID: "heavy", Weight: 3},
			{ID: "light", Weight: 1},
		}
		s := &WeightedStrategy{Weights: map[string]int{}}

		// With weights 3:1, slots are [heavy, heavy, heavy, light]
		// 4 calls should cycle through all slots
		counts := map[string]int{}

		for range 4 {
			got := s.Select(deployments, "model")
			assert.NotNil(t, got)
			counts[got.ID]++
		}

		assert.Equal(t, 3, counts["heavy"])
		assert.Equal(t, 1, counts["light"])
	})

	t.Run("strategy Weights map overrides deployment Weight field", func(t *testing.T) {
		deployments := []Deployment{
			{ID: "a", Weight: 1},
			{ID: "b", Weight: 1},
		}
		s := &WeightedStrategy{Weights: map[string]int{
			"a": 2,
			"b": 1,
		}}

		counts := map[string]int{}

		for range 3 {
			got := s.Select(deployments, "model")
			assert.NotNil(t, got)
			counts[got.ID]++
		}

		assert.Equal(t, 2, counts["a"])
		assert.Equal(t, 1, counts["b"])
	})

	t.Run("zero weight defaults to 1", func(t *testing.T) {
		deployments := []Deployment{
			{ID: "a", Weight: 0},
			{ID: "b", Weight: 0},
		}
		s := &WeightedStrategy{Weights: map[string]int{}}

		counts := map[string]int{}

		for range 4 {
			got := s.Select(deployments, "model")
			assert.NotNil(t, got)
			counts[got.ID]++
		}

		// Each gets weight 1, so alternating
		assert.Equal(t, 2, counts["a"])
		assert.Equal(t, 2, counts["b"])
	})
}

func TestLeastBusyStrategy_Select(t *testing.T) {
	tests := []struct {
		name        string
		deployments []Deployment
		inflight    map[string]int
		wantNil     bool
		wantID      string
	}{
		{
			name:        "empty list returns nil",
			deployments: nil,
			wantNil:     true,
		},
		{
			name:        "picks deployment with zero inflight",
			deployments: testDeployments("busy", "idle"),
			inflight:    map[string]int{"busy": 5},
			wantID:      "idle",
		},
		{
			name:        "picks deployment with lowest inflight",
			deployments: testDeployments("a", "b", "c"),
			inflight:    map[string]int{"a": 10, "b": 2, "c": 7},
			wantID:      "b",
		},
		{
			name:        "equal counts returns first",
			deployments: testDeployments("a", "b"),
			inflight:    map[string]int{"a": 3, "b": 3},
			wantID:      "a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewInFlightTracker()

			for id, count := range tt.inflight {
				for range count {
					tracker.Increment(id)
				}
			}

			s := &LeastBusyStrategy{Tracker: tracker}
			got := s.Select(tt.deployments, "model")

			if tt.wantNil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				assert.Equal(t, tt.wantID, got.ID)
			}
		})
	}
}

func TestLatencyStrategy_Select(t *testing.T) {
	tests := []struct {
		name        string
		deployments []Deployment
		latencies   map[string]float64
		wantNil     bool
		wantID      string
	}{
		{
			name:        "empty list returns nil",
			deployments: nil,
			wantNil:     true,
		},
		{
			name:        "picks deployment with lowest latency",
			deployments: testDeployments("slow", "fast", "medium"),
			latencies:   map[string]float64{"slow": 500, "fast": 50, "medium": 200},
			wantID:      "fast",
		},
		{
			name:        "prefers untested deployment over tested",
			deployments: testDeployments("tested", "untested"),
			latencies:   map[string]float64{"tested": 100},
			wantID:      "untested",
		},
		{
			name:        "all untested returns first",
			deployments: testDeployments("a", "b"),
			latencies:   map[string]float64{},
			wantID:      "a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewLatencyTracker(1.0) // alpha=1 means latency = exact value

			for id, lat := range tt.latencies {
				tracker.mu.Lock()
				tracker.latency[id] = lat
				tracker.mu.Unlock()
			}

			s := &LatencyStrategy{Tracker: tracker}
			got := s.Select(tt.deployments, "model")

			if tt.wantNil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				assert.Equal(t, tt.wantID, got.ID)
			}
		})
	}
}

func TestUsageStrategy_Select(t *testing.T) {
	tests := []struct {
		name        string
		deployments []Deployment
		usage       map[string]int
		wantNil     bool
		wantID      string
	}{
		{
			name:        "empty list returns nil",
			deployments: nil,
			wantNil:     true,
		},
		{
			name:        "picks deployment with lowest usage",
			deployments: testDeployments("heavy", "light", "medium"),
			usage:       map[string]int{"heavy": 10000, "light": 100, "medium": 5000},
			wantID:      "light",
		},
		{
			name:        "no usage data returns first",
			deployments: testDeployments("a", "b"),
			usage:       map[string]int{},
			wantID:      "a",
		},
		{
			name:        "equal usage returns first",
			deployments: testDeployments("a", "b"),
			usage:       map[string]int{"a": 500, "b": 500},
			wantID:      "a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewUsageTracker(60 * 1e9) // 60s window

			for id, tokens := range tt.usage {
				tracker.Record(id, tokens)
			}

			s := &UsageStrategy{Tracker: tracker}
			got := s.Select(tt.deployments, "model")

			if tt.wantNil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				assert.Equal(t, tt.wantID, got.ID)
			}
		})
	}
}

func TestNewStrategy(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType string
	}{
		{name: "round-robin", input: "round-robin", wantType: "*routing.RoundRobinStrategy"},
		{name: "weighted", input: "weighted", wantType: "*routing.WeightedStrategy"},
		{name: "simple explicit", input: "simple", wantType: "*routing.SimpleStrategy"},
		{name: "unknown defaults to simple", input: "unknown", wantType: "*routing.SimpleStrategy"},
		{name: "empty defaults to simple", input: "", wantType: "*routing.SimpleStrategy"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewStrategy(tt.input)
			assert.IsType(t, got, got) // type check via assertion below
			assert.NotNil(t, got)

			switch tt.wantType {
			case "*routing.RoundRobinStrategy":
				assert.IsType(t, &RoundRobinStrategy{}, got)
			case "*routing.WeightedStrategy":
				assert.IsType(t, &WeightedStrategy{}, got)
			case "*routing.SimpleStrategy":
				assert.IsType(t, &SimpleStrategy{}, got)
			}
		})
	}
}
