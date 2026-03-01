package budget

import (
	"database/sql"
	"fmt"

	"gofr.dev/pkg/gofr"
)

// CheckBudget checks if the entity is within budget.
// Returns true if the entity can proceed, false if budget exceeded.
func CheckBudget(ctx *gofr.Context, entityType, entityID string) (bool, error) {
	var maxBudget, currentSpend float64

	err := ctx.SQL.QueryRowContext(ctx,
		"SELECT max_budget, current_spend FROM budgets WHERE entity_type = $1 AND entity_id = $2",
		entityType, entityID,
	).Scan(&maxBudget, &currentSpend)
	if err == sql.ErrNoRows {
		return true, nil // no budget set = unlimited
	}

	if err != nil {
		return true, fmt.Errorf("check budget: %w", err)
	}

	if maxBudget <= 0 {
		return true, nil
	}

	return currentSpend < maxBudget, nil
}

// RecordSpend logs a spend event and updates the budget counter.
func RecordSpend(ctx *gofr.Context, keyID, userID, teamID, orgID, providerName, model string, promptTokens, completionTokens, totalTokens int, costUSD float64) error {
	// Insert spend log
	_, err := ctx.SQL.ExecContext(ctx,
		`INSERT INTO spend_log (key_id, user_id, team_id, org_id, provider, model, prompt_tokens, completion_tokens, total_tokens, cost)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		keyID, userID, teamID, orgID, providerName, model, promptTokens, completionTokens, totalTokens, costUSD,
	)
	if err != nil {
		return fmt.Errorf("insert spend log: %w", err)
	}

	// Update budget counters for each entity type
	entities := []struct{ typ, id string }{
		{"key", keyID},
		{"user", userID},
		{"team", teamID},
		{"org", orgID},
	}

	for _, e := range entities {
		if e.id == "" {
			continue
		}

		_, err := ctx.SQL.ExecContext(ctx,
			"UPDATE budgets SET current_spend = current_spend + $1, updated_at = CURRENT_TIMESTAMP WHERE entity_type = $2 AND entity_id = $3",
			costUSD, e.typ, e.id,
		)
		if err != nil {
			ctx.Errorf("update budget for %s/%s: %v", e.typ, e.id, err)
		}

		// Check and alert
		CheckAndAlert(ctx, e.typ, e.id)
	}

	return nil
}
