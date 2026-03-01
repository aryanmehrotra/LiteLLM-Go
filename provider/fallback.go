package provider

import (
	"fmt"

	"gofr.dev/pkg/gofr"

	"examples/llm-gateway/models"
	"examples/llm-gateway/routing"
)

// contextWindowFallbacks maps models that hit token limits to larger-context alternatives.
var contextWindowFallbacks = map[string]string{
	"gpt-4o":                        "gpt-4-turbo",
	"gpt-4o-mini":                   "gpt-4o",
	"gpt-3.5-turbo":                 "gpt-4o",
	"claude-haiku-4-20250414":       "claude-sonnet-4-20250514",
	"claude-3-5-sonnet-20241022":    "claude-sonnet-4-20250514",
	"gemini-2.0-flash-lite":         "gemini-2.0-flash",
	"gemini-2.0-flash":              "gemini-1.5-pro",
	"gemini-1.5-flash":              "gemini-1.5-pro",
	"llama-3.1-8b-instant":          "llama-3.3-70b-versatile",
	"deepseek-chat":                 "deepseek-reasoner",
}

// FallbackProvider tries multiple providers in sequence, falling through on errors.
// It uses error classification to make smart fallback decisions.
type FallbackProvider struct {
	name     string
	chain    []Provider
	cooldown *routing.CooldownTracker
}

// NewFallbackProvider creates a fallback chain from the given providers.
// The first provider is tried first; on error, the next is tried, and so on.
func NewFallbackProvider(name string, chain []Provider) *FallbackProvider {
	return &FallbackProvider{name: name, chain: chain}
}

// SetCooldown sets the cooldown tracker for error-aware fallback decisions.
func (f *FallbackProvider) SetCooldown(cd *routing.CooldownTracker) {
	f.cooldown = cd
}

func (f *FallbackProvider) Name() string { return f.name }

func (f *FallbackProvider) Models() []string {
	if len(f.chain) > 0 {
		return f.chain[0].Models()
	}

	return nil
}

func (f *FallbackProvider) ChatCompletion(ctx *gofr.Context, req models.ChatCompletionRequest) (*models.ChatCompletionResponse, error) {
	var lastErr error

	for _, p := range f.chain {
		// Skip providers in cooldown
		if f.cooldown != nil && !f.cooldown.IsAvailable(p.Name()) {
			ctx.Logf("fallback: provider %s in cooldown, skipping", p.Name())
			continue
		}

		resp, err := p.ChatCompletion(ctx, req)
		if err == nil {
			resp.Provider = p.Name()

			if f.cooldown != nil {
				f.cooldown.RecordSuccess(p.Name())
			}

			return resp, nil
		}

		kind := routing.ClassifyFromError(err)
		ctx.Errorf("fallback: provider %s failed (kind=%s): %v", p.Name(), kind, err)

		if f.cooldown != nil {
			f.cooldown.RecordFailure(p.Name())
		}

		// Error-aware fallback decisions
		switch kind {
		case routing.ErrAuth:
			// Auth errors are config issues — skip retries, try next provider
			ctx.Errorf("fallback: auth error on %s (config issue), trying next", p.Name())

		case routing.ErrTokenLimit:
			// Try a larger-context model variant on the same provider first
			if alt, ok := contextWindowFallbacks[req.Model]; ok {
				ctx.Logf("fallback: token limit on %s/%s, trying larger model %s", p.Name(), req.Model, alt)

				altReq := req
				altReq.Model = alt

				resp, altErr := p.ChatCompletion(ctx, altReq)
				if altErr == nil {
					resp.Provider = p.Name()
					return resp, nil
				}

				ctx.Errorf("fallback: larger model %s/%s also failed: %v", p.Name(), alt, altErr)
			}

		case routing.ErrContentPolicy:
			// Content policy varies by provider — try next (different moderation policy)
			ctx.Logf("fallback: content policy rejection on %s, trying next provider", p.Name())

		case routing.ErrRateLimit:
			// Rate limited — cooldown recorded above, try next provider
			ctx.Logf("fallback: rate limited on %s, trying next provider", p.Name())
		}

		lastErr = err
	}

	return nil, fmt.Errorf("all providers in fallback chain %q failed, last error: %w", f.name, lastErr)
}

// ChatCompletionStream tries each streaming provider in the chain with error-aware fallback.
func (f *FallbackProvider) ChatCompletionStream(ctx *gofr.Context, req models.ChatCompletionRequest, onChunk func(models.StreamChunk)) error {
	var lastErr error

	for _, p := range f.chain {
		// Skip providers in cooldown
		if f.cooldown != nil && !f.cooldown.IsAvailable(p.Name()) {
			ctx.Logf("fallback: provider %s in cooldown, skipping", p.Name())
			continue
		}

		sp, ok := p.(StreamingProvider)
		if !ok {
			ctx.Errorf("fallback: provider %s does not support streaming, trying next", p.Name())
			continue
		}

		err := sp.ChatCompletionStream(ctx, req, onChunk)
		if err == nil {
			if f.cooldown != nil {
				f.cooldown.RecordSuccess(p.Name())
			}

			return nil
		}

		kind := routing.ClassifyFromError(err)
		ctx.Errorf("fallback: provider %s stream failed (kind=%s): %v, trying next", p.Name(), kind, err)

		if f.cooldown != nil {
			f.cooldown.RecordFailure(p.Name())
		}

		// Auth errors are not transient — but still try next provider
		if kind == routing.ErrAuth {
			ctx.Errorf("fallback: auth error on %s (config issue), trying next", p.Name())
		}

		lastErr = err
	}

	return fmt.Errorf("all providers in fallback chain %q failed streaming, last error: %w", f.name, lastErr)
}
