package provider

import (
	"testing"
)

func TestRegistry_RegisterAndResolve(t *testing.T) {
	reg := NewRegistry("openai")
	reg.Register(NewOpenAI("key", 0))
	reg.Register(NewGroq("key", 0))

	p, modelName, err := reg.ResolveProvider("openai/gpt-4o")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.Name() != "openai" {
		t.Errorf("expected provider 'openai', got %q", p.Name())
	}

	if modelName != "gpt-4o" {
		t.Errorf("expected model 'gpt-4o', got %q", modelName)
	}
}

func TestRegistry_DefaultProvider(t *testing.T) {
	reg := NewRegistry("openai")
	reg.Register(NewOpenAI("key", 0))

	// No prefix — should use default provider
	p, modelName, err := reg.ResolveProvider("gpt-4o")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.Name() != "openai" {
		t.Errorf("expected default provider 'openai', got %q", p.Name())
	}

	if modelName != "gpt-4o" {
		t.Errorf("expected model 'gpt-4o', got %q", modelName)
	}
}

func TestRegistry_UnknownProvider(t *testing.T) {
	reg := NewRegistry("openai")
	reg.Register(NewOpenAI("key", 0))

	_, _, err := reg.ResolveProvider("unknown/model")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestRegistry_Aliases(t *testing.T) {
	reg := NewRegistry("openai")
	reg.Register(NewOpenAI("key", 0))
	reg.RegisterAlias("gpt4", "openai/gpt-4o")

	p, modelName, err := reg.ResolveProvider("gpt4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.Name() != "openai" {
		t.Errorf("expected provider 'openai', got %q", p.Name())
	}

	if modelName != "gpt-4o" {
		t.Errorf("expected model 'gpt-4o', got %q", modelName)
	}
}

func TestRegistry_ListModels(t *testing.T) {
	reg := NewRegistry("openai")
	reg.Register(NewOpenAI("key", 0))
	reg.Register(NewGroq("key", 0))

	models := reg.ListModels()
	if len(models) == 0 {
		t.Fatal("expected non-empty model list")
	}

	// Should contain models from both providers
	found := map[string]bool{}
	for _, m := range models {
		found[m.ID] = true
	}

	if !found["openai/gpt-4o"] {
		t.Error("expected 'openai/gpt-4o' in model list")
	}

	if !found["groq/llama-3.3-70b-versatile"] {
		t.Error("expected 'groq/llama-3.3-70b-versatile' in model list")
	}
}

func TestRegistry_ProviderNames(t *testing.T) {
	reg := NewRegistry("openai")
	reg.Register(NewOpenAI("key", 0))
	reg.Register(NewGroq("key", 0))

	names := reg.ProviderNames()
	if len(names) != 2 {
		t.Fatalf("expected 2 provider names, got %d", len(names))
	}

	if names[0] != "openai" {
		t.Errorf("expected first provider 'openai', got %q", names[0])
	}

	if names[1] != "groq" {
		t.Errorf("expected second provider 'groq', got %q", names[1])
	}
}

func TestRegistry_GetProvider(t *testing.T) {
	reg := NewRegistry("openai")
	reg.Register(NewOpenAI("key", 0))

	p, ok := reg.GetProvider("openai")
	if !ok {
		t.Fatal("expected provider to be found")
	}

	if p.Name() != "openai" {
		t.Errorf("expected 'openai', got %q", p.Name())
	}

	_, ok = reg.GetProvider("nonexistent")
	if ok {
		t.Error("expected false for nonexistent provider")
	}
}

func TestRegistry_ResolveStreamingProvider(t *testing.T) {
	reg := NewRegistry("openai")
	reg.Register(NewOpenAI("key", 0))

	// OpenAICompatible implements StreamingProvider
	sp, modelName, err := reg.ResolveStreamingProvider("openai/gpt-4o")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sp.Name() != "openai" {
		t.Errorf("expected provider 'openai', got %q", sp.Name())
	}

	if modelName != "gpt-4o" {
		t.Errorf("expected model 'gpt-4o', got %q", modelName)
	}
}

func TestRegistry_BuildFallbackChain_TooFewProviders(t *testing.T) {
	reg := NewRegistry("openai")
	reg.Register(NewOpenAI("key", 0))

	// Need at least 2 providers for fallback
	fb := reg.BuildFallbackChain([]string{"openai"}, nil)
	if fb != nil {
		t.Error("expected nil fallback for single provider")
	}
}

func TestRegistry_BuildFallbackChain_Success(t *testing.T) {
	reg := NewRegistry("openai")
	reg.Register(NewOpenAI("key", 0))
	reg.Register(NewGroq("key", 0))

	fb := reg.BuildFallbackChain([]string{"openai", "groq"}, nil)
	if fb == nil {
		t.Fatal("expected non-nil fallback")
	}

	if fb.Name() != "fallback" {
		t.Errorf("expected 'fallback', got %q", fb.Name())
	}
}

func TestRegistry_DuplicateRegisterOverwrites(t *testing.T) {
	reg := NewRegistry("openai")
	reg.Register(NewOpenAI("key1", 0))
	reg.Register(NewOpenAI("key2", 0)) // Should overwrite, not duplicate

	names := reg.ProviderNames()
	if len(names) != 1 {
		t.Fatalf("expected 1 provider name after duplicate register, got %d", len(names))
	}
}
