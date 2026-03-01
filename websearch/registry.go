package websearch

import "fmt"

// Registry holds registered search backends and provides lookup by name.
type Registry struct {
	clients     map[string]SearchClient
	defaultName string
}

// NewRegistry creates a new search backend registry with the given default backend name.
func NewRegistry(defaultName string) *Registry {
	return &Registry{
		clients:     make(map[string]SearchClient),
		defaultName: defaultName,
	}
}

// Register adds a search client to the registry, keyed by its Name().
func (r *Registry) Register(client SearchClient) {
	r.clients[client.Name()] = client
}

// Get returns the search client for the given name, falling back to the default.
func (r *Registry) Get(name string) (SearchClient, error) {
	if name == "" {
		name = r.defaultName
	}

	if c, ok := r.clients[name]; ok {
		return c, nil
	}

	// Fallback to default
	if c, ok := r.clients[r.defaultName]; ok {
		return c, nil
	}

	return nil, fmt.Errorf("search backend %q not found", name)
}
