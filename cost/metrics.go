package cost

import (
	"gofr.dev/pkg/gofr"
)

const (
	metricCostUSD  = "llm_gateway_cost_usd"
	metricTokens   = "llm_gateway_tokens_total"
	metricRequests = "llm_gateway_requests_total"
)

// RegisterMetrics registers cost and token metrics with GoFr's metrics manager.
// Call this at startup with the GoFr app instance.
func RegisterMetrics(app *gofr.App) {
	m := app.Metrics()

	m.NewHistogram(metricCostUSD, "Cost of LLM requests in USD",
		0.0001, 0.001, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0)
	m.NewUpDownCounter(metricTokens, "Total tokens consumed")
	m.NewCounter(metricRequests, "Total LLM requests")
}

// RecordCost records a request's cost and token usage via GoFr context metrics.
func RecordCost(ctx *gofr.Context, providerName, model string, costUSD float64, totalTokens int) {
	ctx.Metrics().RecordHistogram(ctx, metricCostUSD, costUSD, "provider", providerName, "model", model)
	ctx.Metrics().IncrementCounter(ctx, metricRequests, "provider", providerName, "model", model)
	ctx.Metrics().DeltaUpDownCounter(ctx, metricTokens, float64(totalTokens), "provider", providerName, "model", model)
}
