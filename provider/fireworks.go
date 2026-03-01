package provider

import "time"

// NewFireworks creates a Fireworks AI provider using the shared OpenAI-compatible base.
// The base URL for Fireworks is https://api.fireworks.ai/inference which exposes
// the /v1/chat/completions endpoint directly.
func NewFireworks(apiKey string, timeout time.Duration) *OpenAICompatible {
	return NewOpenAICompatible(
		"fireworks", "fireworks", apiKey,
		[]string{
			"accounts/fireworks/models/llama-v3p3-70b-instruct",
			"accounts/fireworks/models/llama-v3p1-8b-instruct",
			"accounts/fireworks/models/mixtral-8x7b-instruct",
			"accounts/fireworks/models/qwen2p5-72b-instruct",
			"accounts/fireworks/models/deepseek-r1",
		},
		timeout,
	)
}
