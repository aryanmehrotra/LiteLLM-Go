package handler

import (
	"aryanmehrotra/llm-gateway/cache"
	"aryanmehrotra/llm-gateway/guardrails"
	"aryanmehrotra/llm-gateway/provider"
	"aryanmehrotra/llm-gateway/routing"
	"aryanmehrotra/llm-gateway/websearch"
)

// APIHandler groups all LLM API endpoint handlers with their shared dependencies.
type APIHandler struct {
	Registry   *provider.Registry
	Cache      *cache.Cache
	Router     *routing.Router
	Guardrails *guardrails.GlobalConfig
	Search     *websearch.Service
}
