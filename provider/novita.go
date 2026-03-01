package provider

import "time"

// NewNovita creates a Novita AI provider using the shared OpenAI-compatible base.
// Novita's chat endpoint is at /v3/openai/chat/completions.
func NewNovita(apiKey string, timeout time.Duration) *OpenAICompatible {
	p := NewOpenAICompatible(
		"novita", "novita", apiKey,
		[]string{
			"meta-llama/llama-3.3-70b-instruct",
			"meta-llama/llama-3.1-8b-instruct",
			"deepseek/deepseek-r1",
			"mistralai/mistral-7b-instruct",
			"qwen/qwen2.5-72b-instruct",
		},
		timeout,
	)
	p.SetChatPath("v3/openai/chat/completions")

	return p
}
