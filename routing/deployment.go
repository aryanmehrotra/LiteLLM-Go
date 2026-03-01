package routing

// Deployment represents a single provider deployment with routing metadata.
type Deployment struct {
	ID       string
	Provider ChatProvider
	Weight   int
	Metadata map[string]string
	Tags     map[string]string
}

// FilterByTags returns deployments that match all the given tags.
// If tags is nil or empty, all deployments are returned.
func FilterByTags(deployments []Deployment, tags map[string]string) []Deployment {
	if len(tags) == 0 {
		return deployments
	}

	var filtered []Deployment

	for _, d := range deployments {
		if matchesTags(d.Tags, tags) {
			filtered = append(filtered, d)
		}
	}

	if len(filtered) == 0 {
		return deployments // fall back to all if no matches
	}

	return filtered
}

func matchesTags(deploymentTags, requestTags map[string]string) bool {
	for k, v := range requestTags {
		if deploymentTags[k] != v {
			return false
		}
	}

	return true
}

// StreamingDeployment wraps a Deployment whose provider also supports streaming.
type StreamingDeployment struct {
	Deployment
	StreamProvider StreamChatProvider
}

// AsStreaming returns a StreamingDeployment if the provider supports streaming.
func (d *Deployment) AsStreaming() (*StreamingDeployment, bool) {
	sp, ok := d.Provider.(StreamChatProvider)
	if !ok {
		return nil, false
	}

	return &StreamingDeployment{
		Deployment:     *d,
		StreamProvider: sp,
	}, true
}
