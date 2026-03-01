package handler

import (
	"aryanmehrotra/litellm-go/cache"
	"aryanmehrotra/litellm-go/guardrails"
	"aryanmehrotra/litellm-go/provider"
	"aryanmehrotra/litellm-go/routing"
	"aryanmehrotra/litellm-go/websearch"
)

// APIHandler groups all LLM API endpoint handlers with their shared dependencies.
type APIHandler struct {
	Registry   *provider.Registry
	Cache      *cache.Cache
	Router     *routing.Router
	Guardrails *guardrails.GlobalConfig
	Search     *websearch.Service
}
