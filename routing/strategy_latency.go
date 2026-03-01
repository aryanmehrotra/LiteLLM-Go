package routing

// LatencyStrategy picks the deployment with the lowest EMA latency.
type LatencyStrategy struct {
	Tracker *LatencyTracker
}

func (s *LatencyStrategy) Select(deployments []Deployment, _ string) *Deployment {
	if len(deployments) == 0 {
		return nil
	}

	best := &deployments[0]
	bestLatency := s.Tracker.Latency(best.ID)

	for i := 1; i < len(deployments); i++ {
		lat := s.Tracker.Latency(deployments[i].ID)

		// Prefer untested deployments (latency=0) or lower latency
		if bestLatency > 0 && (lat == 0 || lat < bestLatency) {
			best = &deployments[i]
			bestLatency = lat
		}
	}

	return best
}
