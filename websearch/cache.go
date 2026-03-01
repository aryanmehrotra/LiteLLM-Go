package websearch

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gofr.dev/pkg/gofr"
)

// SearchCache caches web search results in Redis.
type SearchCache struct {
	ttl time.Duration
}

// NewSearchCache creates a SearchCache with the given TTL in seconds.
func NewSearchCache(ttlSeconds int) *SearchCache {
	if ttlSeconds <= 0 {
		ttlSeconds = 300
	}

	return &SearchCache{
		ttl: time.Duration(ttlSeconds) * time.Second,
	}
}

// Get retrieves cached search results for a query.
func (sc *SearchCache) Get(ctx *gofr.Context, query string) ([]SearchResult, bool) {
	key := searchCacheKey(query)

	val, err := ctx.Redis.Get(ctx, key).Result()
	if err != nil {
		return nil, false
	}

	var results []SearchResult
	if err := json.Unmarshal([]byte(val), &results); err != nil {
		return nil, false
	}

	return results, true
}

// Set stores search results in the cache.
func (sc *SearchCache) Set(ctx *gofr.Context, query string, results []SearchResult) {
	key := searchCacheKey(query)

	data, err := json.Marshal(results)
	if err != nil {
		return
	}

	if err := ctx.Redis.Set(ctx, key, string(data), sc.ttl).Err(); err != nil {
		ctx.Errorf("search cache set error: %v", err)
	}
}

func searchCacheKey(query string) string {
	normalized := strings.ToLower(strings.TrimSpace(query))
	h := sha256.Sum256([]byte(normalized))

	return fmt.Sprintf("llmgw:search:%x", h)
}
