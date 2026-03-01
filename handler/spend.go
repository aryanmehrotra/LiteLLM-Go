package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"gofr.dev/pkg/gofr"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/response"

	"examples/llm-gateway/middleware"
)

// SpendReportRow represents a single row in the spend report.
type SpendReportRow struct {
	GroupBy           string  `json:"group_by"`
	TotalCost         float64 `json:"total_cost"`
	TotalTokens       int     `json:"total_tokens"`
	PromptTokens      int     `json:"prompt_tokens"`
	CompletionTokens  int     `json:"completion_tokens"`
	RequestCount      int     `json:"request_count"`
}

var validGroupBy = map[string]string{
	"provider": "provider",
	"model":    "model",
	"key":      "key_id",
	"team":     "team_id",
	"user":     "user_id",
	"org":      "org_id",
}

// SpendReport handles GET /spend/report.
// Query params: start_date, end_date, group_by (provider/model/key/team/user/org).
func (h *AdminHandler) SpendReport() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		startDate := ctx.Param("start_date")
		endDate := ctx.Param("end_date")
		groupByParam := ctx.Param("group_by")

		if groupByParam == "" {
			groupByParam = "provider"
		}

		groupByCol, ok := validGroupBy[groupByParam]
		if !ok {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"group_by"}}
		}

		// Build dynamic query
		var conditions []string
		var args []any
		argIdx := 1

		if startDate != "" {
			conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argIdx))
			args = append(args, startDate)
			argIdx++
		}

		if endDate != "" {
			conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argIdx))
			args = append(args, endDate)
			argIdx++
		}

		where := ""
		if len(conditions) > 0 {
			where = "WHERE " + strings.Join(conditions, " AND ")
		}

		query := fmt.Sprintf(`
			SELECT %s, COALESCE(SUM(cost), 0), COALESCE(SUM(total_tokens), 0),
			       COALESCE(SUM(prompt_tokens), 0), COALESCE(SUM(completion_tokens), 0), COUNT(*)
			FROM spend_log %s
			GROUP BY %s
			ORDER BY SUM(cost) DESC
		`, groupByCol, where, groupByCol)

		rows, err := ctx.SQL.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("spend report query: %w", err)
		}
		defer rows.Close()

		results := []SpendReportRow{}

		for rows.Next() {
			var row SpendReportRow
			if err := rows.Scan(&row.GroupBy, &row.TotalCost, &row.TotalTokens,
				&row.PromptTokens, &row.CompletionTokens, &row.RequestCount); err != nil {
				return nil, fmt.Errorf("scan spend report row: %w", err)
			}

			results = append(results, row)
		}

		return response.Raw{Data: map[string]any{
			"group_by": groupByParam,
			"data":     results,
		}}, nil
	}
}

// SpendSelf handles GET /spend/self — returns spend data for the authenticated key only.
// No master key required; any virtual key holder can see their own spend.
func (h *AdminHandler) SpendSelf() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		authKey := middleware.GetAuthKey(ctx)
		if authKey == "" {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"authorization"}}
		}

		sum := sha256.Sum256([]byte(authKey))
		keyHash := hex.EncodeToString(sum[:])

		groupByParam := ctx.Param("group_by")
		if groupByParam == "" {
			groupByParam = "model"
		}

		groupByCol, ok := validGroupBy[groupByParam]
		if !ok {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"group_by"}}
		}

		query := fmt.Sprintf(`
			SELECT %s, COALESCE(SUM(cost), 0), COALESCE(SUM(total_tokens), 0),
			       COALESCE(SUM(prompt_tokens), 0), COALESCE(SUM(completion_tokens), 0), COUNT(*)
			FROM spend_log WHERE key_id = $1
			GROUP BY %s
			ORDER BY SUM(cost) DESC
		`, groupByCol, groupByCol)

		rows, err := ctx.SQL.QueryContext(ctx, query, keyHash)
		if err != nil {
			return nil, fmt.Errorf("spend self query: %w", err)
		}
		defer rows.Close()

		results := []SpendReportRow{}

		for rows.Next() {
			var row SpendReportRow
			if err := rows.Scan(&row.GroupBy, &row.TotalCost, &row.TotalTokens,
				&row.PromptTokens, &row.CompletionTokens, &row.RequestCount); err != nil {
				return nil, fmt.Errorf("scan spend self row: %w", err)
			}

			results = append(results, row)
		}

		return response.Raw{Data: map[string]any{
			"group_by": groupByParam,
			"data":     results,
		}}, nil
	}
}
