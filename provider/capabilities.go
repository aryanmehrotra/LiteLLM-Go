package provider

import "aryanmehrotra/litellm-go/models"

// Capability flags for models.
type Capability int

const (
	CapTools     Capability = 1 << iota // supports function calling
	CapVision                           // supports image input
	CapJSON                             // supports JSON mode
	CapStreaming                        // supports streaming
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

	// Together AI
	"meta-llama/Llama-3.3-70B-Instruct-Turbo":     CapTools | CapStreaming,
	"meta-llama/Meta-Llama-3.1-8B-Instruct-Turbo": CapTools | CapStreaming,
	"Qwen/Qwen2.5-72B-Instruct-Turbo":             CapTools | CapStreaming,
	"mistralai/Mixtral-8x7B-Instruct-v0.1":        CapStreaming,
	"deepseek-ai/DeepSeek-R1":                     CapStreaming,

	// Fireworks AI
	"accounts/fireworks/models/llama-v3p3-70b-instruct": CapTools | CapStreaming,
	"accounts/fireworks/models/llama-v3p1-8b-instruct":  CapTools | CapStreaming,
	"accounts/fireworks/models/mixtral-8x7b-instruct":   CapStreaming,
	"accounts/fireworks/models/qwen2p5-72b-instruct":    CapTools | CapStreaming,
	"accounts/fireworks/models/deepseek-r1":             CapStreaming,

	// Perplexity — search-augmented, streaming only (no tool calling on sonar models)
	"sonar-pro":           CapStreaming,
	"sonar":               CapStreaming,
	"sonar-reasoning-pro": CapStreaming,
	"sonar-reasoning":     CapStreaming,

	// xAI (Grok)
	"grok-3":             CapTools | CapStreaming,
	"grok-3-mini":        CapTools | CapStreaming,
	"grok-2-1212":        CapTools | CapStreaming,
	"grok-2-vision-1212": CapTools | CapVision | CapStreaming,

	// Mistral AI
	"mistral-large-latest": CapTools | CapStreaming,
	"mistral-small-latest": CapTools | CapStreaming,
	"codestral-latest":     CapStreaming,
	"open-mistral-nemo":    CapStreaming,

	// Cohere
	"command-r-plus":    CapTools | CapStreaming,
	"command-r":         CapTools | CapStreaming,
	"command-a-03-2025": CapTools | CapStreaming,
	"command-light":     CapStreaming,

	// AWS Bedrock
	"anthropic.claude-3-5-sonnet-20241022-v2:0": CapTools | CapVision | CapStreaming,
	"anthropic.claude-3-haiku-20240307-v1:0":    CapTools | CapVision | CapStreaming,
	"amazon.nova-pro-v1:0":                      CapTools | CapVision | CapStreaming,
	"amazon.nova-lite-v1:0":                     CapTools | CapStreaming,
	"meta.llama3-70b-instruct-v1:0":             CapTools | CapStreaming,
	"mistral.mistral-7b-instruct-v0:2":          CapStreaming,

	// Cerebras — fast inference, tools on Llama 3.3+
	"llama-3.3-70b": CapTools | CapStreaming,
	"llama-3.1-70b": CapTools | CapStreaming,
	"llama-3.1-8b":  CapTools | CapStreaming,

	// SambaNova
	"Meta-Llama-3.3-70B-Instruct":  CapTools | CapStreaming,
	"Meta-Llama-3.1-405B-Instruct": CapTools | CapStreaming,
	"Meta-Llama-3.1-8B-Instruct":   CapTools | CapStreaming,
	"DeepSeek-R1":                  CapStreaming,
	"Qwen2.5-72B-Instruct":         CapTools | CapStreaming,

	// AI21 (Jamba models)
	"jamba-1.5-large": CapTools | CapStreaming,
	"jamba-1.5-mini":  CapTools | CapStreaming,
	"jamba-mini-1.6":  CapTools | CapStreaming,

	// OpenRouter — pass-through, assume fully capable for all listed models
	"openai/gpt-4o":                     CapTools | CapVision | CapJSON | CapStreaming,
	"anthropic/claude-3-5-sonnet":       CapTools | CapVision | CapStreaming,
	"google/gemini-2.0-flash-001":       CapTools | CapVision | CapStreaming,
	"meta-llama/llama-3.3-70b-instruct": CapTools | CapStreaming,
	"deepseek/deepseek-r1":              CapStreaming,
	"mistralai/mistral-large-2411":      CapTools | CapStreaming,

	// Novita — only entries unique to Novita (others overlap with OpenRouter)
	"mistralai/mistral-7b-instruct": CapStreaming,
	"qwen/qwen2.5-72b-instruct":     CapTools | CapStreaming,

	// Nvidia NIM
	"nvidia/llama-3.1-nemotron-70b-instruct": CapTools | CapStreaming,
	"meta/llama-3.1-405b-instruct":           CapTools | CapStreaming,
	"meta/llama-3.1-70b-instruct":            CapTools | CapStreaming,
	"meta/llama-3.1-8b-instruct":             CapTools | CapStreaming,
	"mistralai/mixtral-8x22b-instruct-v0.1":  CapStreaming,

	// Cloudflare Workers AI
	"@cf/meta/llama-3.3-70b-instruct-fp8-fast": CapTools | CapStreaming,
	"@cf/meta/llama-3.1-70b-instruct":          CapTools | CapStreaming,
	"@cf/meta/llama-3.1-8b-instruct":           CapTools | CapStreaming,
	"@cf/mistral/mistral-7b-instruct-v0.2":     CapStreaming,
	"@cf/qwen/qwen1.5-14b-chat-awq":            CapStreaming,

	// Vertex AI (Gemini models on GCP)
	"gemini-2.0-flash-001": CapTools | CapVision | CapStreaming,
	"gemini-1.5-pro-002":   CapTools | CapVision | CapStreaming,
	"gemini-1.5-flash-002": CapTools | CapVision | CapStreaming,
	"gemini-1.0-pro-002":   CapTools | CapStreaming,

	// HuggingFace (open models via TGI Messages API — tool support varies)
	"mistralai/Mistral-7B-Instruct-v0.3": CapTools | CapStreaming,
	"meta-llama/Llama-3.3-70B-Instruct":  CapTools | CapStreaming,
	"meta-llama/Llama-3.1-8B-Instruct":   CapTools | CapStreaming,
	"Qwen/Qwen2.5-72B-Instruct":          CapTools | CapStreaming,
	"microsoft/Phi-3.5-mini-instruct":    CapStreaming,
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
