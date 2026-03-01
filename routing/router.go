package routing

import (
	"fmt"
	"time"

	"gofr.dev/pkg/gofr"

	"examples/llm-gateway/models"
)

// ChatProvider is the interface that routing uses for non-streaming completions.
// Satisfies provider.Provider without importing the provider package.
type ChatProvider interface {
	Name() string
	ChatCompletion(ctx *gofr.Context, req models.ChatCompletionRequest) (*models.ChatCompletionResponse, error)
}

// StreamChatProvider extends ChatProvider with streaming support.
// Satisfies provider.StreamingProvider without importing the provider package.
type StreamChatProvider interface {
	ChatProvider
	ChatCompletionStream(ctx *gofr.Context, req models.ChatCompletionRequest, onChunk func(models.StreamChunk)) error
}

// Router wraps retry + cooldown + strategy into a single call for chat completions.
// Handlers call the Router instead of calling providers directly.
type Router struct {
	RetryPolicy    *RetryPolicy
	Cooldown       *CooldownTracker
	Strategy       Strategy
	InFlight       *InFlightTracker
	Latency        *LatencyTracker
	Usage          *UsageTracker
}

// NewRouter creates a Router with the given components.
func NewRouter(retryPolicy *RetryPolicy, cooldown *CooldownTracker, strategy Strategy) *Router {
	return &Router{
		RetryPolicy: retryPolicy,
		Cooldown:    cooldown,
		Strategy:    strategy,
	}
}

// ChatCompletion executes a chat completion with retry and cooldown logic.
// Flow: check cooldown → execute with retries → record success/failure → track metrics.
func (r *Router) ChatCompletion(ctx *gofr.Context, p ChatProvider, modelName string, req models.ChatCompletionRequest) (*models.ChatCompletionResponse, error) {
	providerName := p.Name()

	// Check cooldown
	if !r.Cooldown.IsAvailable(providerName) {
		return nil, fmt.Errorf("provider %s is in cooldown", providerName)
	}

	req.Model = modelName

	// Track in-flight
	if r.InFlight != nil {
		r.InFlight.Increment(providerName)
		defer r.InFlight.Decrement(providerName)
	}

	start := time.Now()

	// Execute with retry policy
	resp, kind, err := r.RetryPolicy.Execute(ctx, func() (*models.ChatCompletionResponse, error) {
		return p.ChatCompletion(ctx, req)
	})
	if err != nil {
		r.Cooldown.RecordFailure(providerName)
		ctx.Logf("provider %s failed (kind=%s): %v", providerName, kind, err)

		return nil, err
	}

	r.Cooldown.RecordSuccess(providerName)

	// Track latency
	if r.Latency != nil {
		r.Latency.Record(providerName, time.Since(start))
	}

	// Track usage
	if r.Usage != nil && resp != nil {
		r.Usage.Record(providerName, resp.Usage.TotalTokens)
	}

	return resp, nil
}

// ChatCompletionStream executes a streaming chat completion with retry and cooldown logic.
func (r *Router) ChatCompletionStream(ctx *gofr.Context, sp StreamChatProvider, modelName string, req models.ChatCompletionRequest, onChunk func(models.StreamChunk)) error {
	providerName := sp.Name()

	// Check cooldown
	if !r.Cooldown.IsAvailable(providerName) {
		return fmt.Errorf("provider %s is in cooldown", providerName)
	}

	req.Model = modelName

	// Track in-flight
	if r.InFlight != nil {
		r.InFlight.Increment(providerName)
		defer r.InFlight.Decrement(providerName)
	}

	start := time.Now()

	// Execute with retry policy
	kind, err := r.RetryPolicy.ExecuteStream(ctx, func() error {
		return sp.ChatCompletionStream(ctx, req, onChunk)
	})
	if err != nil {
		r.Cooldown.RecordFailure(providerName)
		ctx.Logf("provider %s stream failed (kind=%s): %v", providerName, kind, err)

		return err
	}

	r.Cooldown.RecordSuccess(providerName)

	// Track latency
	if r.Latency != nil {
		r.Latency.Record(providerName, time.Since(start))
	}

	return nil
}
