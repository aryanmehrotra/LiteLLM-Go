package provider

import "time"

// NewCerebras creates a Cerebras provider using the shared OpenAI-compatible base.
// Cerebras offers high-speed inference on their custom silicon.
func NewCerebras(apiKey string, timeout time.Duration) *OpenAICompatible {
	return NewOpenAICompatible(
		"cerebras", "cerebras", apiKey,
		[]string{"llama-3.3-70b", "llama-3.1-70b", "llama-3.1-8b"},
		timeout,
	)
}
