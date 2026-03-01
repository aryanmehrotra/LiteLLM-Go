package provider

import "time"

// NewTogetherAI creates a Together AI provider using the shared OpenAI-compatible base.
func NewTogetherAI(apiKey string, timeout time.Duration) *OpenAICompatible {
	return NewOpenAICompatible(
		"togetherai", "togetherai", apiKey,
		[]string{
			"meta-llama/Llama-3.3-70B-Instruct-Turbo",
			"meta-llama/Meta-Llama-3.1-8B-Instruct-Turbo",
			"Qwen/Qwen2.5-72B-Instruct-Turbo",
			"mistralai/Mixtral-8x7B-Instruct-v0.1",
			"deepseek-ai/DeepSeek-R1",
		},
		timeout,
	)
}
