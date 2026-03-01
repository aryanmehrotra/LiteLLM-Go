package handler

import (
	"fmt"

	"gofr.dev/pkg/gofr"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/response"

	"examples/llm-gateway/audit"
	"examples/llm-gateway/middleware"
)

// UserRequest is the request body for user CRUD.
type UserRequest struct {
	UserID    string  `json:"user_id"`
	Email     string  `json:"email,omitempty"`
	TeamID    string  `json:"team_id,omitempty"`
	OrgID     string  `json:"org_id,omitempty"`
	Role      string  `json:"role,omitempty"` // "admin" or "member" (default: "member")
	MaxBudget float64 `json:"max_budget,omitempty"`
}

// CreateUser handles POST /users.
func (h *AdminHandler) CreateUser() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		if err := h.requireMasterKey(ctx); err != nil {
			return nil, err
		}

		var req UserRequest
		if err := ctx.Bind(&req); err != nil {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"request body"}}
		}

		if req.UserID == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"user_id"}}
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

		// Validate role
		if req.Role == "" {
			req.Role = "member"
		}

		if req.Role != "admin" && req.Role != "member" {
			return nil, ErrInvalidParam("role", "role must be \"admin\" or \"member\"")
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

		var id int
		err := ctx.SQL.QueryRowContext(ctx,
			`INSERT INTO users (user_id, email, team_id, org_id, role, max_budget) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
			req.UserID, req.Email, req.TeamID, req.OrgID, req.Role, req.MaxBudget,
		).Scan(&id)
		if err != nil {
			ctx.Errorf("create user: %v", err)
			return nil, ErrInternal("failed to create user")
		}

		audit.Log(ctx, "create", "user", req.UserID, middleware.GetAuthKey(ctx), "")

		return response.Raw{Data: map[string]any{"id": id, "user_id": req.UserID, "role": req.Role}}, nil
	}
}

// ListUsers handles GET /users.
func (h *AdminHandler) ListUsers() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		if err := h.requireMasterKey(ctx); err != nil {
			return nil, err
		}

		query := "SELECT id, user_id, email, team_id, org_id, role, max_budget FROM users"
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
			ctx.Errorf("list users: %v", err)
			return nil, ErrInternal("failed to list users")
		}
		defer rows.Close()

		users := []map[string]any{}
		for rows.Next() {
			var id int
			var userID, email, teamID, orgID, role string
			var maxBudget float64
			if err := rows.Scan(&id, &userID, &email, &teamID, &orgID, &role, &maxBudget); err != nil {
				continue
			}
			users = append(users, map[string]any{"id": id, "user_id": userID, "email": email, "team_id": teamID, "org_id": orgID, "role": role, "max_budget": maxBudget})
		}

		return response.Raw{Data: users}, nil
	}
}

// DeleteUser handles DELETE /users/{id}.
func (h *AdminHandler) DeleteUser() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		if err := h.requireMasterKey(ctx); err != nil {
			return nil, err
		}

		id := ctx.PathParam("id")
		if id == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"id"}}
		}

		result, err := ctx.SQL.ExecContext(ctx, "DELETE FROM users WHERE id = $1", id)
		if err != nil {
			ctx.Errorf("delete user: %v", err)
			return nil, ErrInternal("failed to delete user")
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return nil, ErrNotFound("user")
		}

		audit.Log(ctx, "delete", "user", id, middleware.GetAuthKey(ctx), "")

		return response.Raw{Data: map[string]string{"status": "deleted"}}, nil
	}
}
