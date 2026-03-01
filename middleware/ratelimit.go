package middleware

import (
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"gofr.dev/pkg/gofr"
)

// CheckRateLimit checks if the key is within its RPM rate limit using a Redis sliding window.
// Returns nil if within limits, error if exceeded.
func CheckRateLimit(ctx *gofr.Context, keyHash string, rpmLimit int) error {
	if rpmLimit <= 0 {
		return nil
	}

	key := fmt.Sprintf("llmgw:rpm:%s", keyHash)
	now := time.Now().Unix()
	windowStart := now - 60

	// Remove entries outside the window
	ctx.Redis.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart))

	// Count entries in current window
	count, err := ctx.Redis.ZCard(ctx, key).Result()
	if err != nil && err != redis.Nil {
		return nil // fail open on Redis errors
	}

	if count >= int64(rpmLimit) {
		return fmt.Errorf("rate limit exceeded: %d requests per minute", rpmLimit)
	}

	// Add current request
	ctx.Redis.ZAdd(ctx, key, redis.Z{Score: float64(now), Member: now})
	ctx.Redis.Expire(ctx, key, 2*time.Minute)

	return nil
}

// RateLimitInfo holds rate limit details for response headers.
type RateLimitInfo struct {
	Limit     int
	Remaining int
	ResetAt   int64 // Unix timestamp
}

// GetRateLimitInfo returns the current RPM usage for a key without incrementing.
func GetRateLimitInfo(ctx *gofr.Context, keyHash string, rpmLimit int) *RateLimitInfo {
	if rpmLimit <= 0 {
		return nil
	}

	key := fmt.Sprintf("llmgw:rpm:%s", keyHash)
	now := time.Now().Unix()
	windowStart := now - 60

	count, err := ctx.Redis.ZCount(ctx, key, fmt.Sprintf("%d", windowStart), "+inf").Result()
	if err != nil {
		return nil
	}

	remaining := rpmLimit - int(count)
	if remaining < 0 {
		remaining = 0
	}

	return &RateLimitInfo{
		Limit:     rpmLimit,
		Remaining: remaining,
		ResetAt:   now + 60,
	}
}

// RecordTokenUsage tracks TPM usage for a key using Redis.
func RecordTokenUsage(ctx *gofr.Context, keyHash string, tokens int, tpmLimit int) error {
	if tpmLimit <= 0 {
		return nil
	}

	key := fmt.Sprintf("llmgw:tpm:%s", keyHash)

	current, err := ctx.Redis.Get(ctx, key).Int64()
	if err != nil && err != redis.Nil {
		return nil // fail open
	}

	if current+int64(tokens) > int64(tpmLimit) {
		return fmt.Errorf("token rate limit exceeded: %d tokens per minute", tpmLimit)
	}

	pipe := ctx.Redis.TxPipeline()
	pipe.IncrBy(ctx, key, int64(tokens))
	pipe.Expire(ctx, key, time.Minute)
	_, _ = pipe.Exec(ctx)

	return nil
}
