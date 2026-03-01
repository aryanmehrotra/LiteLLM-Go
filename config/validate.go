package config

import (
	"fmt"
	"strings"
)

var knownProviders = map[string]bool{
	"openai": true, "anthropic": true, "ollama": true,
	"groq": true, "deepseek": true, "gemini": true,
}

var knownStrategies = map[string]bool{
	"simple": true, "round-robin": true, "weighted": true,
	"least-busy": true, "latency": true, "usage": true,
}

// Validate checks a GatewayConfig for correctness.
func Validate(cfg *GatewayConfig) error {
	var errs []string

	if len(cfg.ModelList) == 0 {
		errs = append(errs, "model_list is empty")
	}

	for i, m := range cfg.ModelList {
		if m.ModelName == "" {
			errs = append(errs, fmt.Sprintf("model_list[%d]: model_name is required", i))
		}

		if m.LiteLLM.Model == "" {
			errs = append(errs, fmt.Sprintf("model_list[%d]: litellm_params.model is required", i))
		} else {
			parts := strings.SplitN(m.LiteLLM.Model, "/", 2)
			if len(parts) == 2 && !knownProviders[parts[0]] {
				errs = append(errs, fmt.Sprintf("model_list[%d]: unknown provider %q in model %q", i, parts[0], m.LiteLLM.Model))
			}
		}
	}

	if s := cfg.RouterSettings.Strategy; s != "" && !knownStrategies[s] {
		errs = append(errs, fmt.Sprintf("router_settings.routing_strategy: unknown strategy %q", s))
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation:\n  - %s", strings.Join(errs, "\n  - "))
	}

	return nil
}
