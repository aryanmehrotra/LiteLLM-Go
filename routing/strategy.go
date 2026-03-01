package routing

import "sync/atomic"

// Strategy selects a deployment from a list of candidates.
type Strategy interface {
	Select(deployments []Deployment, model string) *Deployment
}

// SimpleStrategy returns the first available deployment.
type SimpleStrategy struct{}

func (s *SimpleStrategy) Select(deployments []Deployment, _ string) *Deployment {
	if len(deployments) > 0 {
		return &deployments[0]
	}

	return nil
}

// RoundRobinStrategy rotates through deployments using an atomic counter.
type RoundRobinStrategy struct {
	counter atomic.Uint64
}

func (s *RoundRobinStrategy) Select(deployments []Deployment, _ string) *Deployment {
	if len(deployments) == 0 {
		return nil
	}

	idx := s.counter.Add(1) - 1

	return &deployments[idx%uint64(len(deployments))]
}

// WeightedStrategy selects deployments by configured weights.
// Heavier weight = more traffic. Uses a simple cumulative weight approach.
type WeightedStrategy struct {
	Weights map[string]int // deployment ID → weight
	counter atomic.Uint64
}

func (s *WeightedStrategy) Select(deployments []Deployment, _ string) *Deployment {
	if len(deployments) == 0 {
		return nil
	}

	// Build weighted slots
	var slots []*Deployment

	for i := range deployments {
		w := deployments[i].Weight
		if cw, ok := s.Weights[deployments[i].ID]; ok {
			w = cw
		}

		if w <= 0 {
			w = 1
		}

		for range w {
			slots = append(slots, &deployments[i])
		}
	}

	if len(slots) == 0 {
		return &deployments[0]
	}

	idx := s.counter.Add(1) - 1

	return slots[idx%uint64(len(slots))]
}

// NewStrategy creates a Strategy from a config string.
func NewStrategy(name string) Strategy {
	switch name {
	case "round-robin":
		return &RoundRobinStrategy{}
	case "weighted":
		return &WeightedStrategy{Weights: make(map[string]int)}
	default:
		return &SimpleStrategy{}
	}
}
