package main

import (
	"sync"

	"aryanmehrotra/litellm-go/provider"
	"aryanmehrotra/litellm-go/routing"
)

// Gateway provides thread-safe access to the registry and router,
// allowing hot-reload of configuration.
type Gateway struct {
	mu       sync.RWMutex
	registry *provider.Registry
	router   *routing.Router
}

// NewGateway creates a Gateway wrapping the initial registry and router.
func NewGateway(reg *provider.Registry, router *routing.Router) *Gateway {
	return &Gateway{
		registry: reg,
		router:   router,
	}
}

// Registry returns the current registry (read-locked).
func (g *Gateway) Registry() *provider.Registry {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return g.registry
}

// Router returns the current router (read-locked).
func (g *Gateway) Router() *routing.Router {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return g.router
}

// Swap atomically replaces the registry and router.
func (g *Gateway) Swap(reg *provider.Registry, router *routing.Router) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.registry = reg
	g.router = router
}
