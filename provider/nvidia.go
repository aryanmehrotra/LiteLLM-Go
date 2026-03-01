package provider

import "time"

// NewNvidianim creates an Nvidia NIM provider using the shared OpenAI-compatible base.
// Nvidia NIM exposes optimized model inference at integrate.api.nvidia.com.
func NewNvidianim(apiKey string, timeout time.Duration) *OpenAICompatible {
	return NewOpenAICompatible(
		"nvidia", "nvidia", apiKey,
		[]string{
			"nvidia/llama-3.1-nemotron-70b-instruct",
			"meta/llama-3.1-405b-instruct",
			"meta/llama-3.1-70b-instruct",
			"meta/llama-3.1-8b-instruct",
			"mistralai/mixtral-8x22b-instruct-v0.1",
		},
		timeout,
	)
}
