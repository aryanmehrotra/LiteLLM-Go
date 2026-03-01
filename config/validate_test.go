package config

import (
	"strings"
	"testing"
)

func TestValidate_ValidConfig(t *testing.T) {
	cfg := &GatewayConfig{
		ModelList: []ModelConfig{
			{
				ModelName: "gpt-4o",
				LiteLLM:   LiteLLMParams{Model: "openai/gpt-4o"},
			},
		},
		RouterSettings: RouterSettings{Strategy: "simple"},
	}

	if err := Validate(cfg); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidate_EmptyModelList(t *testing.T) {
	cfg := &GatewayConfig{
		ModelList: []ModelConfig{},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for empty model_list")
	}

	if !strings.Contains(err.Error(), "model_list is empty") {
		t.Errorf("expected 'model_list is empty', got: %v", err)
	}
}

func TestValidate_MissingModelName(t *testing.T) {
	cfg := &GatewayConfig{
		ModelList: []ModelConfig{
			{
				ModelName: "",
				LiteLLM:   LiteLLMParams{Model: "openai/gpt-4o"},
			},
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for missing model_name")
	}

	if !strings.Contains(err.Error(), "model_name is required") {
		t.Errorf("expected 'model_name is required', got: %v", err)
	}
}

func TestValidate_MissingModel(t *testing.T) {
	cfg := &GatewayConfig{
		ModelList: []ModelConfig{
			{
				ModelName: "test",
				LiteLLM:   LiteLLMParams{Model: ""},
			},
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for missing model")
	}

	if !strings.Contains(err.Error(), "litellm_params.model is required") {
		t.Errorf("expected 'litellm_params.model is required', got: %v", err)
	}
}

func TestValidate_UnknownProvider(t *testing.T) {
	cfg := &GatewayConfig{
		ModelList: []ModelConfig{
			{
				ModelName: "test",
				LiteLLM:   LiteLLMParams{Model: "unknown_provider/model"},
			},
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}

	if !strings.Contains(err.Error(), "unknown provider") {
		t.Errorf("expected 'unknown provider', got: %v", err)
	}
}

func TestValidate_KnownProviders(t *testing.T) {
	providers := []string{
		"openai", "anthropic", "ollama", "groq", "deepseek", "gemini",
		"togetherai", "fireworks", "perplexity", "xai", "mistral", "cohere",
		"azure", "bedrock", "cerebras", "sambanova", "ai21", "openrouter",
		"novita", "nvidia", "cloudflare", "vertex", "huggingface",
	}

	for _, p := range providers {
		t.Run(p, func(t *testing.T) {
			cfg := &GatewayConfig{
				ModelList: []ModelConfig{
					{
						ModelName: "test-" + p,
						LiteLLM:   LiteLLMParams{Model: p + "/some-model"},
					},
				},
			}

			if err := Validate(cfg); err != nil {
				t.Errorf("expected no error for provider %q, got: %v", p, err)
			}
		})
	}
}

func TestValidate_UnknownStrategy(t *testing.T) {
	cfg := &GatewayConfig{
		ModelList: []ModelConfig{
			{
				ModelName: "test",
				LiteLLM:   LiteLLMParams{Model: "openai/gpt-4o"},
			},
		},
		RouterSettings: RouterSettings{Strategy: "invalid-strategy"},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for unknown strategy")
	}

	if !strings.Contains(err.Error(), "unknown strategy") {
		t.Errorf("expected 'unknown strategy', got: %v", err)
	}
}

func TestValidate_AllStrategies(t *testing.T) {
	strategies := []string{"simple", "round-robin", "weighted", "least-busy", "latency", "usage"}

	for _, s := range strategies {
		t.Run(s, func(t *testing.T) {
			cfg := &GatewayConfig{
				ModelList: []ModelConfig{
					{
						ModelName: "test",
						LiteLLM:   LiteLLMParams{Model: "openai/gpt-4o"},
					},
				},
				RouterSettings: RouterSettings{Strategy: s},
			}

			if err := Validate(cfg); err != nil {
				t.Errorf("expected no error for strategy %q, got: %v", s, err)
			}
		})
	}
}

func TestValidate_EmptyStrategy(t *testing.T) {
	cfg := &GatewayConfig{
		ModelList: []ModelConfig{
			{
				ModelName: "test",
				LiteLLM:   LiteLLMParams{Model: "openai/gpt-4o"},
			},
		},
		RouterSettings: RouterSettings{Strategy: ""},
	}

	if err := Validate(cfg); err != nil {
		t.Errorf("empty strategy should not error, got: %v", err)
	}
}

func TestValidate_ModelWithoutSlash(t *testing.T) {
	cfg := &GatewayConfig{
		ModelList: []ModelConfig{
			{
				ModelName: "test",
				LiteLLM:   LiteLLMParams{Model: "gpt-4o"},
			},
		},
	}

	// Single-part model name (no slash) should be valid — provider check only applies to provider/model format
	if err := Validate(cfg); err != nil {
		t.Errorf("expected no error for model without slash, got: %v", err)
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	cfg := &GatewayConfig{
		ModelList: []ModelConfig{
			{
				ModelName: "",
				LiteLLM:   LiteLLMParams{Model: ""},
			},
		},
		RouterSettings: RouterSettings{Strategy: "bad"},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for multiple issues")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "model_name is required") {
		t.Error("expected model_name error in output")
	}

	if !strings.Contains(errStr, "model is required") {
		t.Error("expected model error in output")
	}

	if !strings.Contains(errStr, "unknown strategy") {
		t.Error("expected strategy error in output")
	}
}
