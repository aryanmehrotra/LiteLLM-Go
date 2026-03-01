package provider

import "time"

// NewPerplexity creates a Perplexity provider. Perplexity's API uses
// /chat/completions without the v1/ prefix, so we override the chat path.
func NewPerplexity(apiKey string, timeout time.Duration) *OpenAICompatible {
	p := NewOpenAICompatible(
		"perplexity", "perplexity", apiKey,
		[]string{"sonar-pro", "sonar", "sonar-reasoning-pro", "sonar-reasoning"},
		timeout,
	)
	p.SetChatPath("chat/completions")

	return p
}
