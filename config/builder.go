package config

import (
	"strings"
	"time"

	"gofr.dev/pkg/gofr"

	"aryanmehrotra/litellm-go/provider"
	"aryanmehrotra/litellm-go/routing"
)

// BuildFromConfig creates a Registry and Router from a parsed YAML config.
func BuildFromConfig(cfg *GatewayConfig, app *gofr.App) (*provider.Registry, *routing.Router, error) {
	if err := Validate(cfg); err != nil {
		return nil, nil, err
	}

	// Determine default provider from first model entry
	defaultProvider := "openai"
	if len(cfg.ModelList) > 0 {
		parts := strings.SplitN(cfg.ModelList[0].LiteLLM.Model, "/", 2)
		if len(parts) == 2 {
			defaultProvider = parts[0]
		}
	}

	reg := provider.NewRegistry(defaultProvider)

	// Track which providers have been registered
	registered := make(map[string]bool)

	for _, m := range cfg.ModelList {
		parts := strings.SplitN(m.LiteLLM.Model, "/", 2)
		if len(parts) != 2 {
			continue
		}

		providerName := parts[0]
		timeout := time.Duration(m.LiteLLM.Timeout) * time.Millisecond

		// Register provider if not yet done
		if !registered[providerName] {
			p := createProvider(providerName, m.LiteLLM.APIKey, timeout)
			if p != nil {
				reg.Register(p)
				registered[providerName] = true
			}
		}

		// Register deployment
		p, ok := reg.GetProvider(providerName)
		if !ok {
			continue
		}

		weight := m.LiteLLM.Weight
		if weight <= 0 {
			weight = 1
		}

		reg.RegisterDeployment(routing.Deployment{
			ID:       m.ModelName,
			Provider: p,
			Weight:   weight,
			Metadata: m.LiteLLM.Tags,
		})

		// Register alias: model_name → provider/actual_model
		if m.ModelName != m.LiteLLM.Model {
			reg.RegisterAlias(m.ModelName, m.LiteLLM.Model)
		}
	}

	// Register model aliases from general settings
	for alias, target := range cfg.GeneralSettings.ModelAliases {
		reg.RegisterAlias(alias, target)
	}

	// Build router
	rs := cfg.RouterSettings

	retryMax := rs.RetryMax
	if retryMax <= 0 {
		retryMax = 3
	}

	cooldownThreshold := rs.CooldownThreshold
	if cooldownThreshold <= 0 {
		cooldownThreshold = 5
	}

	cooldownPeriod := rs.CooldownPeriodSec
	if cooldownPeriod <= 0 {
		cooldownPeriod = 60
	}

	retryPolicy := routing.DefaultRetryPolicy(retryMax, 500*time.Millisecond)
	cooldown := routing.NewCooldownTracker(cooldownThreshold, time.Duration(cooldownPeriod)*time.Second)

	alpha := rs.LatencyEMAAlpha
	if alpha <= 0 {
		alpha = 0.2
	}

	usageReset := rs.UsageResetPeriodSec
	if usageReset <= 0 {
		usageReset = 60
	}

	inFlight := routing.NewInFlightTracker()
	latencyTracker := routing.NewLatencyTracker(alpha)
	usageTracker := routing.NewUsageTracker(time.Duration(usageReset) * time.Second)

	strategy := buildStrategyFromConfig(rs.Strategy, inFlight, latencyTracker, usageTracker)
	router := routing.NewRouter(retryPolicy, cooldown, strategy)
	router.InFlight = inFlight
	router.Latency = latencyTracker
	router.Usage = usageTracker

	return reg, router, nil
}

func createProvider(name, apiKey string, timeout time.Duration) provider.Provider {
	switch name {
	case "openai":
		return provider.NewOpenAI(apiKey, timeout)
	case "anthropic":
		return provider.NewAnthropic(apiKey, timeout)
	case "ollama":
		return provider.NewOllama(timeout)
	case "groq":
		return provider.NewGroq(apiKey, timeout)
	case "deepseek":
		return provider.NewDeepSeek(apiKey, timeout)
	case "gemini":
		return provider.NewGemini(apiKey, timeout)
	case "togetherai":
		return provider.NewTogetherAI(apiKey, timeout)
	case "fireworks":
		return provider.NewFireworks(apiKey, timeout)
	case "perplexity":
		return provider.NewPerplexity(apiKey, timeout)
	case "xai":
		return provider.NewXAI(apiKey, timeout)
	case "mistral":
		return provider.NewMistral(apiKey, timeout)
	case "cohere":
		return provider.NewCohere(apiKey, timeout)
	case "cerebras":
		return provider.NewCerebras(apiKey, timeout)
	case "sambanova":
		return provider.NewSambaNova(apiKey, timeout)
	case "ai21":
		return provider.NewAI21(apiKey, timeout)
	case "openrouter":
		return provider.NewOpenRouter(apiKey, timeout)
	case "novita":
		return provider.NewNovita(apiKey, timeout)
	case "nvidia":
		return provider.NewNvidianim(apiKey, timeout)
	default:
		return nil
	}
}

func buildStrategyFromConfig(name string, inFlight *routing.InFlightTracker, latency *routing.LatencyTracker, usage *routing.UsageTracker) routing.Strategy {
	switch name {
	case "round-robin":
		return &routing.RoundRobinStrategy{}
	case "weighted":
		return &routing.WeightedStrategy{Weights: make(map[string]int)}
	case "least-busy":
		return &routing.LeastBusyStrategy{Tracker: inFlight}
	case "latency":
		return &routing.LatencyStrategy{Tracker: latency}
	case "usage":
		return &routing.UsageStrategy{Tracker: usage}
	default:
		return &routing.SimpleStrategy{}
	}
}
