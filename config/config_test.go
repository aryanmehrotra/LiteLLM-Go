package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ValidYAML(t *testing.T) {
	content := `
model_list:
  - model_name: gpt-4o
    litellm_params:
      model: openai/gpt-4o
      api_key: test-key
      timeout: 30000
      weight: 2

router_settings:
  routing_strategy: simple
  num_retries: 3

general_settings:
  cache_ttl_seconds: 300
  model_aliases:
    gpt4: openai/gpt-4o
`
	path := writeTempFile(t, "config-valid.yaml", content)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(cfg.ModelList) != 1 {
		t.Fatalf("expected 1 model, got %d", len(cfg.ModelList))
	}

	if cfg.ModelList[0].ModelName != "gpt-4o" {
		t.Errorf("expected model_name 'gpt-4o', got %q", cfg.ModelList[0].ModelName)
	}

	if cfg.ModelList[0].LiteLLM.Model != "openai/gpt-4o" {
		t.Errorf("expected model 'openai/gpt-4o', got %q", cfg.ModelList[0].LiteLLM.Model)
	}

	if cfg.ModelList[0].LiteLLM.APIKey != "test-key" {
		t.Errorf("expected api_key 'test-key', got %q", cfg.ModelList[0].LiteLLM.APIKey)
	}

	if cfg.ModelList[0].LiteLLM.Timeout != 30000 {
		t.Errorf("expected timeout 30000, got %d", cfg.ModelList[0].LiteLLM.Timeout)
	}

	if cfg.ModelList[0].LiteLLM.Weight != 2 {
		t.Errorf("expected weight 2, got %d", cfg.ModelList[0].LiteLLM.Weight)
	}

	if cfg.RouterSettings.Strategy != "simple" {
		t.Errorf("expected strategy 'simple', got %q", cfg.RouterSettings.Strategy)
	}

	if cfg.RouterSettings.RetryMax != 3 {
		t.Errorf("expected num_retries 3, got %d", cfg.RouterSettings.RetryMax)
	}

	if cfg.GeneralSettings.CacheTTL != 300 {
		t.Errorf("expected cache_ttl 300, got %d", cfg.GeneralSettings.CacheTTL)
	}

	if alias, ok := cfg.GeneralSettings.ModelAliases["gpt4"]; !ok || alias != "openai/gpt-4o" {
		t.Errorf("expected alias gpt4 -> openai/gpt-4o, got %q", alias)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	content := `{invalid yaml: [}`
	path := writeTempFile(t, "config-invalid.yaml", content)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoad_EnvVarResolution(t *testing.T) {
	t.Setenv("TEST_API_KEY", "resolved-key-value")

	content := `
model_list:
  - model_name: test-model
    litellm_params:
      model: openai/gpt-4o
      api_key: os.environ/TEST_API_KEY
`
	path := writeTempFile(t, "config-env.yaml", content)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.ModelList[0].LiteLLM.APIKey != "resolved-key-value" {
		t.Errorf("expected resolved env var, got %q", cfg.ModelList[0].LiteLLM.APIKey)
	}
}

func TestLoad_MasterKeyEnvResolution(t *testing.T) {
	t.Setenv("TEST_MASTER_KEY", "my-master-key")

	content := `
model_list:
  - model_name: test-model
    litellm_params:
      model: openai/gpt-4o

general_settings:
  master_key: os.environ/TEST_MASTER_KEY
`
	path := writeTempFile(t, "config-master.yaml", content)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.GeneralSettings.MasterKey != "my-master-key" {
		t.Errorf("expected 'my-master-key', got %q", cfg.GeneralSettings.MasterKey)
	}
}

func TestResolveEnvVars(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		envKey string
		envVal string
		want   string
	}{
		{
			name:  "plain string passthrough",
			input: "sk-plain-key",
			want:  "sk-plain-key",
		},
		{
			name:   "env var resolved",
			input:  "os.environ/MY_KEY",
			envKey: "MY_KEY",
			envVal: "resolved-value",
			want:   "resolved-value",
		},
		{
			name:  "env var not set returns empty",
			input: "os.environ/UNSET_KEY_12345",
			want:  "",
		},
		{
			name:  "empty string passthrough",
			input: "",
			want:  "",
		},
		{
			name:  "partial match not resolved",
			input: "not-os.environ/KEY",
			want:  "not-os.environ/KEY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envKey != "" {
				t.Setenv(tt.envKey, tt.envVal)
			}

			got := ResolveEnvVars(tt.input)
			if got != tt.want {
				t.Errorf("ResolveEnvVars(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLoad_MultipleModels(t *testing.T) {
	content := `
model_list:
  - model_name: gpt-4o
    litellm_params:
      model: openai/gpt-4o
      api_key: key1
  - model_name: claude
    litellm_params:
      model: anthropic/claude-sonnet-4-20250514
      api_key: key2
  - model_name: llama
    litellm_params:
      model: ollama/llama3.1
`
	path := writeTempFile(t, "config-multi.yaml", content)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(cfg.ModelList) != 3 {
		t.Fatalf("expected 3 models, got %d", len(cfg.ModelList))
	}
}

func TestLoad_ModelInfo(t *testing.T) {
	content := `
model_list:
  - model_name: custom-model
    litellm_params:
      model: openai/gpt-4o
      api_key: key1
    model_info:
      max_tokens: 4096
      input_cost_per_1k_tokens: 0.005
      output_cost_per_1k_tokens: 0.015
`
	path := writeTempFile(t, "config-info.yaml", content)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.ModelList[0].ModelInfo == nil {
		t.Fatal("expected model_info to be set")
	}

	if cfg.ModelList[0].ModelInfo.MaxTokens != 4096 {
		t.Errorf("expected max_tokens 4096, got %d", cfg.ModelList[0].ModelInfo.MaxTokens)
	}

	if cfg.ModelList[0].ModelInfo.InputCostPer1K != 0.005 {
		t.Errorf("expected input cost 0.005, got %f", cfg.ModelList[0].ModelInfo.InputCostPer1K)
	}
}

func TestLoad_Tags(t *testing.T) {
	content := `
model_list:
  - model_name: tagged-model
    litellm_params:
      model: openai/gpt-4o
      api_key: key1
      tags:
        env: production
        tier: premium
`
	path := writeTempFile(t, "config-tags.yaml", content)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	tags := cfg.ModelList[0].LiteLLM.Tags
	if tags == nil {
		t.Fatal("expected tags to be set")
	}

	if tags["env"] != "production" {
		t.Errorf("expected tag env=production, got %q", tags["env"])
	}

	if tags["tier"] != "premium" {
		t.Errorf("expected tag tier=premium, got %q", tags["tier"])
	}
}

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, name)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	return path
}
