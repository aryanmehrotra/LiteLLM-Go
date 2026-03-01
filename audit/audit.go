package audit

import (
	"gofr.dev/pkg/gofr"
)

// Log inserts an audit log entry.
func Log(ctx *gofr.Context, action, entityType, entityID, actorID, details string) {
	_, err := ctx.SQL.ExecContext(ctx,
		`INSERT INTO audit_log (action, entity_type, entity_id, actor_id, details)
		 VALUES ($1, $2, $3, $4, $5)`,
		action, entityType, entityID, actorID, details,
	)
	if err != nil {
		ctx.Errorf("audit log error: %v", err)
	}
}
