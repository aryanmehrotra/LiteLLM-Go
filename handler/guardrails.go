package handler

import (
	"database/sql"
	"fmt"

	"gofr.dev/pkg/gofr"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/response"

	"aryanmehrotra/litellm-go/audit"
	"aryanmehrotra/litellm-go/middleware"
)

// GuardrailConfigRequest is the request body for guardrail config CRUD.
type GuardrailConfigRequest struct {
	KeyHash         string `json:"key_hash,omitempty"` // empty = global default
	MaxInputTokens  int    `json:"max_input_tokens"`
	MaxOutputTokens int    `json:"max_output_tokens"`
	BlockedKeywords string `json:"blocked_keywords,omitempty"` // comma-separated
	PIIAction       string `json:"pii_action,omitempty"`       // none, block, redact, log
	Enabled         *bool  `json:"enabled,omitempty"`
}

// ListGuardrails handles GET /guardrails — returns all guardrail configs (global + per-key).
func (h *AdminHandler) ListGuardrails() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		if err := h.requireMasterKey(ctx); err != nil {
			return nil, err
		}

		rows, err := ctx.SQL.QueryContext(ctx,
			`SELECT id, key_hash, max_input_tokens, max_output_tokens, blocked_keywords, pii_action, enabled, created_at, updated_at
			 FROM guardrail_configs ORDER BY key_hash NULLS FIRST, id`)
		if err != nil {
			return nil, fmt.Errorf("list guardrails: %w", err)
		}
		defer rows.Close()

		configs := []map[string]any{}
		for rows.Next() {
			var id, maxIn, maxOut int
			var keyHash, keywords, action sql.NullString
			var enabled bool
			var createdAt, updatedAt sql.NullTime

			if err := rows.Scan(&id, &keyHash, &maxIn, &maxOut, &keywords, &action, &enabled, &createdAt, &updatedAt); err != nil {
				continue
			}

			configs = append(configs, map[string]any{
				"id":                id,
				"key_hash":          nullStr(keyHash),
				"max_input_tokens":  maxIn,
				"max_output_tokens": maxOut,
				"blocked_keywords":  nullStr(keywords),
				"pii_action":        nullStr(action),
				"enabled":           enabled,
				"created_at":        nullTime(createdAt),
				"updated_at":        nullTime(updatedAt),
			})
		}

		if configs == nil {
			configs = []map[string]any{}
		}

		return response.Raw{Data: configs}, nil
	}
}

// UpsertGuardrail handles POST /guardrails — creates or updates a guardrail config.
// If key_hash is empty, it creates/updates the global default (key_hash = NULL).
func (h *AdminHandler) UpsertGuardrail() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		if err := h.requireMasterKey(ctx); err != nil {
			return nil, err
		}

		var req GuardrailConfigRequest
		if err := ctx.Bind(&req); err != nil {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"request body"}}
		}

		// Validate pii_action
		validActions := map[string]bool{"none": true, "block": true, "redact": true, "log": true, "": true}
		if !validActions[req.PIIAction] {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"pii_action (must be none, block, redact, or log)"}}
		}

		if req.PIIAction == "" {
			req.PIIAction = "none"
		}

		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}

		// Normalize key_hash: empty string -> NULL for global
		var keyHashParam any
		label := "global"
		if req.KeyHash != "" {
			keyHashParam = req.KeyHash
			label = req.KeyHash[:min(12, len(req.KeyHash))] + "..."
		}

		// Upsert: insert or update on conflict
		var id int
		err := ctx.SQL.QueryRowContext(ctx,
			`INSERT INTO guardrail_configs (key_hash, max_input_tokens, max_output_tokens, blocked_keywords, pii_action, enabled, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP)
			 ON CONFLICT (key_hash) DO UPDATE SET
			   max_input_tokens = EXCLUDED.max_input_tokens,
			   max_output_tokens = EXCLUDED.max_output_tokens,
			   blocked_keywords = EXCLUDED.blocked_keywords,
			   pii_action = EXCLUDED.pii_action,
			   enabled = EXCLUDED.enabled,
			   updated_at = CURRENT_TIMESTAMP
			 RETURNING id`,
			keyHashParam, req.MaxInputTokens, req.MaxOutputTokens,
			req.BlockedKeywords, req.PIIAction, enabled,
		).Scan(&id)
		if err != nil {
			return nil, fmt.Errorf("upsert guardrail config: %w", err)
		}

		audit.Log(ctx, "upsert", "guardrail_config", fmt.Sprintf("%d", id), middleware.GetAuthKey(ctx), label)

		return response.Raw{Data: map[string]any{
			"id":     id,
			"status": "saved",
			"scope":  label,
		}}, nil
	}
}

// DeleteGuardrail handles DELETE /guardrails/{id}.
func (h *AdminHandler) DeleteGuardrail() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		if err := h.requireMasterKey(ctx); err != nil {
			return nil, err
		}

		id := ctx.PathParam("id")
		if id == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"id"}}
		}

		result, err := ctx.SQL.ExecContext(ctx, "DELETE FROM guardrail_configs WHERE id = $1", id)
		if err != nil {
			return nil, fmt.Errorf("delete guardrail config: %w", err)
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return nil, ErrNotFound("guardrail config")
		}

		audit.Log(ctx, "delete", "guardrail_config", id, middleware.GetAuthKey(ctx), "")

		return response.Raw{Data: map[string]string{"status": "deleted"}}, nil
	}
}

func nullStr(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func nullTime(nt sql.NullTime) any {
	if nt.Valid {
		return nt.Time
	}
	return nil
}
