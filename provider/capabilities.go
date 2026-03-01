package provider

import "examples/llm-gateway/models"

// Capability flags for models.
type Capability int

const (
	CapTools        Capability = 1 << iota // supports function calling
	CapVision                              // supports image input
	CapJSON                                // supports JSON mode
	CapStreaming                            // supports streaming
)

// modelCapabilities maps model names to their capability flags.
var modelCapabilities = map[string]Capability{
	// OpenAI — all support tools
	"gpt-4o":        CapTools | CapVision | CapJSON | CapStreaming,
	"gpt-4o-mini":   CapTools | CapVision | CapJSON | CapStreaming,
	"gpt-4-turbo":   CapTools | CapVision | CapJSON | CapStreaming,
	"gpt-3.5-turbo": CapTools | CapJSON | CapStreaming,

	// Anthropic
	"claude-sonnet-4-20250514":   CapTools | CapVision | CapStreaming,
	"claude-haiku-4-20250414":    CapTools | CapVision | CapStreaming,
	"claude-3-5-sonnet-20241022": CapTools | CapVision | CapStreaming,

	// Groq — tools on some models
	"llama-3.3-70b-versatile": CapTools | CapStreaming,
	"llama-3.1-8b-instant":    CapTools | CapStreaming,
	"mixtral-8x7b-32768":      CapStreaming,
	"gemma2-9b-it":            CapStreaming,

	// DeepSeek
	"deepseek-chat":     CapTools | CapStreaming,
	"deepseek-reasoner": CapStreaming,

	// Gemini
	"gemini-2.0-flash":      CapTools | CapVision | CapStreaming,
	"gemini-2.0-flash-lite": CapTools | CapStreaming,
	"gemini-1.5-pro":        CapTools | CapVision | CapStreaming,
	"gemini-1.5-flash":      CapTools | CapVision | CapStreaming,

	// Ollama — tools supported on some models
	"llama3":    CapStreaming,
	"llama3.1":  CapTools | CapStreaming,
	"mistral":   CapStreaming,
	"codellama": CapStreaming,
	"gemma2":    CapStreaming,
}

// HasCapability checks if a model has a specific capability.
func HasCapability(model string, cap Capability) bool {
	caps, ok := modelCapabilities[model]
	if !ok {
		return true // assume capable if unknown
	}

	return caps&cap != 0
}

// StripUnsupportedParams removes tools/tool_choice from a request if the
// model doesn't support function calling.
func StripUnsupportedParams(model string, req *models.ChatCompletionRequest) bool {
	if !HasCapability(model, CapTools) && len(req.Tools) > 0 {
		req.Tools = nil
		req.ToolChoice = nil

		return true
	}

	return false
}
