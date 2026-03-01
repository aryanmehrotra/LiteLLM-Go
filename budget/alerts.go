package budget

import (
	"database/sql"

	"gofr.dev/pkg/gofr"
)

// Budget threshold percentages that trigger alerts.
var alertThresholds = []float64{0.50, 0.80, 1.00}

// CheckAndAlert checks current spend against budget and logs alerts on threshold crossings.
func CheckAndAlert(ctx *gofr.Context, entityType, entityID string) {
	var maxBudget, currentSpend float64

	err := ctx.SQL.QueryRowContext(ctx,
		"SELECT max_budget, current_spend FROM budgets WHERE entity_type = $1 AND entity_id = $2",
		entityType, entityID,
	).Scan(&maxBudget, &currentSpend)
	if err == sql.ErrNoRows || err != nil {
		return
	}

	if maxBudget <= 0 {
		return
	}

	ratio := currentSpend / maxBudget

	for _, threshold := range alertThresholds {
		if ratio >= threshold {
			ctx.Logf("BUDGET ALERT: %s/%s at %.0f%% of budget ($%.4f / $%.4f)",
				entityType, entityID, threshold*100, currentSpend, maxBudget)
		}
	}
}
