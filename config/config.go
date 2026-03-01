package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// GatewayConfig is the top-level YAML configuration structure.
type GatewayConfig struct {
	ModelList       []ModelConfig   `yaml:"model_list"`
	RouterSettings  RouterSettings  `yaml:"router_settings,omitempty"`
	GeneralSettings GeneralSettings `yaml:"general_settings,omitempty"`
}

// ModelConfig defines a model deployment in the YAML config.
type ModelConfig struct {
	ModelName  string           `yaml:"model_name"`
	LiteLLM   LiteLLMParams    `yaml:"litellm_params"`
	ModelInfo  *ModelInfoConfig `yaml:"model_info,omitempty"`
}

// LiteLLMParams maps to provider-specific parameters for a model deployment.
type LiteLLMParams struct {
	Model      string            `yaml:"model"`       // e.g. "openai/gpt-4o"
	APIKey     string            `yaml:"api_key"`     // supports "os.environ/OPENAI_API_KEY"
	APIBase    string            `yaml:"api_base,omitempty"`
	Timeout    int               `yaml:"timeout,omitempty"`    // ms
	MaxRetries int               `yaml:"max_retries,omitempty"`
	Weight     int               `yaml:"weight,omitempty"`
	Tags       map[string]string `yaml:"tags,omitempty"`
}

// ModelInfoConfig holds optional metadata for a model.
type ModelInfoConfig struct {
	MaxTokens     int     `yaml:"max_tokens,omitempty"`
	InputCostPer1K  float64 `yaml:"input_cost_per_1k_tokens,omitempty"`
	OutputCostPer1K float64 `yaml:"output_cost_per_1k_tokens,omitempty"`
}

// RouterSettings configures the routing behavior.
type RouterSettings struct {
	Strategy            string  `yaml:"routing_strategy,omitempty"`
	RetryMax            int     `yaml:"num_retries,omitempty"`
	CooldownThreshold   int     `yaml:"cooldown_threshold,omitempty"`
	CooldownPeriodSec   int     `yaml:"cooldown_period_seconds,omitempty"`
	LatencyEMAAlpha     float64 `yaml:"latency_ema_alpha,omitempty"`
	UsageResetPeriodSec int     `yaml:"usage_reset_period_seconds,omitempty"`
	FallbackModels      []string `yaml:"fallback_models,omitempty"`
}

// GeneralSettings holds non-routing configuration.
type GeneralSettings struct {
	MasterKey    string            `yaml:"master_key,omitempty"`
	ModelAliases map[string]string `yaml:"model_aliases,omitempty"`
	CacheTTL     int               `yaml:"cache_ttl_seconds,omitempty"`
}

// Load reads and parses a YAML config file.
func Load(path string) (*GatewayConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg GatewayConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config YAML: %w", err)
	}

	// Resolve environment variable references
	for i := range cfg.ModelList {
		cfg.ModelList[i].LiteLLM.APIKey = ResolveEnvVars(cfg.ModelList[i].LiteLLM.APIKey)
		cfg.ModelList[i].LiteLLM.APIBase = ResolveEnvVars(cfg.ModelList[i].LiteLLM.APIBase)
	}

	cfg.GeneralSettings.MasterKey = ResolveEnvVars(cfg.GeneralSettings.MasterKey)

	return &cfg, nil
}

// ResolveEnvVars replaces "os.environ/VAR_NAME" with the env value.
func ResolveEnvVars(s string) string {
	if strings.HasPrefix(s, "os.environ/") {
		envKey := strings.TrimPrefix(s, "os.environ/")
		return os.Getenv(envKey)
	}

	return s
}
