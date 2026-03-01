package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"gofr.dev/pkg/gofr"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/response"

	"aryanmehrotra/llm-gateway/middleware"
)

// GenerateKeyRequest is the request body for POST /key/generate.
type GenerateKeyRequest struct {
	Name          string   `json:"name"`
	TeamID        string   `json:"team_id,omitempty"`
	UserID        string   `json:"user_id,omitempty"`
	OrgID         string   `json:"org_id,omitempty"`
	AllowedModels []string `json:"allowed_models,omitempty"`
	MaxBudget     float64  `json:"max_budget,omitempty"`
	RateLimitRPM  int      `json:"rate_limit_rpm,omitempty"`
	RateLimitTPM  int      `json:"rate_limit_tpm,omitempty"`
	Tier          string   `json:"tier,omitempty"`
	ExpiresInDays int      `json:"expires_in_days,omitempty"`
}

// KeyInfoResponse is the response for key info endpoints.
type KeyInfoResponse struct {
	ID            int       `json:"id"`
	KeyPrefix     string    `json:"key_prefix"`
	Name          string    `json:"name"`
	TeamID        string    `json:"team_id,omitempty"`
	UserID        string    `json:"user_id,omitempty"`
	OrgID         string    `json:"org_id,omitempty"`
	AllowedModels string    `json:"allowed_models,omitempty"`
	MaxBudget     float64   `json:"max_budget"`
	RateLimitRPM  int       `json:"rate_limit_rpm"`
	RateLimitTPM  int       `json:"rate_limit_tpm"`
	Tier          string    `json:"tier"`
	IsActive      bool      `json:"is_active"`
	ExpiresAt     *string   `json:"expires_at,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// KeyGenerate handles POST /key/generate.
func (h *AdminHandler) KeyGenerate() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		if err := h.requireMasterKey(ctx); err != nil {
			return nil, err
		}

		var req GenerateKeyRequest
		if err := ctx.Bind(&req); err != nil {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"request body"}}
		}

		if req.Name == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"name"}}
		}

		// org_id is required
		if req.OrgID == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"org_id"}}
		}

		// Validate org_id exists
		var orgExists int
		if err := ctx.SQL.QueryRowContext(ctx, "SELECT id FROM organizations WHERE id = $1", req.OrgID).Scan(&orgExists); err != nil {
			return nil, ErrInvalidParam("org_id", fmt.Sprintf("org_id %q does not exist", req.OrgID))
		}

		// Cross-validate: if team_id set, team must belong to same org
		if req.TeamID != "" {
			var teamOrgID string
			err := ctx.SQL.QueryRowContext(ctx, "SELECT org_id FROM teams WHERE id = $1", req.TeamID).Scan(&teamOrgID)
			if err != nil {
				return nil, ErrInvalidParam("team_id", fmt.Sprintf("team_id %q does not exist", req.TeamID))
			}

			if teamOrgID != req.OrgID {
				return nil, ErrInvalidParam("team_id", fmt.Sprintf("team %q belongs to org %q, not %q", req.TeamID, teamOrgID, req.OrgID))
			}
		}

		// Cross-validate: if user_id set, user must belong to same org
		if req.UserID != "" {
			var userOrgID string
			err := ctx.SQL.QueryRowContext(ctx, "SELECT org_id FROM users WHERE user_id = $1", req.UserID).Scan(&userOrgID)
			if err != nil {
				return nil, ErrInvalidParam("user_id", fmt.Sprintf("user_id %q does not exist", req.UserID))
			}

			if userOrgID != req.OrgID {
				return nil, ErrInvalidParam("user_id", fmt.Sprintf("user %q belongs to org %q, not %q", req.UserID, userOrgID, req.OrgID))
			}
		}

		// Generate a random key
		rawKey := generateRandomKey()
		keyHash := hashKey(rawKey)
		keyPrefix := rawKey[:12]

		if req.Tier == "" {
			req.Tier = "default"
		}

		allowedModels := strings.Join(req.AllowedModels, ",")

		var expiresAt *time.Time
		if req.ExpiresInDays > 0 {
			t := time.Now().AddDate(0, 0, req.ExpiresInDays)
			expiresAt = &t
		}

		_, err := ctx.SQL.ExecContext(ctx,
			`INSERT INTO virtual_keys (key_hash, key_prefix, name, team_id, user_id, org_id, allowed_models, max_budget, rate_limit_rpm, rate_limit_tpm, tier, expires_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
			keyHash, keyPrefix, req.Name, req.TeamID, req.UserID, req.OrgID,
			allowedModels, req.MaxBudget, req.RateLimitRPM, req.RateLimitTPM, req.Tier, expiresAt,
		)
		if err != nil {
			ctx.Errorf("insert virtual key: %v", err)
			return nil, ErrInternal("failed to create key")
		}

		// Write-through: update in-memory keystore
		h.KeyStore.Add(keyHash)

		return response.Raw{Data: map[string]any{
			"key":        rawKey,
			"key_prefix": keyPrefix,
			"name":       req.Name,
			"expires_at": expiresAt,
		}}, nil
	}
}

// KeyInfo handles GET /key/info?key=sk-...
func (h *AdminHandler) KeyInfo() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		if err := h.requireMasterKey(ctx); err != nil {
			return nil, err
		}

		keyParam := ctx.Param("key")
		if keyParam == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"key"}}
		}

		keyHash := hashKey(keyParam)

		var info KeyInfoResponse
		var expiresAt sql.NullTime

		err := ctx.SQL.QueryRowContext(ctx,
			`SELECT id, key_prefix, name, team_id, user_id, org_id, allowed_models, max_budget, rate_limit_rpm, rate_limit_tpm, tier, is_active, expires_at, created_at
			 FROM virtual_keys WHERE key_hash = $1`, keyHash,
		).Scan(&info.ID, &info.KeyPrefix, &info.Name, &info.TeamID, &info.UserID, &info.OrgID,
			&info.AllowedModels, &info.MaxBudget, &info.RateLimitRPM, &info.RateLimitTPM,
			&info.Tier, &info.IsActive, &expiresAt, &info.CreatedAt)
		if err == sql.ErrNoRows {
			return nil, ErrNotFound("key")
		}

		if err != nil {
			ctx.Errorf("query key info: %v", err)
			return nil, ErrInternal("failed to retrieve key info")
		}

		if expiresAt.Valid {
			s := expiresAt.Time.Format(time.RFC3339)
			info.ExpiresAt = &s
		}

		return response.Raw{Data: info}, nil
	}
}

// KeyDelete handles DELETE /key/{id}.
func (h *AdminHandler) KeyDelete() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		if err := h.requireMasterKey(ctx); err != nil {
			return nil, err
		}

		id := ctx.PathParam("id")
		if id == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"id"}}
		}

		// Look up hash before deactivating so we can remove from in-memory store
		var keyHash string
		_ = ctx.SQL.QueryRowContext(ctx,
			"SELECT key_hash FROM virtual_keys WHERE id = $1", id,
		).Scan(&keyHash)

		_, err := ctx.SQL.ExecContext(ctx,
			"UPDATE virtual_keys SET is_active = FALSE, updated_at = CURRENT_TIMESTAMP WHERE id = $1", id)
		if err != nil {
			ctx.Errorf("deactivate key: %v", err)
			return nil, ErrInternal("failed to deactivate key")
		}

		if keyHash != "" {
			h.KeyStore.Remove(keyHash)
		}

		return response.Raw{Data: map[string]string{"status": "deleted"}}, nil
	}
}

// KeyRotate handles POST /key/{id}/rotate — deactivates old key, generates new one.
func (h *AdminHandler) KeyRotate() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		if err := h.requireMasterKey(ctx); err != nil {
			return nil, err
		}

		id := ctx.PathParam("id")
		if id == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"id"}}
		}

		// Get old key hash + info
		var oldKeyHash string
		_ = ctx.SQL.QueryRowContext(ctx,
			"SELECT key_hash FROM virtual_keys WHERE id = $1", id,
		).Scan(&oldKeyHash)

		var name, teamID, userID, orgID, allowedModels, tier string
		var maxBudget float64
		var rpmLimit, tpmLimit int

		err := ctx.SQL.QueryRowContext(ctx,
			`SELECT name, team_id, user_id, org_id, allowed_models, max_budget, rate_limit_rpm, rate_limit_tpm, tier
			 FROM virtual_keys WHERE id = $1 AND is_active = TRUE`, id,
		).Scan(&name, &teamID, &userID, &orgID, &allowedModels, &maxBudget, &rpmLimit, &tpmLimit, &tier)
		if err == sql.ErrNoRows {
			return nil, ErrNotFound("key (not found or inactive)")
		}

		if err != nil {
			ctx.Errorf("query key for rotation: %v", err)
			return nil, ErrInternal("failed to retrieve key for rotation")
		}

		// Deactivate old key
		_, _ = ctx.SQL.ExecContext(ctx,
			"UPDATE virtual_keys SET is_active = FALSE, updated_at = CURRENT_TIMESTAMP WHERE id = $1", id)

		// Generate new key
		rawKey := generateRandomKey()
		keyHash := hashKey(rawKey)
		keyPrefix := rawKey[:12]

		_, err = ctx.SQL.ExecContext(ctx,
			`INSERT INTO virtual_keys (key_hash, key_prefix, name, team_id, user_id, org_id, allowed_models, max_budget, rate_limit_rpm, rate_limit_tpm, tier)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			keyHash, keyPrefix, name, teamID, userID, orgID, allowedModels, maxBudget, rpmLimit, tpmLimit, tier,
		)
		if err != nil {
			ctx.Errorf("insert rotated key: %v", err)
			return nil, ErrInternal("failed to create rotated key")
		}

		// Write-through: remove old, add new
		if oldKeyHash != "" {
			h.KeyStore.Remove(oldKeyHash)
		}

		h.KeyStore.Add(keyHash)

		return response.Raw{Data: map[string]any{
			"key":          rawKey,
			"key_prefix":   keyPrefix,
			"name":         name,
			"rotated_from": id,
		}}, nil
	}
}

// KeySelf handles GET /key/self — returns info for the currently authenticated virtual key.
// No master key required; works for any valid virtual key holder.
func (h *AdminHandler) KeySelf() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		authKey := middleware.GetAuthKey(ctx)
		if authKey == "" {
			return nil, ErrUnauthorized("API key required")
		}

		keyHash := hashKey(authKey)

		var info KeyInfoResponse
		var expiresAt sql.NullTime

		err := ctx.SQL.QueryRowContext(ctx,
			`SELECT id, key_prefix, name, team_id, user_id, org_id, allowed_models, max_budget, rate_limit_rpm, rate_limit_tpm, tier, is_active, expires_at, created_at
			 FROM virtual_keys WHERE key_hash = $1`, keyHash,
		).Scan(&info.ID, &info.KeyPrefix, &info.Name, &info.TeamID, &info.UserID, &info.OrgID,
			&info.AllowedModels, &info.MaxBudget, &info.RateLimitRPM, &info.RateLimitTPM,
			&info.Tier, &info.IsActive, &expiresAt, &info.CreatedAt)
		if err == sql.ErrNoRows {
			return nil, ErrNotFound("key")
		}

		if err != nil {
			ctx.Errorf("query key self info: %v", err)
			return nil, ErrInternal("failed to retrieve key info")
		}

		if expiresAt.Valid {
			s := expiresAt.Time.Format(time.RFC3339)
			info.ExpiresAt = &s
		}

		return response.Raw{Data: info}, nil
	}
}

// ListKeys handles GET /keys — lists all virtual keys (admin only).
func (h *AdminHandler) ListKeys() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		if err := h.requireMasterKey(ctx); err != nil {
			return nil, err
		}

		query := `SELECT id, key_prefix, name, team_id, org_id, tier, is_active, rate_limit_rpm, created_at
			 FROM virtual_keys WHERE is_active = TRUE`
		var args []any

		orgID := ctx.PathParam("org_id")
		if orgID == "" {
			orgID = ctx.Param("org_id")
		}

		if orgID != "" {
			args = append(args, orgID)
			query += fmt.Sprintf(" AND org_id = $%d", len(args))
		}

		query += " ORDER BY created_at DESC"

		rows, err := ctx.SQL.QueryContext(ctx, query, args...)
		if err != nil {
			ctx.Errorf("list keys: %v", err)
			return nil, ErrInternal("failed to list keys")
		}
		defer rows.Close()

		var keys []map[string]any

		for rows.Next() {
			var id, rpmLimit int
			var keyPrefix, name, teamID, orgID, tier string
			var isActive bool
			var createdAt time.Time

			if err := rows.Scan(&id, &keyPrefix, &name, &teamID, &orgID, &tier, &isActive, &rpmLimit, &createdAt); err != nil {
				continue
			}

			keys = append(keys, map[string]any{
				"id":             id,
				"key_prefix":     keyPrefix,
				"name":           name,
				"team_id":        teamID,
				"org_id":         orgID,
				"tier":           tier,
				"is_active":      isActive,
				"rate_limit_rpm": rpmLimit,
				"created_at":     createdAt,
			})
		}

		if err := rows.Err(); err != nil {
			ctx.Errorf("iterate keys: %v", err)
			return nil, ErrInternal("failed to list keys")
		}

		if keys == nil {
			keys = []map[string]any{}
		}

		return response.Raw{Data: keys}, nil
	}
}

func generateRandomKey() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)

	return "sk-" + hex.EncodeToString(b)
}

func hashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}
