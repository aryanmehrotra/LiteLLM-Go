package handler

import (
	"examples/llm-gateway/cache"
	"examples/llm-gateway/guardrails"
	"examples/llm-gateway/provider"
	"examples/llm-gateway/routing"
	"examples/llm-gateway/websearch"
)

// APIHandler groups all LLM API endpoint handlers with their shared dependencies.
type APIHandler struct {
	Registry   *provider.Registry
	Cache      *cache.Cache
	Router     *routing.Router
	Guardrails *guardrails.GlobalConfig
	Search     *websearch.Service
}
