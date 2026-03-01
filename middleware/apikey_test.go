package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseAPIKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]bool
	}{
		{
			name:     "empty string",
			input:    "",
			expected: map[string]bool{},
		},
		{
			name:     "single key",
			input:    "sk-key-1",
			expected: map[string]bool{"sk-key-1": true},
		},
		{
			name:     "multiple keys",
			input:    "sk-key-1,sk-key-2,sk-key-3",
			expected: map[string]bool{"sk-key-1": true, "sk-key-2": true, "sk-key-3": true},
		},
		{
			name:     "trimming spaces",
			input:    " sk-key-1 , sk-key-2 , sk-key-3 ",
			expected: map[string]bool{"sk-key-1": true, "sk-key-2": true, "sk-key-3": true},
		},
		{
			name:     "trailing comma produces no empty key",
			input:    "sk-key-1,",
			expected: map[string]bool{"sk-key-1": true},
		},
		{
			name:     "only commas and spaces",
			input:    " , , ",
			expected: map[string]bool{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ParseAPIKeys(tc.input)

			if len(result) != len(tc.expected) {
				t.Fatalf("expected %d keys, got %d", len(tc.expected), len(result))
			}

			for k, v := range tc.expected {
				if result[k] != v {
					t.Errorf("expected key %q = %v, got %v", k, v, result[k])
				}
			}
		})
	}
}

func TestParseKeyConfigs(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedKeys   []string
		expectedChains map[string][]string
	}{
		{
			name:           "empty string",
			input:          "",
			expectedKeys:   nil,
			expectedChains: map[string][]string{},
		},
		{
			name:         "single config",
			input:        "sk-key-1:openai,anthropic",
			expectedKeys: []string{"sk-key-1"},
			expectedChains: map[string][]string{
				"sk-key-1": {"openai", "anthropic"},
			},
		},
		{
			name:         "multiple configs",
			input:        "sk-key-1:openai,anthropic;sk-key-2:ollama",
			expectedKeys: []string{"sk-key-1", "sk-key-2"},
			expectedChains: map[string][]string{
				"sk-key-1": {"openai", "anthropic"},
				"sk-key-2": {"ollama"},
			},
		},
		{
			name:         "spaces are trimmed",
			input:        " sk-key-1 : openai , anthropic ; sk-key-2 : ollama ",
			expectedKeys: []string{"sk-key-1", "sk-key-2"},
			expectedChains: map[string][]string{
				"sk-key-1": {"openai", "anthropic"},
				"sk-key-2": {"ollama"},
			},
		},
		{
			name:           "entry without colon is skipped",
			input:          "sk-key-1-no-colon;sk-key-2:openai",
			expectedKeys:   []string{"sk-key-2"},
			expectedChains: map[string][]string{"sk-key-2": {"openai"}},
		},
		{
			name:           "trailing semicolon",
			input:          "sk-key-1:openai;",
			expectedKeys:   []string{"sk-key-1"},
			expectedChains: map[string][]string{"sk-key-1": {"openai"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ParseKeyConfigs(tc.input)

			// Check expected keys are present
			for _, k := range tc.expectedKeys {
				if _, ok := result[k]; !ok {
					t.Fatalf("expected key %q not found in result", k)
				}
			}

			// Check total count matches
			expectedCount := len(tc.expectedChains)
			if len(result) != expectedCount {
				t.Fatalf("expected %d configs, got %d", expectedCount, len(result))
			}

			// Check fallback chains
			for key, expectedChain := range tc.expectedChains {
				kc, ok := result[key]
				if !ok {
					t.Fatalf("missing config for key %q", key)
				}

				if kc.KeyID != key {
					t.Errorf("expected KeyID %q, got %q", key, kc.KeyID)
				}

				if len(kc.FallbackChain) != len(expectedChain) {
					t.Fatalf("key %q: expected chain length %d, got %d", key, len(expectedChain), len(kc.FallbackChain))
				}

				for i, p := range expectedChain {
					if kc.FallbackChain[i] != p {
						t.Errorf("key %q chain[%d]: expected %q, got %q", key, i, p, kc.FallbackChain[i])
					}
				}
			}
		})
	}
}

func TestAPIKeyAuth_MissingAuthHeader(t *testing.T) {
	validKeys := map[string]bool{"sk-test": true}
	handler := APIKeyAuth(validKeys, nil, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}

	body := rr.Body.String()
	if !searchString(body, "missing Authorization header") {
		t.Errorf("expected missing auth error message, got %q", body)
	}
}

func TestAPIKeyAuth_InvalidAuthHeader_NoBearerPrefix(t *testing.T) {
	validKeys := map[string]bool{"sk-test": true}
	handler := APIKeyAuth(validKeys, nil, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Basic sk-test")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}

	body := rr.Body.String()
	if !searchString(body, "invalid Authorization header") {
		t.Errorf("expected invalid auth header error, got %q", body)
	}
}

func TestAPIKeyAuth_InvalidKey(t *testing.T) {
	validKeys := map[string]bool{"sk-valid": true}
	handler := APIKeyAuth(validKeys, nil, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-wrong-key")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}

	body := rr.Body.String()
	if !searchString(body, "invalid API key") {
		t.Errorf("expected invalid API key error, got %q", body)
	}
}

func TestAPIKeyAuth_ValidStaticKey(t *testing.T) {
	validKeys := map[string]bool{"sk-valid": true}
	called := false

	handler := APIKeyAuth(validKeys, nil, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-valid")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	if !called {
		t.Error("expected inner handler to be called")
	}
}

func TestAPIKeyAuth_ValidVirtualKey(t *testing.T) {
	validKeys := map[string]bool{} // no static keys
	ks := NewKeyStore()
	virtualKey := "vk-my-virtual-key"
	ks.Add(HashKey(virtualKey))

	called := false

	handler := APIKeyAuth(validKeys, nil, ks)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer "+virtualKey)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	if !called {
		t.Error("expected inner handler to be called for virtual key")
	}
}

func TestAPIKeyAuth_InvalidKeyWithKeyStore(t *testing.T) {
	validKeys := map[string]bool{}
	ks := NewKeyStore()
	ks.Add(HashKey("vk-registered"))

	handler := APIKeyAuth(validKeys, nil, ks)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer vk-not-registered")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestAPIKeyAuth_HealthEndpointSkipsAuth(t *testing.T) {
	healthPaths := []string{"/health", "/.well-known/alive"}

	for _, path := range healthPaths {
		t.Run(path, func(t *testing.T) {
			validKeys := map[string]bool{"sk-valid": true}
			called := false

			handler := APIKeyAuth(validKeys, nil, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			}))

			// No Authorization header set
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("expected status %d for %s, got %d", http.StatusOK, path, rr.Code)
			}

			if !called {
				t.Errorf("expected inner handler to be called for %s", path)
			}
		})
	}
}

func TestAPIKeyAuth_GetAuthKeyFromContext(t *testing.T) {
	validKeys := map[string]bool{"sk-ctx-test": true}
	var capturedKey string

	handler := APIKeyAuth(validKeys, nil, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedKey = GetAuthKey(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-ctx-test")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if capturedKey != "sk-ctx-test" {
		t.Errorf("expected auth key %q, got %q", "sk-ctx-test", capturedKey)
	}
}

func TestAPIKeyAuth_GetKeyConfigFromContext(t *testing.T) {
	validKeys := map[string]bool{"sk-cfg-test": true}
	keyConfigs := map[string]*KeyConfig{
		"sk-cfg-test": {
			KeyID:         "sk-cfg-test",
			FallbackChain: []string{"openai", "anthropic"},
			Tier:          "premium",
		},
	}

	var capturedConfig *KeyConfig

	handler := APIKeyAuth(validKeys, keyConfigs, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedConfig = GetKeyConfig(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-cfg-test")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if capturedConfig == nil {
		t.Fatal("expected key config to be set in context")
	}

	if capturedConfig.KeyID != "sk-cfg-test" {
		t.Errorf("expected KeyID %q, got %q", "sk-cfg-test", capturedConfig.KeyID)
	}

	if capturedConfig.Tier != "premium" {
		t.Errorf("expected Tier %q, got %q", "premium", capturedConfig.Tier)
	}

	if len(capturedConfig.FallbackChain) != 2 {
		t.Fatalf("expected 2 providers in chain, got %d", len(capturedConfig.FallbackChain))
	}

	if capturedConfig.FallbackChain[0] != "openai" || capturedConfig.FallbackChain[1] != "anthropic" {
		t.Errorf("unexpected fallback chain: %v", capturedConfig.FallbackChain)
	}
}

func TestAPIKeyAuth_NoKeyConfigInContext(t *testing.T) {
	validKeys := map[string]bool{"sk-no-cfg": true}
	keyConfigs := map[string]*KeyConfig{} // no config for this key

	var capturedConfig *KeyConfig

	handler := APIKeyAuth(validKeys, keyConfigs, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedConfig = GetKeyConfig(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-no-cfg")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if capturedConfig != nil {
		t.Errorf("expected nil key config, got %+v", capturedConfig)
	}
}

func TestGetAuthKey_EmptyContext(t *testing.T) {
	key := GetAuthKey(context.Background())
	if key != "" {
		t.Errorf("expected empty string for missing auth key, got %q", key)
	}
}

func TestGetKeyConfig_EmptyContext(t *testing.T) {
	kc := GetKeyConfig(context.Background())
	if kc != nil {
		t.Errorf("expected nil for missing key config, got %+v", kc)
	}
}
