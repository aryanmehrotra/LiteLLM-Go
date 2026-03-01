package cost

import (
	"strconv"
	"strings"
	"sync"

	"aryanmehrotra/llm-gateway/models"
)

// ModelPricing holds per-token pricing for a model.
type ModelPricing struct {
	InputPer1KTokens  float64
	OutputPer1KTokens float64
}

// default pricing table for known models (USD per 1K tokens).
var defaultPricing = map[string]ModelPricing{
	// OpenAI
	"gpt-4o":        {InputPer1KTokens: 0.0025, OutputPer1KTokens: 0.01},
	"gpt-4o-mini":   {InputPer1KTokens: 0.00015, OutputPer1KTokens: 0.0006},
	"gpt-4-turbo":   {InputPer1KTokens: 0.01, OutputPer1KTokens: 0.03},
	"gpt-3.5-turbo": {InputPer1KTokens: 0.0005, OutputPer1KTokens: 0.0015},

	// Anthropic
	"claude-sonnet-4-20250514":   {InputPer1KTokens: 0.003, OutputPer1KTokens: 0.015},
	"claude-haiku-4-20250414":    {InputPer1KTokens: 0.0008, OutputPer1KTokens: 0.004},
	"claude-3-5-sonnet-20241022": {InputPer1KTokens: 0.003, OutputPer1KTokens: 0.015},

	// Groq
	"llama-3.3-70b-versatile": {InputPer1KTokens: 0.00059, OutputPer1KTokens: 0.00079},
	"llama-3.1-8b-instant":    {InputPer1KTokens: 0.00005, OutputPer1KTokens: 0.00008},
	"mixtral-8x7b-32768":      {InputPer1KTokens: 0.00024, OutputPer1KTokens: 0.00024},
	"gemma2-9b-it":            {InputPer1KTokens: 0.0002, OutputPer1KTokens: 0.0002},

	// DeepSeek
	"deepseek-chat":     {InputPer1KTokens: 0.00014, OutputPer1KTokens: 0.00028},
	"deepseek-reasoner": {InputPer1KTokens: 0.00055, OutputPer1KTokens: 0.00219},

	// Gemini
	"gemini-2.0-flash":      {InputPer1KTokens: 0.0001, OutputPer1KTokens: 0.0004},
	"gemini-2.0-flash-lite": {InputPer1KTokens: 0.000075, OutputPer1KTokens: 0.0003},
	"gemini-1.5-pro":        {InputPer1KTokens: 0.00125, OutputPer1KTokens: 0.005},
	"gemini-1.5-flash":      {InputPer1KTokens: 0.000075, OutputPer1KTokens: 0.0003},

	// Embeddings
	"text-embedding-3-small": {InputPer1KTokens: 0.00002, OutputPer1KTokens: 0},
	"text-embedding-3-large": {InputPer1KTokens: 0.00013, OutputPer1KTokens: 0},
	"text-embedding-ada-002": {InputPer1KTokens: 0.0001, OutputPer1KTokens: 0},

	// Together AI
	"meta-llama/Llama-3.3-70B-Instruct-Turbo":     {InputPer1KTokens: 0.00088, OutputPer1KTokens: 0.00088},
	"meta-llama/Meta-Llama-3.1-8B-Instruct-Turbo": {InputPer1KTokens: 0.00018, OutputPer1KTokens: 0.00018},
	"Qwen/Qwen2.5-72B-Instruct-Turbo":             {InputPer1KTokens: 0.0012, OutputPer1KTokens: 0.0012},
	"mistralai/Mixtral-8x7B-Instruct-v0.1":        {InputPer1KTokens: 0.0006, OutputPer1KTokens: 0.0006},
	"deepseek-ai/DeepSeek-R1":                     {InputPer1KTokens: 0.003, OutputPer1KTokens: 0.007},

	// Fireworks AI
	"accounts/fireworks/models/llama-v3p3-70b-instruct": {InputPer1KTokens: 0.0009, OutputPer1KTokens: 0.0009},
	"accounts/fireworks/models/llama-v3p1-8b-instruct":  {InputPer1KTokens: 0.0002, OutputPer1KTokens: 0.0002},
	"accounts/fireworks/models/mixtral-8x7b-instruct":   {InputPer1KTokens: 0.0005, OutputPer1KTokens: 0.0005},
	"accounts/fireworks/models/qwen2p5-72b-instruct":    {InputPer1KTokens: 0.0009, OutputPer1KTokens: 0.0009},
	"accounts/fireworks/models/deepseek-r1":             {InputPer1KTokens: 0.003, OutputPer1KTokens: 0.008},

	// Perplexity
	"sonar-pro":           {InputPer1KTokens: 0.003, OutputPer1KTokens: 0.015},
	"sonar":               {InputPer1KTokens: 0.001, OutputPer1KTokens: 0.001},
	"sonar-reasoning-pro": {InputPer1KTokens: 0.002, OutputPer1KTokens: 0.008},
	"sonar-reasoning":     {InputPer1KTokens: 0.001, OutputPer1KTokens: 0.005},

	// xAI (Grok)
	"grok-3":             {InputPer1KTokens: 0.003, OutputPer1KTokens: 0.015},
	"grok-3-mini":        {InputPer1KTokens: 0.0003, OutputPer1KTokens: 0.0005},
	"grok-2-1212":        {InputPer1KTokens: 0.002, OutputPer1KTokens: 0.010},
	"grok-2-vision-1212": {InputPer1KTokens: 0.002, OutputPer1KTokens: 0.010},

	// Mistral AI
	"mistral-large-latest": {InputPer1KTokens: 0.002, OutputPer1KTokens: 0.006},
	"mistral-small-latest": {InputPer1KTokens: 0.0002, OutputPer1KTokens: 0.0006},
	"codestral-latest":     {InputPer1KTokens: 0.0003, OutputPer1KTokens: 0.0009},
	"open-mistral-nemo":    {InputPer1KTokens: 0.00015, OutputPer1KTokens: 0.00015},

	// Cohere
	"command-r-plus":    {InputPer1KTokens: 0.0025, OutputPer1KTokens: 0.010},
	"command-r":         {InputPer1KTokens: 0.00015, OutputPer1KTokens: 0.0006},
	"command-a-03-2025": {InputPer1KTokens: 0.0025, OutputPer1KTokens: 0.010},
	"command-light":     {InputPer1KTokens: 0.00015, OutputPer1KTokens: 0.0006},

	// AWS Bedrock — Anthropic models on Bedrock
	"anthropic.claude-3-5-sonnet-20241022-v2:0": {InputPer1KTokens: 0.003, OutputPer1KTokens: 0.015},
	"anthropic.claude-3-haiku-20240307-v1:0":    {InputPer1KTokens: 0.00025, OutputPer1KTokens: 0.00125},
	// AWS Bedrock — Amazon Nova models
	"amazon.nova-pro-v1:0":  {InputPer1KTokens: 0.0008, OutputPer1KTokens: 0.0032},
	"amazon.nova-lite-v1:0": {InputPer1KTokens: 0.00006, OutputPer1KTokens: 0.00024},
	// AWS Bedrock — Meta and Mistral models
	"meta.llama3-70b-instruct-v1:0":    {InputPer1KTokens: 0.00265, OutputPer1KTokens: 0.0035},
	"mistral.mistral-7b-instruct-v0:2": {InputPer1KTokens: 0.00015, OutputPer1KTokens: 0.0002},

	// Cerebras
	"llama-3.3-70b": {InputPer1KTokens: 0.00059, OutputPer1KTokens: 0.00059},
	"llama-3.1-70b": {InputPer1KTokens: 0.0006, OutputPer1KTokens: 0.0006},
	"llama-3.1-8b":  {InputPer1KTokens: 0.0001, OutputPer1KTokens: 0.0001},

	// SambaNova
	"Meta-Llama-3.3-70B-Instruct":  {InputPer1KTokens: 0.0006, OutputPer1KTokens: 0.0009},
	"Meta-Llama-3.1-405B-Instruct": {InputPer1KTokens: 0.005, OutputPer1KTokens: 0.010},
	"Meta-Llama-3.1-8B-Instruct":   {InputPer1KTokens: 0.0001, OutputPer1KTokens: 0.0002},
	"DeepSeek-R1":                  {InputPer1KTokens: 0.005, OutputPer1KTokens: 0.010},
	"Qwen2.5-72B-Instruct":         {InputPer1KTokens: 0.0006, OutputPer1KTokens: 0.0012},

	// AI21 (Jamba models)
	"jamba-1.5-large": {InputPer1KTokens: 0.002, OutputPer1KTokens: 0.008},
	"jamba-1.5-mini":  {InputPer1KTokens: 0.0002, OutputPer1KTokens: 0.0004},
	"jamba-mini-1.6":  {InputPer1KTokens: 0.0002, OutputPer1KTokens: 0.0004},

	// Nvidia NIM
	"nvidia/llama-3.1-nemotron-70b-instruct": {InputPer1KTokens: 0.00035, OutputPer1KTokens: 0.00040},
	"meta/llama-3.1-405b-instruct":           {InputPer1KTokens: 0.00099, OutputPer1KTokens: 0.00099},
	"meta/llama-3.1-70b-instruct":            {InputPer1KTokens: 0.00035, OutputPer1KTokens: 0.00040},
	"meta/llama-3.1-8b-instruct":             {InputPer1KTokens: 0.00005, OutputPer1KTokens: 0.00005},
	"mistralai/mixtral-8x22b-instruct-v0.1":  {InputPer1KTokens: 0.00040, OutputPer1KTokens: 0.00040},

	// Vertex AI (Gemini on GCP — same pricing band as Gemini API)
	"gemini-2.0-flash-001": {InputPer1KTokens: 0.0001, OutputPer1KTokens: 0.0004},
	"gemini-1.5-pro-002":   {InputPer1KTokens: 0.00125, OutputPer1KTokens: 0.005},
	"gemini-1.5-flash-002": {InputPer1KTokens: 0.000075, OutputPer1KTokens: 0.0003},
	"gemini-1.0-pro-002":   {InputPer1KTokens: 0.0005, OutputPer1KTokens: 0.0015},
}

var (
	overrideMu      sync.RWMutex
	overridePricing = make(map[string]ModelPricing)
)

// SetPricing adds or overrides pricing for a model.
func SetPricing(model string, pricing ModelPricing) {
	overrideMu.Lock()
	overridePricing[model] = pricing
	overrideMu.Unlock()
}

// Calculate computes the dollar cost for a request based on the model and usage.
// Checks custom overrides first, then falls back to default pricing.
// Returns 0 if the model is not in any pricing table.
func Calculate(model string, usage models.Usage) float64 {
	pricing, ok := getPricing(model)
	if !ok {
		return 0
	}

	inputCost := float64(usage.PromptTokens) / 1000 * pricing.InputPer1KTokens
	outputCost := float64(usage.CompletionTokens) / 1000 * pricing.OutputPer1KTokens

	return inputCost + outputCost
}

// GetPricing returns the pricing for a model, if known.
func GetPricing(model string) (ModelPricing, bool) {
	return getPricing(model)
}

func getPricing(model string) (ModelPricing, bool) {
	overrideMu.RLock()
	if p, ok := overridePricing[model]; ok {
		overrideMu.RUnlock()
		return p, true
	}
	overrideMu.RUnlock()

	p, ok := defaultPricing[model]
	return p, ok
}

// ParseCustomPricing parses "model:input:output,model:input:output" format.
func ParseCustomPricing(config string) {
	if config == "" {
		return
	}

	for _, entry := range strings.Split(config, ",") {
		entry = strings.TrimSpace(entry)
		parts := strings.SplitN(entry, ":", 3)

		if len(parts) != 3 {
			continue
		}

		model := strings.TrimSpace(parts[0])
		input, err1 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		output, err2 := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)

		if err1 != nil || err2 != nil {
			continue
		}

		SetPricing(model, ModelPricing{
			InputPer1KTokens:  input,
			OutputPer1KTokens: output,
		})
	}
}
