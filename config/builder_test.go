package config

import (
	"testing"
	"time"

	"aryanmehrotra/llm-gateway/routing"
)

func TestCreateProvider_KnownProviders(t *testing.T) {
	// Providers that take (apiKey, timeout) and should return non-nil
	simpleProviders := []string{
		"openai", "anthropic", "groq", "deepseek", "gemini",
		"togetherai", "fireworks", "perplexity", "xai", "mistral",
		"cohere", "cerebras", "sambanova", "ai21", "openrouter",
		"novita", "nvidia",
	}

	for _, name := range simpleProviders {
		t.Run(name, func(t *testing.T) {
			p := createProvider(name, "test-key", 5*time.Second)
			if p == nil {
				t.Errorf("expected non-nil provider for %q", name)
				return
			}

			if p.Name() != name {
				t.Errorf("expected provider name %q, got %q", name, p.Name())
			}
		})
	}
}

func TestCreateProvider_Ollama(t *testing.T) {
	// Ollama doesn't use an API key
	p := createProvider("ollama", "", 5*time.Second)
	if p == nil {
		t.Fatal("expected non-nil ollama provider")
	}

	if p.Name() != "ollama" {
		t.Errorf("expected 'ollama', got %q", p.Name())
	}
}

func TestCreateProvider_Unknown(t *testing.T) {
	p := createProvider("nonexistent", "key", 5*time.Second)
	if p != nil {
		t.Errorf("expected nil for unknown provider, got %v", p)
	}
}

func TestBuildStrategyFromConfig_AllStrategies(t *testing.T) {
	inFlight := newTestInFlight()
	latency := newTestLatency()
	usage := newTestUsage()

	tests := []struct {
		name     string
		strategy string
	}{
		{"simple", "simple"},
		{"simple default", ""},
		{"round-robin", "round-robin"},
		{"weighted", "weighted"},
		{"least-busy", "least-busy"},
		{"latency", "latency"},
		{"usage", "usage"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := buildStrategyFromConfig(tt.strategy, inFlight, latency, usage)
			if s == nil {
				t.Fatal("expected non-nil strategy")
			}
		})
	}
}

// Helper constructors for tracker types needed by buildStrategyFromConfig
func newTestInFlight() *routing.InFlightTracker {
	return routing.NewInFlightTracker()
}

func newTestLatency() *routing.LatencyTracker {
	return routing.NewLatencyTracker(0.2)
}

func newTestUsage() *routing.UsageTracker {
	return routing.NewUsageTracker(60 * time.Second)
}
