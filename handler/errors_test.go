package handler

import (
	"testing"
)

func TestAPIError_Error(t *testing.T) {
	err := &APIError{HTTPCode: 400, ErrType: "invalid_request_error", Msg: "test message"}
	if err.Error() != "test message" {
		t.Errorf("expected 'test message', got %q", err.Error())
	}
}

func TestAPIError_StatusCode(t *testing.T) {
	tests := []struct {
		name     string
		err      *APIError
		wantCode int
	}{
		{"bad request", ErrBadRequest("bad"), 400},
		{"unauthorized", ErrUnauthorized("unauth"), 401},
		{"forbidden", ErrForbidden("forbidden"), 403},
		{"not found", ErrNotFound("thing"), 404},
		{"rate limit", ErrRateLimit("slow down"), 429},
		{"budget exceeded", ErrBudgetExceeded(), 429},
		{"provider failure", ErrProviderFailure("upstream"), 502},
		{"internal", ErrInternal("oops"), 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.StatusCode() != tt.wantCode {
				t.Errorf("expected status %d, got %d", tt.wantCode, tt.err.StatusCode())
			}
		})
	}
}

func TestAPIError_Response(t *testing.T) {
	err := &APIError{
		HTTPCode: 400,
		ErrType:  "invalid_request_error",
		ErrCode:  "invalid_api_key",
		Msg:      "test",
		Param:    "model",
	}

	resp := err.Response()

	if resp["type"] != "invalid_request_error" {
		t.Errorf("expected type 'invalid_request_error', got %v", resp["type"])
	}

	if resp["code"] != "invalid_api_key" {
		t.Errorf("expected code 'invalid_api_key', got %v", resp["code"])
	}

	if resp["param"] != "model" {
		t.Errorf("expected param 'model', got %v", resp["param"])
	}
}

func TestAPIError_Response_OmitsEmptyFields(t *testing.T) {
	err := &APIError{
		HTTPCode: 500,
		ErrType:  "server_error",
		Msg:      "test",
	}

	resp := err.Response()

	if _, ok := resp["code"]; ok {
		t.Error("expected 'code' to be omitted when empty")
	}

	if _, ok := resp["param"]; ok {
		t.Error("expected 'param' to be omitted when empty")
	}
}

func TestErrBadRequest(t *testing.T) {
	err := ErrBadRequest("bad input")
	if err.HTTPCode != 400 {
		t.Errorf("expected 400, got %d", err.HTTPCode)
	}

	if err.ErrType != "invalid_request_error" {
		t.Errorf("expected 'invalid_request_error', got %q", err.ErrType)
	}

	if err.Msg != "bad input" {
		t.Errorf("expected 'bad input', got %q", err.Msg)
	}
}

func TestErrMissingParam(t *testing.T) {
	err := ErrMissingParam("model")
	if err.HTTPCode != 400 {
		t.Errorf("expected 400, got %d", err.HTTPCode)
	}

	if err.Param != "model" {
		t.Errorf("expected param 'model', got %q", err.Param)
	}

	if err.Msg != "missing required parameter: model" {
		t.Errorf("unexpected message: %q", err.Msg)
	}
}

func TestErrInvalidParam(t *testing.T) {
	err := ErrInvalidParam("model", "model 'foo' not found")
	if err.HTTPCode != 400 {
		t.Errorf("expected 400, got %d", err.HTTPCode)
	}

	if err.Param != "model" {
		t.Errorf("expected param 'model', got %q", err.Param)
	}
}

func TestErrUnauthorized(t *testing.T) {
	err := ErrUnauthorized("invalid key")
	if err.HTTPCode != 401 {
		t.Errorf("expected 401, got %d", err.HTTPCode)
	}

	if err.ErrCode != "invalid_api_key" {
		t.Errorf("expected code 'invalid_api_key', got %q", err.ErrCode)
	}
}

func TestErrNotFound(t *testing.T) {
	err := ErrNotFound("key")
	if err.HTTPCode != 404 {
		t.Errorf("expected 404, got %d", err.HTTPCode)
	}

	if err.ErrCode != "not_found" {
		t.Errorf("expected code 'not_found', got %q", err.ErrCode)
	}

	if err.Msg != "key not found" {
		t.Errorf("expected 'key not found', got %q", err.Msg)
	}
}

func TestErrGuardrail(t *testing.T) {
	err := ErrGuardrail("blocked keyword detected")
	if err.HTTPCode != 400 {
		t.Errorf("expected 400, got %d", err.HTTPCode)
	}

	if err.ErrCode != "content_filtered" {
		t.Errorf("expected code 'content_filtered', got %q", err.ErrCode)
	}

	if err.ErrType != "content_policy_error" {
		t.Errorf("expected type 'content_policy_error', got %q", err.ErrType)
	}
}

func TestErrModelNotAllowed(t *testing.T) {
	err := ErrModelNotAllowed("openai/gpt-4o")
	if err.HTTPCode != 403 {
		t.Errorf("expected 403, got %d", err.HTTPCode)
	}

	if err.ErrCode != "model_not_allowed" {
		t.Errorf("expected code 'model_not_allowed', got %q", err.ErrCode)
	}
}

func TestErrUserBlocked(t *testing.T) {
	err := ErrUserBlocked()
	if err.HTTPCode != 403 {
		t.Errorf("expected 403, got %d", err.HTTPCode)
	}

	if err.ErrCode != "user_blocked" {
		t.Errorf("expected code 'user_blocked', got %q", err.ErrCode)
	}
}

func TestErrProviderFailure(t *testing.T) {
	err := ErrProviderFailure("timeout")
	if err.HTTPCode != 502 {
		t.Errorf("expected 502, got %d", err.HTTPCode)
	}

	if err.ErrCode != "provider_error" {
		t.Errorf("expected code 'provider_error', got %q", err.ErrCode)
	}
}

func TestErrBudgetExceeded(t *testing.T) {
	err := ErrBudgetExceeded()
	if err.HTTPCode != 429 {
		t.Errorf("expected 429, got %d", err.HTTPCode)
	}

	if err.ErrCode != "budget_exceeded" {
		t.Errorf("expected code 'budget_exceeded', got %q", err.ErrCode)
	}
}

func TestErrRateLimit(t *testing.T) {
	err := ErrRateLimit("too many requests")
	if err.HTTPCode != 429 {
		t.Errorf("expected 429, got %d", err.HTTPCode)
	}

	if err.ErrCode != "rate_limit_exceeded" {
		t.Errorf("expected code 'rate_limit_exceeded', got %q", err.ErrCode)
	}
}

func TestHashKey(t *testing.T) {
	key := "sk-test-key-12345"
	h1 := hashKey(key)
	h2 := hashKey(key)

	if h1 != h2 {
		t.Error("expected deterministic hash")
	}

	if len(h1) != 64 { // SHA-256 hex is 64 chars
		t.Errorf("expected 64 char hex hash, got %d", len(h1))
	}
}

func TestHashKey_DifferentKeys(t *testing.T) {
	h1 := hashKey("sk-key-1")
	h2 := hashKey("sk-key-2")

	if h1 == h2 {
		t.Error("different keys should produce different hashes")
	}
}

func TestGenerateRandomKey(t *testing.T) {
	key1 := generateRandomKey()
	key2 := generateRandomKey()

	if key1 == key2 {
		t.Error("expected different random keys")
	}

	if len(key1) < 10 {
		t.Errorf("key too short: %q", key1)
	}

	if key1[:3] != "sk-" {
		t.Errorf("expected 'sk-' prefix, got %q", key1[:3])
	}
}

func TestSha256hex(t *testing.T) {
	result := sha256hex("test")
	if len(result) != 64 {
		t.Errorf("expected 64 char hex, got %d", len(result))
	}

	// Should be deterministic
	if sha256hex("test") != result {
		t.Error("expected deterministic output")
	}

	// Different inputs should produce different outputs
	if sha256hex("test1") == sha256hex("test2") {
		t.Error("different inputs should produce different hashes")
	}
}

func TestNullStr(t *testing.T) {
	tests := []struct {
		name string
		ns   interface{ Valid() bool }
		want string
	}{}
	_ = tests // placeholder - nullStr takes sql.NullString which we can test directly
}

func TestCountKeys(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"empty string", "", 0},
		{"single key", "sk-key1", 1},
		{"two keys", "sk-key1,sk-key2", 2},
		{"three keys", "sk-key1,sk-key2,sk-key3", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countKeys(tt.input)
			if got != tt.want {
				t.Errorf("countKeys(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsConfigured(t *testing.T) {
	tests := []struct {
		name   string
		envKey string
		dbKey  string
		want   bool
	}{
		{"both empty", "", "", false},
		{"env key set", "sk-real-key", "", true},
		{"db key set", "", "sk-db-key", true},
		{"both set", "sk-real-key", "sk-db-key", true},
		{"env placeholder", "your-openai-key", "", false},
		{"env placeholder uppercase", "Your-key", "", false},
		{"env placeholder with db key", "your-openai-key", "sk-db-key", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isConfigured(tt.envKey, tt.dbKey)
			if got != tt.want {
				t.Errorf("isConfigured(%q, %q) = %v, want %v", tt.envKey, tt.dbKey, got, tt.want)
			}
		})
	}
}
