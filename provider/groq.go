package provider

import "time"

// NewGroq creates a Groq provider using the shared OpenAI-compatible base.
func NewGroq(apiKey string, timeout time.Duration) *OpenAICompatible {
	return NewOpenAICompatible(
		"groq", "groq", apiKey,
		[]string{"llama-3.3-70b-versatile", "llama-3.1-8b-instant", "mixtral-8x7b-32768", "gemma2-9b-it"},
		timeout,
	)
}
