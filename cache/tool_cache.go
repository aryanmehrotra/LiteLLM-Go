package cache

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"gofr.dev/pkg/gofr"
)

// ToolCache caches tool call results keyed by function name + arguments.
type ToolCache struct {
	ttl              time.Duration
	deterministicSet map[string]bool
}

// NewToolCache creates a ToolCache with the given TTL and set of deterministic tool names.
func NewToolCache(ttlSeconds int, deterministicTools string) *ToolCache {
	if ttlSeconds <= 0 {
		ttlSeconds = 3600
	}

	set := make(map[string]bool)

	for _, name := range strings.Split(deterministicTools, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			set[name] = true
		}
	}

	return &ToolCache{
		ttl:              time.Duration(ttlSeconds) * time.Second,
		deterministicSet: set,
	}
}

// IsDeterministic checks if a function name is in the deterministic set.
func (tc *ToolCache) IsDeterministic(functionName string) bool {
	if len(tc.deterministicSet) == 0 {
		return false
	}

	return tc.deterministicSet[functionName]
}

// Get retrieves a cached tool result.
func (tc *ToolCache) Get(ctx *gofr.Context, functionName, arguments string) (string, bool) {
	if !tc.IsDeterministic(functionName) {
		return "", false
	}

	key := toolCacheKey(functionName, arguments)

	val, err := ctx.Redis.Get(ctx, key).Result()
	if err == redis.Nil || err != nil {
		return "", false
	}

	return val, true
}

// Set stores a tool result in the cache.
func (tc *ToolCache) Set(ctx *gofr.Context, functionName, arguments, result string) {
	if !tc.IsDeterministic(functionName) {
		return
	}

	key := toolCacheKey(functionName, arguments)

	if err := ctx.Redis.Set(ctx, key, result, tc.ttl).Err(); err != nil {
		ctx.Errorf("tool cache set error: %v", err)
	}
}

func toolCacheKey(functionName, arguments string) string {
	h := sha256.New()

	fmt.Fprintf(h, "fn:%s|", functionName)

	// Normalize arguments JSON for consistent hashing
	var normalized any
	if err := json.Unmarshal([]byte(arguments), &normalized); err == nil {
		sortedArgs, _ := json.Marshal(normalized)
		h.Write(sortedArgs)
	} else {
		fmt.Fprint(h, arguments)
	}

	return fmt.Sprintf("llmgw:tool:%x", h.Sum(nil))
}
