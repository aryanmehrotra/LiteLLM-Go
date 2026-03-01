package provider

import "time"

// NewMistral creates a Mistral AI provider using the shared OpenAI-compatible base.
func NewMistral(apiKey string, timeout time.Duration) *OpenAICompatible {
	return NewOpenAICompatible(
		"mistral", "mistral", apiKey,
		[]string{
			"mistral-large-latest",
			"mistral-small-latest",
			"codestral-latest",
			"open-mistral-nemo",
		},
		timeout,
	)
}
