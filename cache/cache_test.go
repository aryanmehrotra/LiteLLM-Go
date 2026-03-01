package cache

import (
	"testing"

	"aryanmehrotra/llm-gateway/models"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name       string
		ttlSeconds int
		wantTTL    int // expected TTL in seconds
	}{
		{"positive TTL", 600, 600},
		{"zero defaults to 300", 0, 300},
		{"negative defaults to 300", -1, 300},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New(tt.ttlSeconds)
			if c == nil {
				t.Fatal("expected non-nil cache")
			}

			// TTL is stored as time.Duration in seconds
			gotSeconds := int(c.ttl.Seconds())
			if gotSeconds != tt.wantTTL {
				t.Errorf("expected TTL %d seconds, got %d", tt.wantTTL, gotSeconds)
			}
		})
	}
}

func TestCacheKey_Deterministic(t *testing.T) {
	req := &models.ChatCompletionRequest{
		Model: "openai/gpt-4o",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	key1 := cacheKey(req)
	key2 := cacheKey(req)

	if key1 != key2 {
		t.Error("expected deterministic cache key")
	}

	if key1 == "" {
		t.Error("expected non-empty cache key")
	}

	// Key should have the prefix
	if len(key1) < 6 || key1[:6] != "llmgw:" {
		t.Errorf("expected 'llmgw:' prefix, got %q", key1[:6])
	}
}

func TestCacheKey_DifferentModels(t *testing.T) {
	req1 := &models.ChatCompletionRequest{
		Model: "openai/gpt-4o",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	req2 := &models.ChatCompletionRequest{
		Model: "anthropic/claude",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	if cacheKey(req1) == cacheKey(req2) {
		t.Error("different models should produce different keys")
	}
}

func TestCacheKey_DifferentMessages(t *testing.T) {
	req1 := &models.ChatCompletionRequest{
		Model: "openai/gpt-4o",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	req2 := &models.ChatCompletionRequest{
		Model: "openai/gpt-4o",
		Messages: []models.Message{
			{Role: "user", Content: "Goodbye"},
		},
	}

	if cacheKey(req1) == cacheKey(req2) {
		t.Error("different messages should produce different keys")
	}
}

func TestCacheKey_WithTemperature(t *testing.T) {
	temp1 := float64(0.5)
	temp2 := float64(0.9)

	req1 := &models.ChatCompletionRequest{
		Model:       "openai/gpt-4o",
		Messages:    []models.Message{{Role: "user", Content: "test"}},
		Temperature: &temp1,
	}

	req2 := &models.ChatCompletionRequest{
		Model:       "openai/gpt-4o",
		Messages:    []models.Message{{Role: "user", Content: "test"}},
		Temperature: &temp2,
	}

	if cacheKey(req1) == cacheKey(req2) {
		t.Error("different temperatures should produce different keys")
	}
}

func TestCacheKey_WithMaxTokens(t *testing.T) {
	max1 := 100
	max2 := 200

	req1 := &models.ChatCompletionRequest{
		Model:     "openai/gpt-4o",
		Messages:  []models.Message{{Role: "user", Content: "test"}},
		MaxTokens: &max1,
	}

	req2 := &models.ChatCompletionRequest{
		Model:     "openai/gpt-4o",
		Messages:  []models.Message{{Role: "user", Content: "test"}},
		MaxTokens: &max2,
	}

	if cacheKey(req1) == cacheKey(req2) {
		t.Error("different max_tokens should produce different keys")
	}
}

func TestCacheKey_NilOptionalFields(t *testing.T) {
	req := &models.ChatCompletionRequest{
		Model:    "openai/gpt-4o",
		Messages: []models.Message{{Role: "user", Content: "test"}},
	}

	// Should not panic with nil Temperature and MaxTokens
	key := cacheKey(req)
	if key == "" {
		t.Error("expected non-empty key even with nil optional fields")
	}
}

func TestCacheKey_MultipleMessages(t *testing.T) {
	req := &models.ChatCompletionRequest{
		Model: "openai/gpt-4o",
		Messages: []models.Message{
			{Role: "system", Content: "You are helpful"},
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there"},
			{Role: "user", Content: "How are you?"},
		},
	}

	key := cacheKey(req)
	if key == "" {
		t.Error("expected non-empty key for multi-message request")
	}
}

func TestCacheKey_MessageOrder(t *testing.T) {
	req1 := &models.ChatCompletionRequest{
		Model: "openai/gpt-4o",
		Messages: []models.Message{
			{Role: "user", Content: "first"},
			{Role: "user", Content: "second"},
		},
	}

	req2 := &models.ChatCompletionRequest{
		Model: "openai/gpt-4o",
		Messages: []models.Message{
			{Role: "user", Content: "second"},
			{Role: "user", Content: "first"},
		},
	}

	if cacheKey(req1) == cacheKey(req2) {
		t.Error("different message orders should produce different keys")
	}
}
