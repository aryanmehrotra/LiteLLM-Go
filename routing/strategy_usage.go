package routing

// UsageStrategy picks the deployment with the lowest windowed token usage.
type UsageStrategy struct {
	Tracker *UsageTracker
}

func (s *UsageStrategy) Select(deployments []Deployment, _ string) *Deployment {
	if len(deployments) == 0 {
		return nil
	}

	best := &deployments[0]
	bestUsage := s.Tracker.Usage(best.ID)

	for i := 1; i < len(deployments); i++ {
		u := s.Tracker.Usage(deployments[i].ID)
		if u < bestUsage {
			best = &deployments[i]
			bestUsage = u
		}
	}

	return best
}
