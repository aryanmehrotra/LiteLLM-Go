package handler

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"

	"gofr.dev/pkg/gofr"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/response"

	"aryanmehrotra/litellm-go/budget"
	"aryanmehrotra/litellm-go/cost"
	"aryanmehrotra/litellm-go/guardrails"
	"aryanmehrotra/litellm-go/middleware"
	"aryanmehrotra/litellm-go/models"
	"aryanmehrotra/litellm-go/provider"
)

// ChatCompletion handles POST /v1/chat/completions.
// It resolves the provider from the model prefix, checks the cache,
// routes through the Router (retry + cooldown), caches the response,
// and returns an OpenAI-compatible JSON response.
func (h *APIHandler) ChatCompletion() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		var req models.ChatCompletionRequest
		if err := ctx.Bind(&req); err != nil {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"request body"}}
		}

		if len(req.Messages) == 0 {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"messages"}}
		}

		if req.Model == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"model"}}
		}

		if req.Stream {
			return nil, ErrBadRequest("streaming not supported on this endpoint; use WebSocket endpoint /v1/chat/completions/stream instead")
		}

		// Validate tools if present
		if len(req.Tools) > 0 {
			if err := ValidateTools(req.Tools); err != nil {
				return nil, ErrInvalidParam("tools", err.Error())
			}
		}

		// Per-key enforcement: blocked users, model restrictions, rate limits, budgets
		authKey := middleware.GetAuthKey(ctx)
		keyHash := ""

		if authKey != "" {
			keyHash = sha256hex(authKey)

			if err := checkBlockedUser(ctx, keyHash); err != nil {
				return nil, err
			}

			if err := checkAllowedModels(ctx, keyHash, req.Model); err != nil {
				return nil, err
			}

			vk := loadVirtualKey(ctx, keyHash)
			if vk != nil {
				if err := middleware.CheckRateLimit(ctx, keyHash, vk.RateLimitRPM); err != nil {
					return nil, ErrRateLimit(err.Error())
				}
			}

			if ok, _ := budget.CheckBudget(ctx, "key", keyHash); !ok {
				return nil, ErrBudgetExceeded()
			}
		}

		// Pre-call guardrails
		grCfg := guardrails.LoadConfig(ctx, keyHash, h.Guardrails)
		if err := guardrails.Check(grCfg, req.Messages); err != nil {
			return nil, ErrGuardrail(err.Error())
		}

		// Web search augmentation — single service call, non-fatal
		var searchAnnotations []models.Annotation
		if h.Search != nil {
			searchAnnotations = h.Search.Augment(ctx, &req)
		}

		// Check per-key fallback override
		if kc := middleware.GetKeyConfig(ctx); kc != nil && len(kc.FallbackChain) > 0 {
			fb := h.Registry.BuildFallbackChain(kc.FallbackChain, h.Router.Cooldown)
			if fb != nil {
				_, modelName, err := h.Registry.ResolveProvider(req.Model)
				if err != nil {
					return nil, ErrInvalidParam("model", fmt.Sprintf("model %q not found", req.Model))
				}

				req.Model = modelName

				resp, err := fb.ChatCompletion(ctx, req)
				if err != nil {
					ctx.Errorf("per-key fallback error: %v", err)
					return nil, err
				}

				h.Cache.Set(ctx, &req, resp)

				return response.Raw{Data: resp}, nil
			}
		}

		// Resolve provider
		p, modelName, err := h.Registry.ResolveProvider(req.Model)
		if err != nil {
			return nil, ErrInvalidParam("model", fmt.Sprintf("model %q not found", req.Model))
		}

		// Use the cleaned model name for the upstream call
		req.Model = modelName

		// Strip unsupported params or inject tools as prompt for non-tool models
		if len(req.Tools) > 0 {
			if !provider.HasCapability(modelName, provider.CapTools) {
				provider.InjectToolsAsPrompt(&req)
			} else {
				provider.StripUnsupportedParams(modelName, &req)
			}
		}

		// Check cache
		if cached, found := h.Cache.Get(ctx, &req); found {
			cached.Cached = true
			return response.Raw{Data: cached}, nil
		}

		// Route through Router (retry + cooldown)
		resp, err := h.Router.ChatCompletion(ctx, p, modelName, req)
		if err != nil {
			ctx.Errorf("provider %s error: %v", p.Name(), err)
			return nil, err
		}

		// Post-call guardrails (PII redaction, output length)
		resp = guardrails.Filter(ctx, grCfg, resp)

		// Compute cost and record metrics
		resp.Cost = cost.Calculate(modelName, resp.Usage)
		cost.RecordCost(ctx, p.Name(), modelName, resp.Cost, resp.Usage.TotalTokens)

		// Record TPM usage
		if authKey != "" {
			vk := loadVirtualKey(ctx, keyHash)
			if vk != nil && resp.Usage.TotalTokens > 0 {
				_ = middleware.RecordTokenUsage(ctx, keyHash, resp.Usage.TotalTokens, vk.RateLimitTPM)
			}

			// Record spend for budget tracking
			_ = budget.RecordSpend(ctx, keyHash, "", "", "", p.Name(), modelName,
				resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens, resp.Cost)

			// Add rate limit info to response
			if vk != nil && vk.RateLimitRPM > 0 {
				rlInfo := middleware.GetRateLimitInfo(ctx, keyHash, vk.RateLimitRPM)
				if rlInfo != nil {
					resp.RateLimit = &models.RateLimitInfo{
						Limit:     rlInfo.Limit,
						Remaining: rlInfo.Remaining,
						ResetAt:   rlInfo.ResetAt,
					}
				}
			}
		}

		// Attach web search annotations
		if len(searchAnnotations) > 0 {
			resp.Annotations = searchAnnotations
		}

		// Cache the response
		h.Cache.Set(ctx, &req, resp)

		return response.Raw{Data: resp}, nil
	}
}

// ListModels handles GET /v1/models.
func (h *APIHandler) ListModels() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		modelList := h.Registry.ListModels()

		return response.Raw{Data: models.ModelListResponse{
			Object: "list",
			Data:   modelList,
		}}, nil
	}
}

// Health handles GET /health.
func (h *APIHandler) Health() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		return response.Raw{Data: map[string]any{
			"status": "ok",
			"models": len(h.Registry.ListModels()),
		}}, nil
	}
}

// virtualKeyInfo holds fields loaded from the virtual_keys table.
type virtualKeyInfo struct {
	AllowedModels string
	RateLimitRPM  int
	RateLimitTPM  int
}

// loadVirtualKey loads virtual key info from the DB. Returns nil if not found or DB not configured.
func loadVirtualKey(ctx *gofr.Context, keyHash string) *virtualKeyInfo {
	var vk virtualKeyInfo

	err := ctx.SQL.QueryRowContext(ctx,
		"SELECT allowed_models, rate_limit_rpm, rate_limit_tpm FROM virtual_keys WHERE key_hash = $1 AND is_active = TRUE",
		keyHash,
	).Scan(&vk.AllowedModels, &vk.RateLimitRPM, &vk.RateLimitTPM)
	if err != nil {
		return nil
	}

	return &vk
}

// checkAllowedModels verifies the requested model is in the key's allowed list.
func checkAllowedModels(ctx *gofr.Context, keyHash, model string) error {
	var allowedModels string

	err := ctx.SQL.QueryRowContext(ctx,
		"SELECT allowed_models FROM virtual_keys WHERE key_hash = $1 AND is_active = TRUE",
		keyHash,
	).Scan(&allowedModels)
	if err == sql.ErrNoRows || err != nil {
		return nil // no key found or DB error = skip check
	}

	if allowedModels == "" {
		return nil // empty = all models allowed
	}

	for _, m := range strings.Split(allowedModels, ",") {
		if strings.TrimSpace(m) == model || strings.TrimSpace(m) == strings.Split(model, "/")[0]+"/*" {
			return nil
		}
	}

	return ErrModelNotAllowed(model)
}

// checkBlockedUser checks if the key's associated user is blocked.
func checkBlockedUser(ctx *gofr.Context, keyHash string) error {
	// Look up user_id from virtual_keys
	var userID string

	err := ctx.SQL.QueryRowContext(ctx,
		"SELECT user_id FROM virtual_keys WHERE key_hash = $1 AND is_active = TRUE",
		keyHash,
	).Scan(&userID)
	if err != nil || userID == "" {
		return nil // no key or no user = skip check
	}

	var blockedID int

	err = ctx.SQL.QueryRowContext(ctx,
		"SELECT id FROM blocked_users WHERE user_id = $1",
		userID,
	).Scan(&blockedID)
	if err == sql.ErrNoRows || err != nil {
		return nil
	}

	return ErrUserBlocked()
}

// sha256hex returns the hex-encoded SHA-256 hash of the input string.
func sha256hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// HealthProviders handles GET /health/providers.
// Returns per-provider status showing which are configured and available.
func (h *APIHandler) HealthProviders() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		type providerStatus struct {
			Name       string `json:"name"`
			Configured bool   `json:"configured"`
			Models     int    `json:"models"`
		}

		var providers []providerStatus
		for _, name := range h.Registry.ProviderNames() {
			p, _, _ := h.Registry.ResolveProvider(name + "/dummy")
			modelCount := 0
			if p != nil {
				modelCount = len(p.Models())
			}
			providers = append(providers, providerStatus{
				Name:       name,
				Configured: p != nil,
				Models:     modelCount,
			})
		}

		return response.Raw{Data: map[string]any{
			"status":    "ok",
			"providers": providers,
		}}, nil
	}
}
