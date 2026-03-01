package handler

import (
	"database/sql"
	"strings"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/http/response"
)

// SettingsConfig holds the gateway config references for the settings endpoint.
type SettingsConfig struct {
	GetOrDefault func(string, string) string
}

// Settings handles GET /settings — returns current gateway configuration for display.
// API keys are masked for security. Only accessible with master key.
func (h *AdminHandler) Settings(cfg *SettingsConfig) gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		if err := h.requireMasterKey(ctx); err != nil {
			return nil, err
		}

		get := cfg.GetOrDefault

		// Check DB for saved provider configs (overrides env)
		dbProviders := loadProviderConfigsFromDB(ctx)

		providerStatus := map[string]any{
			"openai":      map[string]any{"configured": isConfigured(get("OPENAI_API_KEY", ""), dbProviders["openai"]), "base_url": get("OPENAI_BASE_URL", "https://api.openai.com"), "timeout_ms": get("OPENAI_TIMEOUT_MS", "0")},
			"anthropic":   map[string]any{"configured": isConfigured(get("ANTHROPIC_API_KEY", ""), dbProviders["anthropic"]), "timeout_ms": get("ANTHROPIC_TIMEOUT_MS", "0")},
			"groq":        map[string]any{"configured": isConfigured(get("GROQ_API_KEY", ""), dbProviders["groq"]), "timeout_ms": get("GROQ_TIMEOUT_MS", "0")},
			"deepseek":    map[string]any{"configured": isConfigured(get("DEEPSEEK_API_KEY", ""), dbProviders["deepseek"]), "timeout_ms": get("DEEPSEEK_TIMEOUT_MS", "0")},
			"gemini":      map[string]any{"configured": isConfigured(get("GEMINI_API_KEY", ""), dbProviders["gemini"]), "timeout_ms": get("GEMINI_TIMEOUT_MS", "0")},
			"ollama":      map[string]any{"configured": true, "base_url": get("OLLAMA_BASE_URL", "http://localhost:11434"), "timeout_ms": get("OLLAMA_TIMEOUT_MS", "0")},
			"togetherai":  map[string]any{"configured": isConfigured(get("TOGETHER_API_KEY", ""), dbProviders["togetherai"]), "timeout_ms": get("TOGETHER_TIMEOUT_MS", "0")},
			"fireworks":   map[string]any{"configured": isConfigured(get("FIREWORKS_API_KEY", ""), dbProviders["fireworks"]), "timeout_ms": get("FIREWORKS_TIMEOUT_MS", "0")},
			"perplexity":  map[string]any{"configured": isConfigured(get("PERPLEXITY_API_KEY", ""), dbProviders["perplexity"]), "timeout_ms": get("PERPLEXITY_TIMEOUT_MS", "0")},
			"xai":         map[string]any{"configured": isConfigured(get("XAI_API_KEY", ""), dbProviders["xai"]), "timeout_ms": get("XAI_TIMEOUT_MS", "0")},
			"mistral":     map[string]any{"configured": isConfigured(get("MISTRAL_API_KEY", ""), dbProviders["mistral"]), "timeout_ms": get("MISTRAL_TIMEOUT_MS", "0")},
			"cohere":      map[string]any{"configured": isConfigured(get("COHERE_API_KEY", ""), dbProviders["cohere"]), "timeout_ms": get("COHERE_TIMEOUT_MS", "0")},
			"azure":       map[string]any{"configured": isConfigured(get("AZURE_OPENAI_API_KEY", ""), dbProviders["azure"]), "timeout_ms": get("AZURE_TIMEOUT_MS", "0")},
			"bedrock":     map[string]any{"configured": isConfigured(get("AWS_ACCESS_KEY_ID", ""), dbProviders["bedrock"]), "timeout_ms": get("BEDROCK_TIMEOUT_MS", "0")},
			"cerebras":    map[string]any{"configured": isConfigured(get("CEREBRAS_API_KEY", ""), dbProviders["cerebras"]), "timeout_ms": get("CEREBRAS_TIMEOUT_MS", "0")},
			"sambanova":   map[string]any{"configured": isConfigured(get("SAMBANOVA_API_KEY", ""), dbProviders["sambanova"]), "timeout_ms": get("SAMBANOVA_TIMEOUT_MS", "0")},
			"ai21":        map[string]any{"configured": isConfigured(get("AI21_API_KEY", ""), dbProviders["ai21"]), "timeout_ms": get("AI21_TIMEOUT_MS", "0")},
			"openrouter":  map[string]any{"configured": isConfigured(get("OPENROUTER_API_KEY", ""), dbProviders["openrouter"]), "timeout_ms": get("OPENROUTER_TIMEOUT_MS", "0")},
			"novita":      map[string]any{"configured": isConfigured(get("NOVITA_API_KEY", ""), dbProviders["novita"]), "timeout_ms": get("NOVITA_TIMEOUT_MS", "0")},
			"nvidia":      map[string]any{"configured": isConfigured(get("NVIDIA_API_KEY", ""), dbProviders["nvidia"]), "timeout_ms": get("NVIDIA_TIMEOUT_MS", "0")},
			"cloudflare":  map[string]any{"configured": isConfigured(get("CLOUDFLARE_API_TOKEN", ""), dbProviders["cloudflare"]), "timeout_ms": get("CLOUDFLARE_TIMEOUT_MS", "0")},
			"vertex":      map[string]any{"configured": isConfigured(get("VERTEX_PROJECT", ""), dbProviders["vertex"]), "timeout_ms": get("VERTEX_TIMEOUT_MS", "0")},
			"huggingface": map[string]any{"configured": isConfigured(get("HUGGINGFACE_API_KEY", ""), dbProviders["huggingface"]), "timeout_ms": get("HUGGINGFACE_TIMEOUT_MS", "0")},
		}

		settings := map[string]any{
			"providers": providerStatus,
			"routing": map[string]any{
				"default_provider":    get("DEFAULT_PROVIDER", "openai"),
				"strategy":            get("ROUTING_STRATEGY", "simple"),
				"retry_max":           get("RETRY_MAX", "3"),
				"retry_backoff_ms":    get("RETRY_BACKOFF_BASE_MS", "500"),
				"cooldown_threshold":  get("COOLDOWN_THRESHOLD", "5"),
				"cooldown_period_sec": get("COOLDOWN_PERIOD_SECONDS", "60"),
				"cb_threshold":        get("CB_THRESHOLD", "5"),
				"cb_interval_sec":     get("CB_INTERVAL_SECONDS", "30"),
				"fallback_chain":      get("FALLBACK_CHAIN", ""),
				"queue_enabled":       get("REQUEST_QUEUE_ENABLED", "false"),
				"queue_size":          get("REQUEST_QUEUE_SIZE", "100"),
			},
			"guardrails": map[string]any{
				"enabled":           get("GUARDRAIL_ENABLED", "false"),
				"pii_action":        get("GUARDRAIL_PII_ACTION", "none"),
				"blocked_keywords":  get("GUARDRAIL_BLOCKED_KEYWORDS", ""),
				"max_input_tokens":  get("GUARDRAIL_MAX_INPUT_TOKENS", "0"),
				"max_output_tokens": get("GUARDRAIL_MAX_OUTPUT_TOKENS", "0"),
			},
			"cache": map[string]any{
				"ttl_seconds": get("CACHE_TTL_SECONDS", "300"),
			},
			"batch": map[string]any{
				"workers":          get("BATCH_WORKERS", "5"),
				"task_timeout_sec": get("BATCH_TASK_TIMEOUT_SECONDS", "120"),
			},
			"auth": map[string]any{
				"gateway_keys_configured": get("GATEWAY_API_KEYS", "") != "",
				"master_key_configured":   get("GATEWAY_MASTER_KEY", "") != "",
				"key_count":               countKeys(get("GATEWAY_API_KEYS", "")),
			},
			"websearch": map[string]any{
				"enabled":     get("WEBSEARCH_ENABLED", "false"),
				"provider":    get("WEBSEARCH_PROVIDER", "searxng"),
				"base_url":    get("WEBSEARCH_BASE_URL", "http://localhost:8080"),
				"max_results": get("WEBSEARCH_MAX_RESULTS", "5"),
				"cache_ttl":   get("WEBSEARCH_CACHE_TTL", "300"),
				"api_key_set": get("WEBSEARCH_API_KEY", "") != "",
				"query_model": get("WEBSEARCH_QUERY_MODEL", ""),
			},
		}

		return response.Raw{Data: settings}, nil
	}
}

// SaveProviderConfig handles PUT /settings/providers — saves provider API keys to the database.
func (h *AdminHandler) SaveProviderConfig() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		if err := h.requireMasterKey(ctx); err != nil {
			return nil, err
		}

		var req map[string]string
		if err := ctx.Bind(&req); err != nil {
			return nil, ErrBadRequest("invalid request body")
		}

		// Map from request field names to provider names
		keyMap := map[string]string{
			"openai_api_key":    "openai",
			"anthropic_api_key": "anthropic",
			"groq_api_key":     "groq",
			"deepseek_api_key":  "deepseek",
			"gemini_api_key":    "gemini",
			"ollama_base_url":   "ollama",
		}

		saved := 0

		for field, providerName := range keyMap {
			val, ok := req[field]
			if !ok || val == "" {
				continue
			}

			isURL := strings.HasSuffix(field, "_url")
			var apiKey, baseURL sql.NullString

			if isURL {
				baseURL = sql.NullString{String: val, Valid: true}
			} else {
				apiKey = sql.NullString{String: val, Valid: true}
			}

			_, err := ctx.SQL.ExecContext(ctx,
				`INSERT INTO provider_config (provider_name, api_key, base_url)
				 VALUES ($1, $2, $3)
				 ON CONFLICT (provider_name) DO UPDATE SET
				   api_key = COALESCE($2, provider_config.api_key),
				   base_url = COALESCE($3, provider_config.base_url),
				   updated_at = CURRENT_TIMESTAMP`,
				providerName, apiKey, baseURL)
			if err != nil {
				ctx.Errorf("save provider config %s: %v", providerName, err)
				continue
			}

			saved++
		}

		return response.Raw{Data: map[string]any{
			"saved":   saved,
			"message": "Provider config saved. Restart gateway to apply changes.",
		}}, nil
	}
}

func countKeys(keys string) int {
	if keys == "" {
		return 0
	}

	return len(strings.Split(keys, ","))
}

// loadProviderConfigsFromDB loads saved provider configs from the database.
// Returns a map of provider_name -> api_key (used to check if configured).
func loadProviderConfigsFromDB(ctx *gofr.Context) map[string]string {
	result := make(map[string]string)

	rows, err := ctx.SQL.QueryContext(ctx,
		"SELECT provider_name, api_key FROM provider_config WHERE api_key IS NOT NULL AND api_key != ''")
	if err != nil {
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var name, key string
		if err := rows.Scan(&name, &key); err == nil {
			result[name] = key
		}
	}

	return result
}

// isConfigured returns true if either the env key or DB key is set.
func isConfigured(envKey string, dbKey string) bool {
	return (envKey != "" && !strings.HasPrefix(strings.ToLower(envKey), "your-")) || dbKey != ""
}
