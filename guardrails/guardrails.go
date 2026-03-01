package guardrails

import (
	"fmt"
	"strings"

	"gofr.dev/pkg/gofr"

	"examples/llm-gateway/models"
)

// GuardrailConfig holds per-key or global guardrail settings.
type GuardrailConfig struct {
	MaxInputTokens  int
	MaxOutputTokens int
	BlockedKeywords []string
	PIIAction       string // "none", "block", "redact", "log"
	Enabled         bool
}

// GlobalConfig holds default guardrail settings loaded from env.
type GlobalConfig struct {
	Enabled         bool
	BlockedKeywords []string
	PIIAction       string
	MaxInputTokens  int
	MaxOutputTokens int
}

// Check runs pre-call guardrails on input messages.
// Returns an error if the request should be blocked.
func Check(cfg *GuardrailConfig, messages []models.Message) error {
	if cfg == nil || !cfg.Enabled {
		return nil
	}

	// Collect all message content
	var contents []string
	for _, m := range messages {
		if m.Content != "" {
			contents = append(contents, m.Content)
		}
	}

	// Keyword check
	if err := CheckKeywords(cfg.BlockedKeywords, contents); err != nil {
		return fmt.Errorf("guardrail: %w", err)
	}

	// PII check (input)
	if cfg.PIIAction == "block" {
		for _, content := range contents {
			if ContainsPII(content) {
				matches := DetectPII(content)
				types := make([]string, 0, len(matches))

				seen := make(map[PIIType]bool)
				for _, m := range matches {
					if !seen[m.Type] {
						types = append(types, string(m.Type))
						seen[m.Type] = true
					}
				}

				return fmt.Errorf("guardrail: input contains PII (%s)", strings.Join(types, ", "))
			}
		}
	}

	// Input length check (approximate token count: ~4 chars per token)
	if cfg.MaxInputTokens > 0 {
		totalChars := 0
		for _, c := range contents {
			totalChars += len(c)
		}

		approxTokens := totalChars / 4
		if approxTokens > cfg.MaxInputTokens {
			return fmt.Errorf("guardrail: input too long (~%d tokens, max %d)", approxTokens, cfg.MaxInputTokens)
		}
	}

	return nil
}

// Filter runs post-call guardrails on the response.
// Modifies the response in-place (e.g., PII redaction) and returns it.
func Filter(ctx *gofr.Context, cfg *GuardrailConfig, resp *models.ChatCompletionResponse) *models.ChatCompletionResponse {
	if cfg == nil || !cfg.Enabled || resp == nil {
		return resp
	}

	// PII redaction/logging on output
	switch cfg.PIIAction {
	case "redact":
		for i := range resp.Choices {
			if ContainsPII(resp.Choices[i].Message.Content) {
				resp.Choices[i].Message.Content = RedactPII(resp.Choices[i].Message.Content)
			}
		}
	case "log":
		for _, choice := range resp.Choices {
			if ContainsPII(choice.Message.Content) {
				matches := DetectPII(choice.Message.Content)

				types := make([]string, 0, len(matches))
				seen := make(map[PIIType]bool)

				for _, m := range matches {
					if !seen[m.Type] {
						types = append(types, string(m.Type))
						seen[m.Type] = true
					}
				}

				ctx.Logf("guardrail: output contains PII (%s)", strings.Join(types, ", "))
			}
		}
	}

	// Output token limit check (approximate; truncation marker)
	if cfg.MaxOutputTokens > 0 {
		for i := range resp.Choices {
			content := resp.Choices[i].Message.Content
			approxTokens := len(content) / 4

			if approxTokens > cfg.MaxOutputTokens {
				maxChars := cfg.MaxOutputTokens * 4
				if maxChars < len(content) {
					resp.Choices[i].Message.Content = content[:maxChars] + "... [truncated by guardrail]"
					resp.Choices[i].FinishReason = "length"
				}
			}
		}
	}

	return resp
}

// LoadConfig loads guardrail config for a specific key hash from DB,
// falling back to global defaults.
func LoadConfig(ctx *gofr.Context, keyHash string, global *GlobalConfig) *GuardrailConfig {
	if global == nil || !global.Enabled {
		return nil
	}

	// Start with global defaults
	cfg := &GuardrailConfig{
		Enabled:         global.Enabled,
		BlockedKeywords: global.BlockedKeywords,
		PIIAction:       global.PIIAction,
		MaxInputTokens:  global.MaxInputTokens,
		MaxOutputTokens: global.MaxOutputTokens,
	}

	if keyHash == "" {
		return cfg
	}

	// Try to load per-key override from DB
	var (
		maxIn, maxOut    int
		keywords, action string
		enabled          bool
	)

	err := ctx.SQL.QueryRowContext(ctx,
		`SELECT max_input_tokens, max_output_tokens, blocked_keywords, pii_action, enabled
		 FROM guardrail_configs WHERE key_hash = $1`, keyHash,
	).Scan(&maxIn, &maxOut, &keywords, &action, &enabled)
	if err != nil {
		return cfg // no per-key config, use global
	}

	// Override with per-key values
	cfg.Enabled = enabled
	if maxIn > 0 {
		cfg.MaxInputTokens = maxIn
	}

	if maxOut > 0 {
		cfg.MaxOutputTokens = maxOut
	}

	if keywords != "" {
		cfg.BlockedKeywords = strings.Split(keywords, ",")
	}

	if action != "" && action != "none" {
		cfg.PIIAction = action
	}

	return cfg
}

// ParseGlobalConfig reads guardrail env vars and returns the global config.
func ParseGlobalConfig(getOrDefault func(string, string) string) *GlobalConfig {
	enabled := getOrDefault("GUARDRAIL_ENABLED", "false") == "true"
	if !enabled {
		return &GlobalConfig{Enabled: false}
	}

	keywords := getOrDefault("GUARDRAIL_BLOCKED_KEYWORDS", "")
	var blocked []string

	if keywords != "" {
		for _, kw := range strings.Split(keywords, ",") {
			kw = strings.TrimSpace(kw)
			if kw != "" {
				blocked = append(blocked, kw)
			}
		}
	}

	maxIn := 0
	maxOut := 0

	if v := getOrDefault("GUARDRAIL_MAX_INPUT_TOKENS", "0"); v != "0" {
		fmt.Sscanf(v, "%d", &maxIn)
	}

	if v := getOrDefault("GUARDRAIL_MAX_OUTPUT_TOKENS", "0"); v != "0" {
		fmt.Sscanf(v, "%d", &maxOut)
	}

	return &GlobalConfig{
		Enabled:         true,
		BlockedKeywords: blocked,
		PIIAction:       getOrDefault("GUARDRAIL_PII_ACTION", "none"),
		MaxInputTokens:  maxIn,
		MaxOutputTokens: maxOut,
	}
}
