package provider

import "time"

// NewDeepSeek creates a DeepSeek provider using the shared OpenAI-compatible base.
func NewDeepSeek(apiKey string, timeout time.Duration) *OpenAICompatible {
	return NewOpenAICompatible(
		"deepseek", "deepseek", apiKey,
		[]string{"deepseek-chat", "deepseek-reasoner"},
		timeout,
	)
}
