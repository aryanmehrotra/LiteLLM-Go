// Package testutil provides shared test helpers for the LiteLLM-Go gateway.
// It includes a MockProvider (returns pre-set responses without HTTP),
// a MockLLMServer (real httptest.Server serving OpenAI-format responses),
// and NewGofrCtx (creates a minimal *gofr.Context for unit tests).
package testutil

import (
	"context"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/logging"
)

// NewGofrCtx creates a minimal *gofr.Context suitable for unit tests.
// It has logging but no database, Redis, or HTTP service connections.
// This is sufficient for testing logic that does not touch ctx.SQL or
// ctx.GetHTTPService (i.e. handler logic that goes through a MockProvider).
func NewGofrCtx() *gofr.Context {
	logger := logging.NewLogger(logging.INFO)
	cl := logging.NewContextLogger(context.Background(), logger)
	ctx := &gofr.Context{
		Context: context.Background(),
	}
	// ContextLogger is embedded by value; copy the dereferenced struct.
	// This is valid Go: unexported fields are preserved by value copy.
	ctx.ContextLogger = *cl

	return ctx
}
