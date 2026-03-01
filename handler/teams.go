package handler

import (
	"fmt"
	"time"

	"gofr.dev/pkg/gofr"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/response"

	"examples/llm-gateway/audit"
	"examples/llm-gateway/middleware"
)

// TeamRequest is the request body for team CRUD.
type TeamRequest struct {
	Name      string  `json:"name"`
	OrgID     string  `json:"org_id,omitempty"`
	MaxBudget float64 `json:"max_budget,omitempty"`
}

// CreateTeam handles POST /teams.
func (h *AdminHandler) CreateTeam() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		if err := h.requireMasterKey(ctx); err != nil {
			return nil, err
		}

		var req TeamRequest
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

		var id int
		err := ctx.SQL.QueryRowContext(ctx,
			`INSERT INTO teams (name, org_id, max_budget) VALUES ($1, $2, $3) RETURNING id`,
			req.Name, req.OrgID, req.MaxBudget,
		).Scan(&id)
		if err != nil {
			ctx.Errorf("create team: %v", err)
			return nil, ErrInternal("failed to create team")
		}

		audit.Log(ctx, "create", "team", fmt.Sprintf("%d", id), middleware.GetAuthKey(ctx), req.Name)

		return response.Raw{Data: map[string]any{"id": id, "name": req.Name}}, nil
	}
}

// ListTeams handles GET /teams.
func (h *AdminHandler) ListTeams() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		if err := h.requireMasterKey(ctx); err != nil {
			return nil, err
		}

		query := "SELECT id, name, org_id, max_budget, created_at FROM teams"
		var args []any

		orgID := ctx.PathParam("org_id")
		if orgID == "" {
			orgID = ctx.Param("org_id")
		}

		if orgID != "" {
			query += " WHERE org_id = $1"
			args = append(args, orgID)
		}

		query += " ORDER BY id"

		rows, err := ctx.SQL.QueryContext(ctx, query, args...)
		if err != nil {
			ctx.Errorf("list teams: %v", err)
			return nil, ErrInternal("failed to list teams")
		}
		defer rows.Close()

		teams := []map[string]any{}
		for rows.Next() {
			var id int
			var name, orgID string
			var maxBudget float64
			var createdAt time.Time
			if err := rows.Scan(&id, &name, &orgID, &maxBudget, &createdAt); err != nil {
				continue
			}
			teams = append(teams, map[string]any{"id": id, "name": name, "org_id": orgID, "max_budget": maxBudget, "created_at": createdAt})
		}

		return response.Raw{Data: teams}, nil
	}
}

// DeleteTeam handles DELETE /teams/{id}.
func (h *AdminHandler) DeleteTeam() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		if err := h.requireMasterKey(ctx); err != nil {
			return nil, err
		}

		id := ctx.PathParam("id")
		if id == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"id"}}
		}

		// Check for referencing users or keys
		var refCount int
		_ = ctx.SQL.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM users WHERE team_id = $1", id).Scan(&refCount)

		if refCount > 0 {
			return nil, ErrBadRequest(fmt.Sprintf("cannot delete team: %d user(s) still reference it", refCount))
		}

		_ = ctx.SQL.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM virtual_keys WHERE team_id = $1 AND is_active = TRUE", id).Scan(&refCount)

		if refCount > 0 {
			return nil, ErrBadRequest(fmt.Sprintf("cannot delete team: %d active key(s) still reference it", refCount))
		}

		result, err := ctx.SQL.ExecContext(ctx, "DELETE FROM teams WHERE id = $1", id)
		if err != nil {
			ctx.Errorf("delete team: %v", err)
			return nil, ErrInternal("failed to delete team")
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return nil, ErrNotFound("team")
		}

		audit.Log(ctx, "delete", "team", id, middleware.GetAuthKey(ctx), "")

		return response.Raw{Data: map[string]string{"status": "deleted"}}, nil
	}
}
