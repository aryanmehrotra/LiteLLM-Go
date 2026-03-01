package provider

import "time"

// NewXAI creates an xAI (Grok) provider using the shared OpenAI-compatible base.
func NewXAI(apiKey string, timeout time.Duration) *OpenAICompatible {
	return NewOpenAICompatible(
		"xai", "xai", apiKey,
		[]string{"grok-3", "grok-3-mini", "grok-2-1212", "grok-2-vision-1212"},
		timeout,
	)
}
