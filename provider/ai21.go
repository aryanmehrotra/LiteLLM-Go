package provider

import "time"

// NewAI21 creates an AI21 Labs provider using the shared OpenAI-compatible base.
// AI21's chat endpoint lives at api.ai21.com/studio, which is registered as the
// base URL, so the default v1/chat/completions path applies directly.
func NewAI21(apiKey string, timeout time.Duration) *OpenAICompatible {
	return NewOpenAICompatible(
		"ai21", "ai21", apiKey,
		[]string{"jamba-1.5-large", "jamba-1.5-mini", "jamba-mini-1.6"},
		timeout,
	)
}
