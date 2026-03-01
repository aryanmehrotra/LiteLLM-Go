package cache

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"gofr.dev/pkg/gofr"

	"aryanmehrotra/litellm-go/models"
)

// Cache wraps Redis to cache LLM responses keyed by request content.
type Cache struct {
	ttl time.Duration
}

// New creates a Cache with the given TTL.
func New(ttlSeconds int) *Cache {
	if ttlSeconds <= 0 {
		ttlSeconds = 300
	}

	return &Cache{ttl: time.Duration(ttlSeconds) * time.Second}
}

// Get returns a cached response for the request, if one exists.
func (c *Cache) Get(ctx *gofr.Context, req *models.ChatCompletionRequest) (*models.ChatCompletionResponse, bool) {
	key := cacheKey(req)

	val, err := ctx.Redis.Get(ctx, key).Result()
	if err == redis.Nil || err != nil {
		return nil, false
	}

	var resp models.ChatCompletionResponse
	if err := json.Unmarshal([]byte(val), &resp); err != nil {
		ctx.Errorf("cache unmarshal error: %v", err)
		return nil, false
	}

	ctx.Infof("cache HIT for key %s", key[:16])

	return &resp, true
}

// Set stores a response in the cache.
func (c *Cache) Set(ctx *gofr.Context, req *models.ChatCompletionRequest, resp *models.ChatCompletionResponse) {
	key := cacheKey(req)

	data, err := json.Marshal(resp)
	if err != nil {
		ctx.Errorf("cache marshal error: %v", err)
		return
	}

	if err := ctx.Redis.Set(ctx, key, string(data), c.ttl).Err(); err != nil {
		ctx.Errorf("cache set error: %v", err)
		return
	}

	ctx.Debugf("cache SET for key %s (ttl=%s)", key[:16], c.ttl)
}

// cacheKey produces a deterministic SHA-256 key from request fields that affect the response.
func cacheKey(req *models.ChatCompletionRequest) string {
	h := sha256.New()

	fmt.Fprintf(h, "model:%s|", req.Model)

	for _, m := range req.Messages {
		fmt.Fprintf(h, "%s:%s|", m.Role, m.Content)
	}

	if req.Temperature != nil {
		fmt.Fprintf(h, "temp:%.2f|", *req.Temperature)
	}

	if req.MaxTokens != nil {
		fmt.Fprintf(h, "max:%d|", *req.MaxTokens)
	}

	return fmt.Sprintf("llmgw:%x", h.Sum(nil))
}
