package provider

import "time"

// NewSambaNova creates a SambaNova provider using the shared OpenAI-compatible base.
// SambaNova offers high-throughput inference via its RDU hardware.
func NewSambaNova(apiKey string, timeout time.Duration) *OpenAICompatible {
	return NewOpenAICompatible(
		"sambanova", "sambanova", apiKey,
		[]string{
			"Meta-Llama-3.3-70B-Instruct",
			"Meta-Llama-3.1-405B-Instruct",
			"Meta-Llama-3.1-8B-Instruct",
			"DeepSeek-R1",
			"Qwen2.5-72B-Instruct",
		},
		timeout,
	)
}
