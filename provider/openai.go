package provider

import "time"

// NewOpenAI creates an OpenAI provider using the shared OpenAI-compatible base.
func NewOpenAI(apiKey string, timeout time.Duration) *OpenAICompatible {
	o := NewOpenAICompatible(
		"openai", "openai", apiKey,
		[]string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "gpt-3.5-turbo"},
		timeout,
	)
	o.SetEmbeddingModels([]string{"text-embedding-3-small", "text-embedding-3-large", "text-embedding-ada-002"})

	return o
}
