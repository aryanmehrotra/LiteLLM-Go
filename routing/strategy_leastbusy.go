package routing

// LeastBusyStrategy picks the deployment with the lowest in-flight request count.
type LeastBusyStrategy struct {
	Tracker *InFlightTracker
}

func (s *LeastBusyStrategy) Select(deployments []Deployment, _ string) *Deployment {
	if len(deployments) == 0 {
		return nil
	}

	best := &deployments[0]
	bestCount := s.Tracker.Count(best.ID)

	for i := 1; i < len(deployments); i++ {
		count := s.Tracker.Count(deployments[i].ID)
		if count < bestCount {
			best = &deployments[i]
			bestCount = count
		}
	}

	return best
}
