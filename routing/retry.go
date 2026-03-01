package routing

import (
	"context"
	"math"
	"math/rand/v2"
	"time"

	"aryanmehrotra/litellm-go/models"
)

// RetryPolicy controls application-level retries with exponential backoff and jitter.
// This complements GoFr's transport-level RetryConfig — GoFr retries any 5xx at the HTTP
// layer (low count, no backoff), while this retries with smart policies based on error kind.
type RetryPolicy struct {
	MaxRetries     int
	BaseDelay      time.Duration
	MaxDelay       time.Duration
	Jitter         bool
	RetryableKinds map[ErrorKind]bool
}

// DefaultRetryPolicy returns a sensible default: retry rate limits and server errors.
func DefaultRetryPolicy(maxRetries int, baseDelay time.Duration) *RetryPolicy {
	return &RetryPolicy{
		MaxRetries: maxRetries,
		BaseDelay:  baseDelay,
		MaxDelay:   30 * time.Second,
		Jitter:     true,
		RetryableKinds: map[ErrorKind]bool{
			ErrRateLimit:   true,
			ErrServerError: true,
			ErrTimeout:     true,
		},
	}
}

// IsRetryable returns whether an error kind should be retried.
func (p *RetryPolicy) IsRetryable(kind ErrorKind) bool {
	return p.RetryableKinds[kind]
}

// Execute runs fn with retry logic. On retryable failures it backs off exponentially.
// Errors are classified using ClassifyFromError which handles both structured
// ProviderError and plain error strings from existing providers.
func (p *RetryPolicy) Execute(ctx context.Context, fn func() (*models.ChatCompletionResponse, error)) (*models.ChatCompletionResponse, ErrorKind, error) {
	var (
		lastKind ErrorKind
		lastErr  error
	)

	for attempt := range p.MaxRetries + 1 {
		resp, err := fn()
		if err == nil {
			return resp, ErrUnknown, nil
		}

		kind := ClassifyFromError(err)
		lastKind = kind
		lastErr = err

		if !p.IsRetryable(kind) {
			return nil, kind, err
		}

		// Don't sleep after the last attempt
		if attempt < p.MaxRetries {
			delay := p.backoff(attempt)

			select {
			case <-ctx.Done():
				return nil, ErrTimeout, ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	return nil, lastKind, lastErr
}

// ExecuteStream runs a streaming fn with retry logic.
func (p *RetryPolicy) ExecuteStream(ctx context.Context, fn func() error) (ErrorKind, error) {
	var (
		lastKind ErrorKind
		lastErr  error
	)

	for attempt := range p.MaxRetries + 1 {
		err := fn()
		if err == nil {
			return ErrUnknown, nil
		}

		kind := ClassifyFromError(err)
		lastKind = kind
		lastErr = err

		if !p.IsRetryable(kind) {
			return kind, err
		}

		if attempt < p.MaxRetries {
			delay := p.backoff(attempt)

			select {
			case <-ctx.Done():
				return ErrTimeout, ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	return lastKind, lastErr
}

// backoff calculates delay: min(baseDelay * 2^attempt + jitter, maxDelay).
func (p *RetryPolicy) backoff(attempt int) time.Duration {
	delay := float64(p.BaseDelay) * math.Pow(2, float64(attempt))

	if p.Jitter {
		jitter := rand.Float64() * float64(p.BaseDelay)
		delay += jitter
	}

	if delay > float64(p.MaxDelay) {
		delay = float64(p.MaxDelay)
	}

	return time.Duration(delay)
}
