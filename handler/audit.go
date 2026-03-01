package handler

import (
	"fmt"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/http/response"
)

// AuditLog handles GET /audit/log with optional query filters.
func (h *AdminHandler) AuditLog() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		if err := h.requireMasterKey(ctx); err != nil {
			return nil, err
		}

		entityType := ctx.Param("entity_type")
		entityID := ctx.Param("entity_id")
		action := ctx.Param("action")
		limit := ctx.Param("limit")
		if limit == "" {
			limit = "100"
		}

		query := "SELECT id, action, entity_type, entity_id, actor_id, details, created_at FROM audit_log WHERE 1=1"
		var args []any
		argIdx := 1

		if entityType != "" {
			query += fmt.Sprintf(" AND entity_type = $%d", argIdx)
			args = append(args, entityType)
			argIdx++
		}

		if entityID != "" {
			query += fmt.Sprintf(" AND entity_id = $%d", argIdx)
			args = append(args, entityID)
			argIdx++
		}

		if action != "" {
			query += fmt.Sprintf(" AND action = $%d", argIdx)
			args = append(args, action)
			argIdx++
		}

		query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", argIdx)
		args = append(args, limit)

		rows, err := ctx.SQL.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("audit log query: %w", err)
		}
		defer rows.Close()

		entries := []map[string]any{}
		for rows.Next() {
			var id int
			var act, entType, entID, actorID, details, createdAt string
			if err := rows.Scan(&id, &act, &entType, &entID, &actorID, &details, &createdAt); err != nil {
				continue
			}
			entries = append(entries, map[string]any{
				"id": id, "action": act, "entity_type": entType, "entity_id": entID,
				"actor_id": actorID, "details": details, "created_at": createdAt,
			})
		}

		return response.Raw{Data: entries}, nil
	}
}
