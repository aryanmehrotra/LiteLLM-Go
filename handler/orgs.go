package handler

import (
	"fmt"

	"gofr.dev/pkg/gofr"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/response"

	"examples/llm-gateway/audit"
	"examples/llm-gateway/middleware"
)

// OrgRequest is the request body for organization CRUD.
type OrgRequest struct {
	Name       string  `json:"name"`
	AdminEmail string  `json:"admin_email,omitempty"`
	MaxBudget  float64 `json:"max_budget,omitempty"`
}

// CreateOrg handles POST /organizations.
func (h *AdminHandler) CreateOrg() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		if err := h.requireMasterKey(ctx); err != nil {
			return nil, err
		}

		var req OrgRequest
		if err := ctx.Bind(&req); err != nil {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"request body"}}
		}

		if req.Name == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"name"}}
		}

		var id int
		err := ctx.SQL.QueryRowContext(ctx,
			`INSERT INTO organizations (name, admin_email, max_budget) VALUES ($1, $2, $3) RETURNING id`,
			req.Name, req.AdminEmail, req.MaxBudget,
		).Scan(&id)
		if err != nil {
			ctx.Errorf("create org: %v", err)
			return nil, ErrInternal("failed to create organization")
		}

		audit.Log(ctx, "create", "organization", fmt.Sprintf("%d", id), middleware.GetAuthKey(ctx), req.Name)

		return response.Raw{Data: map[string]any{"id": id, "name": req.Name, "admin_email": req.AdminEmail}}, nil
	}
}

// ListOrgs handles GET /organizations.
func (h *AdminHandler) ListOrgs() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		if err := h.requireMasterKey(ctx); err != nil {
			return nil, err
		}

		rows, err := ctx.SQL.QueryContext(ctx, "SELECT id, name, admin_email, max_budget FROM organizations ORDER BY id")
		if err != nil {
			ctx.Errorf("list orgs: %v", err)
			return nil, ErrInternal("failed to list organizations")
		}
		defer rows.Close()

		orgs := []map[string]any{}
		for rows.Next() {
			var id int
			var name, adminEmail string
			var maxBudget float64
			if err := rows.Scan(&id, &name, &adminEmail, &maxBudget); err != nil {
				continue
			}
			orgs = append(orgs, map[string]any{"id": id, "name": name, "admin_email": adminEmail, "max_budget": maxBudget})
		}

		return response.Raw{Data: orgs}, nil
	}
}

// DeleteOrg handles DELETE /organizations/{id}.
func (h *AdminHandler) DeleteOrg() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		if err := h.requireMasterKey(ctx); err != nil {
			return nil, err
		}

		id := ctx.PathParam("id")
		if id == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"id"}}
		}

		// Prevent deleting the last organization — at least one must always exist
		var orgCount int
		_ = ctx.SQL.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM organizations", ).Scan(&orgCount)

		if orgCount <= 1 {
			return nil, ErrBadRequest("cannot delete the last organization; at least one must exist")
		}

		// Check for referencing teams, users, or keys
		var refCount int
		_ = ctx.SQL.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM teams WHERE org_id = $1", id).Scan(&refCount)

		if refCount > 0 {
			return nil, ErrBadRequest(fmt.Sprintf("cannot delete org: %d team(s) still reference it", refCount))
		}

		_ = ctx.SQL.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM users WHERE org_id = $1", id).Scan(&refCount)

		if refCount > 0 {
			return nil, ErrBadRequest(fmt.Sprintf("cannot delete org: %d user(s) still reference it", refCount))
		}

		result, err := ctx.SQL.ExecContext(ctx, "DELETE FROM organizations WHERE id = $1", id)
		if err != nil {
			ctx.Errorf("delete org: %v", err)
			return nil, ErrInternal("failed to delete organization")
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return nil, ErrNotFound("organization")
		}

		audit.Log(ctx, "delete", "organization", id, middleware.GetAuthKey(ctx), "")

		return response.Raw{Data: map[string]string{"status": "deleted"}}, nil
	}
}
