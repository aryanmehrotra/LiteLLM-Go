package provider

import (
	"fmt"
	"strings"

	"gofr.dev/pkg/gofr"

	"examples/llm-gateway/models"
	"examples/llm-gateway/routing"
)

// Provider translates OpenAI-compatible requests into provider-specific API calls.
type Provider interface {
	Name() string
	ChatCompletion(ctx *gofr.Context, req models.ChatCompletionRequest) (*models.ChatCompletionResponse, error)
	Models() []string
}

// StreamingProvider extends Provider with real-time token streaming.
// The onChunk callback is invoked for each chunk; the handler is responsible
// for delivering chunks to the client (e.g. via WebSocket).
type StreamingProvider interface {
	Provider
	ChatCompletionStream(ctx *gofr.Context, req models.ChatCompletionRequest, onChunk func(models.StreamChunk)) error
}

// EmbeddingProvider is an optional interface for providers that support embeddings.
type EmbeddingProvider interface {
	Provider
	Embedding(ctx *gofr.Context, req models.EmbeddingRequest) (*models.EmbeddingResponse, error)
	EmbeddingModels() []string
}

// Registry maps provider name prefixes to their implementations.
type Registry struct {
	providers       map[string]Provider
	providerOrder   []string // insertion order for deterministic iteration
	aliases         map[string]string
	deployments     []routing.Deployment
	defaultProvider string
}

// NewRegistry creates an empty provider registry.
func NewRegistry(defaultProvider string) *Registry {
	return &Registry{
		providers:       make(map[string]Provider),
		aliases:         make(map[string]string),
		defaultProvider: defaultProvider,
	}
}

// Register adds a provider to the registry.
func (r *Registry) Register(p Provider) {
	name := p.Name()
	if _, exists := r.providers[name]; !exists {
		r.providerOrder = append(r.providerOrder, name)
	}

	r.providers[name] = p
}

// RegisterAlias maps an alias model name to a target "provider/model" string.
func (r *Registry) RegisterAlias(alias, target string) {
	r.aliases[alias] = target
}

// RegisterDeployment adds a deployment to the registry.
func (r *Registry) RegisterDeployment(d routing.Deployment) {
	r.deployments = append(r.deployments, d)
}

// ResolveDeployments returns all registered deployments.
func (r *Registry) ResolveDeployments() []routing.Deployment {
	return r.deployments
}

// ProviderNames returns the list of registered provider names in insertion order.
func (r *Registry) ProviderNames() []string {
	return r.providerOrder
}

// GetProvider returns a provider by name (for building fallback chains etc.).
func (r *Registry) GetProvider(name string) (Provider, bool) {
	p, ok := r.providers[name]
	return p, ok
}

// BuildFallbackChain creates a FallbackProvider from a list of provider names.
func (r *Registry) BuildFallbackChain(names []string, cooldown *routing.CooldownTracker) *FallbackProvider {
	var providers []Provider

	for _, name := range names {
		name = strings.TrimSpace(name)

		p, ok := r.providers[name]
		if !ok {
			continue
		}

		providers = append(providers, p)
	}

	if len(providers) < 2 {
		return nil
	}

	fb := NewFallbackProvider("fallback", providers)
	fb.SetCooldown(cooldown)

	return fb
}

// ResolveProvider parses a model string like "openai/gpt-4o" and returns the
// matching provider along with the cleaned model name ("gpt-4o").
// Unprefixed models are routed to the default provider.
// Aliases are resolved first.
func (r *Registry) ResolveProvider(model string) (Provider, string, error) {
	// Check aliases first
	if target, ok := r.aliases[model]; ok {
		model = target
	}

	parts := strings.SplitN(model, "/", 2)

	var providerName, modelName string

	if len(parts) == 2 {
		providerName = parts[0]
		modelName = parts[1]
	} else {
		providerName = r.defaultProvider
		modelName = model
	}

	p, ok := r.providers[providerName]
	if !ok {
		return nil, "", fmt.Errorf("unknown provider: %s", providerName)
	}

	return p, modelName, nil
}

// ResolveStreamingProvider resolves a provider that supports streaming.
// Returns an error if the provider doesn't implement StreamingProvider.
func (r *Registry) ResolveStreamingProvider(model string) (StreamingProvider, string, error) {
	p, modelName, err := r.ResolveProvider(model)
	if err != nil {
		return nil, "", err
	}

	sp, ok := p.(StreamingProvider)
	if !ok {
		return nil, "", fmt.Errorf("provider %q does not support streaming", p.Name())
	}

	return sp, modelName, nil
}

// ListModels aggregates all models from every registered provider.
// Providers are iterated in registration order for deterministic output.
func (r *Registry) ListModels() []models.ModelInfo {
	var all []models.ModelInfo

	for _, name := range r.providerOrder {
		p := r.providers[name]

		for _, m := range p.Models() {
			all = append(all, models.ModelInfo{
				ID:       name + "/" + m,
				Object:   "model",
				OwnedBy:  name,
				Provider: name,
				Type:     "chat",
			})
		}

		// Also list embedding models if available
		if ep, ok := p.(EmbeddingProvider); ok {
			for _, m := range ep.EmbeddingModels() {
				all = append(all, models.ModelInfo{
					ID:       name + "/" + m,
					Object:   "model",
					OwnedBy:  name,
					Provider: name,
					Type:     "embedding",
				})
			}
		}
	}

	return all
}
