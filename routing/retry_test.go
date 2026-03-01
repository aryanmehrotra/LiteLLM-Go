package routing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"aryanmehrotra/llm-gateway/models"
)

func TestRetryPolicy_Execute_SucceedsOnFirstAttempt(t *testing.T) {
	policy := DefaultRetryPolicy(3, 10*time.Millisecond)

	callCount := 0
	resp, kind, err := policy.Execute(context.Background(), func() (*models.ChatCompletionResponse, error) {
		callCount++
		return &models.ChatCompletionResponse{ID: "test-1"}, nil
	})

	assert.NoError(t, err)
	assert.Equal(t, ErrUnknown, kind) // success returns ErrUnknown
	assert.Equal(t, "test-1", resp.ID)
	assert.Equal(t, 1, callCount)
}

func TestRetryPolicy_Execute_RetriesOnRetryableError(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:     3,
		BaseDelay:      1 * time.Millisecond,
		MaxDelay:       10 * time.Millisecond,
		Jitter:         false,
		RetryableKinds: map[ErrorKind]bool{ErrServerError: true},
	}

	callCount := 0
	_, kind, err := policy.Execute(context.Background(), func() (*models.ChatCompletionResponse, error) {
		callCount++
		return nil, &ProviderError{
			Provider:   "test",
			StatusCode: 500,
			Body:       "internal server error",
			Err:        errors.New("test returned status 500: internal server error"),
		}
	})

	assert.Error(t, err)
	assert.Equal(t, ErrServerError, kind)
	assert.Equal(t, 4, callCount) // 1 initial + 3 retries
}

func TestRetryPolicy_Execute_StopsOnNonRetryableError(t *testing.T) {
	policy := DefaultRetryPolicy(3, 10*time.Millisecond)

	callCount := 0
	_, kind, err := policy.Execute(context.Background(), func() (*models.ChatCompletionResponse, error) {
		callCount++
		return nil, &ProviderError{
			Provider:   "test",
			StatusCode: 401,
			Body:       "unauthorized",
			Err:        errors.New("test returned status 401: unauthorized"),
		}
	})

	assert.Error(t, err)
	assert.Equal(t, ErrAuth, kind)
	assert.Equal(t, 1, callCount) // no retries for auth errors
}

func TestRetryPolicy_Execute_RespectsMaxRetries(t *testing.T) {
	tests := []struct {
		name       string
		maxRetries int
		wantCalls  int
	}{
		{name: "zero retries", maxRetries: 0, wantCalls: 1},
		{name: "one retry", maxRetries: 1, wantCalls: 2},
		{name: "five retries", maxRetries: 5, wantCalls: 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &RetryPolicy{
				MaxRetries:     tt.maxRetries,
				BaseDelay:      1 * time.Millisecond,
				MaxDelay:       10 * time.Millisecond,
				Jitter:         false,
				RetryableKinds: map[ErrorKind]bool{ErrRateLimit: true},
			}

			callCount := 0
			_, _, err := policy.Execute(context.Background(), func() (*models.ChatCompletionResponse, error) {
				callCount++
				return nil, &ProviderError{
					Provider:   "test",
					StatusCode: 429,
					Body:       "rate limited",
					Err:        errors.New("test returned status 429: rate limited"),
				}
			})

			assert.Error(t, err)
			assert.Equal(t, tt.wantCalls, callCount)
		})
	}
}

func TestRetryPolicy_Execute_SucceedsAfterRetries(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:     3,
		BaseDelay:      1 * time.Millisecond,
		MaxDelay:       10 * time.Millisecond,
		Jitter:         false,
		RetryableKinds: map[ErrorKind]bool{ErrServerError: true},
	}

	callCount := 0
	resp, kind, err := policy.Execute(context.Background(), func() (*models.ChatCompletionResponse, error) {
		callCount++
		if callCount < 3 {
			return nil, &ProviderError{
				Provider:   "test",
				StatusCode: 500,
				Body:       "server error",
				Err:        errors.New("test returned status 500: server error"),
			}
		}

		return &models.ChatCompletionResponse{ID: "success"}, nil
	})

	assert.NoError(t, err)
	assert.Equal(t, ErrUnknown, kind)
	assert.Equal(t, "success", resp.ID)
	assert.Equal(t, 3, callCount)
}

func TestRetryPolicy_Execute_ContextCancellation(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:     5,
		BaseDelay:      100 * time.Millisecond,
		MaxDelay:       1 * time.Second,
		Jitter:         false,
		RetryableKinds: map[ErrorKind]bool{ErrServerError: true},
	}

	ctx, cancel := context.WithCancel(context.Background())

	callCount := 0
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, kind, err := policy.Execute(ctx, func() (*models.ChatCompletionResponse, error) {
		callCount++
		return nil, &ProviderError{
			Provider:   "test",
			StatusCode: 500,
			Body:       "error",
			Err:        errors.New("test returned status 500: error"),
		}
	})

	assert.Error(t, err)
	assert.Equal(t, ErrTimeout, kind)
	assert.True(t, callCount >= 1, "should have been called at least once")
}

func TestRetryPolicy_Backoff(t *testing.T) {
	t.Run("exponential without jitter", func(t *testing.T) {
		policy := &RetryPolicy{
			BaseDelay: 100 * time.Millisecond,
			MaxDelay:  10 * time.Second,
			Jitter:    false,
		}

		tests := []struct {
			attempt int
			wantMin time.Duration
			wantMax time.Duration
		}{
			{attempt: 0, wantMin: 100 * time.Millisecond, wantMax: 100 * time.Millisecond},
			{attempt: 1, wantMin: 200 * time.Millisecond, wantMax: 200 * time.Millisecond},
			{attempt: 2, wantMin: 400 * time.Millisecond, wantMax: 400 * time.Millisecond},
			{attempt: 3, wantMin: 800 * time.Millisecond, wantMax: 800 * time.Millisecond},
		}

		for _, tt := range tests {
			delay := policy.backoff(tt.attempt)
			assert.GreaterOrEqual(t, delay, tt.wantMin, "attempt %d", tt.attempt)
			assert.LessOrEqual(t, delay, tt.wantMax, "attempt %d", tt.attempt)
		}
	})

	t.Run("exponential with jitter", func(t *testing.T) {
		policy := &RetryPolicy{
			BaseDelay: 100 * time.Millisecond,
			MaxDelay:  10 * time.Second,
			Jitter:    true,
		}

		// With jitter, delay = baseDelay * 2^attempt + rand * baseDelay
		// For attempt 0: 100ms + [0, 100ms) => [100ms, 200ms)
		delay := policy.backoff(0)
		assert.GreaterOrEqual(t, delay, 100*time.Millisecond)
		assert.Less(t, delay, 200*time.Millisecond)

		// For attempt 1: 200ms + [0, 100ms) => [200ms, 300ms)
		delay = policy.backoff(1)
		assert.GreaterOrEqual(t, delay, 200*time.Millisecond)
		assert.Less(t, delay, 300*time.Millisecond)
	})

	t.Run("capped at max delay", func(t *testing.T) {
		policy := &RetryPolicy{
			BaseDelay: 1 * time.Second,
			MaxDelay:  5 * time.Second,
			Jitter:    false,
		}

		// Attempt 10: 1s * 2^10 = 1024s, should be capped at 5s
		delay := policy.backoff(10)
		assert.Equal(t, 5*time.Second, delay)
	})
}

func TestRetryPolicy_IsRetryable(t *testing.T) {
	policy := DefaultRetryPolicy(3, 10*time.Millisecond)

	tests := []struct {
		name string
		kind ErrorKind
		want bool
	}{
		{name: "rate limit is retryable", kind: ErrRateLimit, want: true},
		{name: "server error is retryable", kind: ErrServerError, want: true},
		{name: "timeout is retryable", kind: ErrTimeout, want: true},
		{name: "auth is not retryable", kind: ErrAuth, want: false},
		{name: "token limit is not retryable", kind: ErrTokenLimit, want: false},
		{name: "content policy is not retryable", kind: ErrContentPolicy, want: false},
		{name: "unknown is not retryable", kind: ErrUnknown, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, policy.IsRetryable(tt.kind))
		})
	}
}

func TestRetryPolicy_ExecuteStream(t *testing.T) {
	t.Run("succeeds on first attempt", func(t *testing.T) {
		policy := DefaultRetryPolicy(3, 1*time.Millisecond)

		callCount := 0
		kind, err := policy.ExecuteStream(context.Background(), func() error {
			callCount++
			return nil
		})

		assert.NoError(t, err)
		assert.Equal(t, ErrUnknown, kind)
		assert.Equal(t, 1, callCount)
	})

	t.Run("retries on retryable error then succeeds", func(t *testing.T) {
		policy := &RetryPolicy{
			MaxRetries:     3,
			BaseDelay:      1 * time.Millisecond,
			MaxDelay:       10 * time.Millisecond,
			Jitter:         false,
			RetryableKinds: map[ErrorKind]bool{ErrServerError: true},
		}

		callCount := 0
		kind, err := policy.ExecuteStream(context.Background(), func() error {
			callCount++
			if callCount < 2 {
				return &ProviderError{
					Provider:   "test",
					StatusCode: 500,
					Body:       "error",
					Err:        errors.New("test returned status 500: error"),
				}
			}

			return nil
		})

		assert.NoError(t, err)
		assert.Equal(t, ErrUnknown, kind)
		assert.Equal(t, 2, callCount)
	})

	t.Run("stops on non-retryable error", func(t *testing.T) {
		policy := DefaultRetryPolicy(3, 1*time.Millisecond)

		callCount := 0
		kind, err := policy.ExecuteStream(context.Background(), func() error {
			callCount++
			return &ProviderError{
				Provider:   "test",
				StatusCode: 403,
				Body:       "forbidden",
				Err:        errors.New("test returned status 403: forbidden"),
			}
		})

		assert.Error(t, err)
		assert.Equal(t, ErrAuth, kind)
		assert.Equal(t, 1, callCount)
	})
}

func TestDefaultRetryPolicy(t *testing.T) {
	policy := DefaultRetryPolicy(5, 500*time.Millisecond)

	assert.Equal(t, 5, policy.MaxRetries)
	assert.Equal(t, 500*time.Millisecond, policy.BaseDelay)
	assert.Equal(t, 30*time.Second, policy.MaxDelay)
	assert.True(t, policy.Jitter)
	assert.True(t, policy.RetryableKinds[ErrRateLimit])
	assert.True(t, policy.RetryableKinds[ErrServerError])
	assert.True(t, policy.RetryableKinds[ErrTimeout])
	assert.False(t, policy.RetryableKinds[ErrAuth])
}
