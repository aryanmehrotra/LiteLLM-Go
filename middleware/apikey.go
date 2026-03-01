package middleware

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const keyConfigKey contextKey = "keyConfig"
const authKeyKey contextKey = "authKey"

// KeyConfig holds per-key configuration resolved from the gateway config.
type KeyConfig struct {
	KeyID         string
	FallbackChain []string
	Tier          string
}

// GetAuthKey returns the authenticated API key from the request context.
func GetAuthKey(ctx context.Context) string {
	key, _ := ctx.Value(authKeyKey).(string)
	return key
}

// GetKeyConfig returns the KeyConfig from the request context, if any.
func GetKeyConfig(ctx context.Context) *KeyConfig {
	kc, _ := ctx.Value(keyConfigKey).(*KeyConfig)
	return kc
}

// APIKeyAuth returns a GoFr-compatible middleware that validates Bearer tokens
// against static keys and the in-memory KeyStore for virtual keys.
// keyStore may be nil if virtual keys are not configured.
func APIKeyAuth(validKeys map[string]bool, keyConfigs map[string]*KeyConfig, keyStore *KeyStore) func(inner http.Handler) http.Handler {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth for health checks, metrics, and admin static assets
			if strings.HasPrefix(r.URL.Path, "/health") || r.URL.Path == "/.well-known/alive" || strings.HasPrefix(r.URL.Path, "/admin") {
				inner.ServeHTTP(w, r)
				return
			}

			auth := r.Header.Get("Authorization")
			if auth == "" {
				http.Error(w, `{"error":"missing Authorization header"}`, http.StatusUnauthorized)
				return
			}

			key := strings.TrimPrefix(auth, "Bearer ")
			if key == auth {
				http.Error(w, `{"error":"invalid Authorization header"}`, http.StatusUnauthorized)
				return
			}

			// Check static keys first, then virtual keystore
			if !validKeys[key] && (keyStore == nil || !keyStore.IsValid(key)) {
				http.Error(w, `{"error":"invalid API key"}`, http.StatusUnauthorized)
				return
			}

			// Store the authenticated key in context
			reqCtx := context.WithValue(r.Context(), authKeyKey, key)

			// Attach per-key config if available
			if kc, ok := keyConfigs[key]; ok {
				reqCtx = context.WithValue(reqCtx, keyConfigKey, kc)
			}

			inner.ServeHTTP(w, r.WithContext(reqCtx))
		})
	}
}

// ParseAPIKeys splits a comma-separated key string into a lookup map.
func ParseAPIKeys(keys string) map[string]bool {
	m := make(map[string]bool)

	for _, k := range strings.Split(keys, ",") {
		k = strings.TrimSpace(k)
		if k != "" {
			m[k] = true
		}
	}

	return m
}

// ParseKeyConfigs parses key config string format:
// "sk-key-1:openai,anthropic;sk-key-2:anthropic,ollama"
func ParseKeyConfigs(config string) map[string]*KeyConfig {
	m := make(map[string]*KeyConfig)

	if config == "" {
		return m
	}

	entries := strings.Split(config, ";")
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		parts := strings.SplitN(entry, ":", 2)
		if len(parts) != 2 {
			continue
		}

		keyID := strings.TrimSpace(parts[0])
		providers := strings.Split(parts[1], ",")

		var chain []string
		for _, p := range providers {
			p = strings.TrimSpace(p)
			if p != "" {
				chain = append(chain, p)
			}
		}

		m[keyID] = &KeyConfig{
			KeyID:         keyID,
			FallbackChain: chain,
		}
	}

	return m
}
