package provider

import "time"

// NewOpenRouter creates an OpenRouter provider using the shared OpenAI-compatible base.
// OpenRouter is a meta-router that proxies requests to many upstream providers.
// Its base URL is https://openrouter.ai and the chat path is api/v1/chat/completions.
func NewOpenRouter(apiKey string, timeout time.Duration) *OpenAICompatible {
	p := NewOpenAICompatible(
		"openrouter", "openrouter", apiKey,
		[]string{
			"openai/gpt-4o",
			"anthropic/claude-3-5-sonnet",
			"google/gemini-2.0-flash-001",
			"meta-llama/llama-3.3-70b-instruct",
			"deepseek/deepseek-r1",
			"mistralai/mistral-large-2411",
		},
		timeout,
	)
	p.SetChatPath("api/v1/chat/completions")

	return p
}
