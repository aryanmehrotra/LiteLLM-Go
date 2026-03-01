package handler

import "fmt"

// APIError returns structured error responses compatible with the OpenAI error format.
// It implements Go's error interface plus GoFr's StatusCodeResponder and ResponseMarshaller
// interfaces so the framework serializes it as:
//
//	{"error":{"message":"...","type":"...","code":"...","param":"..."}}
type APIError struct {
	HTTPCode int    `json:"-"`
	ErrType  string `json:"type"`
	ErrCode  string `json:"code,omitempty"`
	Msg      string `json:"message"`
	Param    string `json:"param,omitempty"`
}

// Error implements the error interface.
func (e *APIError) Error() string { return e.Msg }

// StatusCode implements GoFr's StatusCodeResponder so the framework returns the correct HTTP status.
func (e *APIError) StatusCode() int { return e.HTTPCode }

// Response implements GoFr's ResponseMarshaller so extra fields (type, code, param) are merged
// into the error response alongside the "message" field.
func (e *APIError) Response() map[string]any {
	m := map[string]any{
		"type": e.ErrType,
	}

	if e.ErrCode != "" {
		m["code"] = e.ErrCode
	}

	if e.Param != "" {
		m["param"] = e.Param
	}

	return m
}

// ---------------------------------------------------------------------------
// Convenience constructors
// ---------------------------------------------------------------------------

// ErrBadRequest returns a 400 invalid_request_error.
func ErrBadRequest(msg string) *APIError {
	return &APIError{HTTPCode: 400, ErrType: "invalid_request_error", Msg: msg}
}

// ErrMissingParam returns a 400 for a required parameter that was not provided.
func ErrMissingParam(param string) *APIError {
	return &APIError{
		HTTPCode: 400,
		ErrType:  "invalid_request_error",
		Msg:      fmt.Sprintf("missing required parameter: %s", param),
		Param:    param,
	}
}

// ErrInvalidParam returns a 400 for a parameter that has an invalid value.
func ErrInvalidParam(param, detail string) *APIError {
	return &APIError{HTTPCode: 400, ErrType: "invalid_request_error", Msg: detail, Param: param}
}

// ErrUnauthorized returns a 401 authentication_error.
func ErrUnauthorized(msg string) *APIError {
	return &APIError{HTTPCode: 401, ErrType: "authentication_error", Msg: msg, ErrCode: "invalid_api_key"}
}

// ErrForbidden returns a 403 permission_error.
func ErrForbidden(msg string) *APIError {
	return &APIError{HTTPCode: 403, ErrType: "permission_error", Msg: msg}
}

// ErrNotFound returns a 404 not_found_error for the given resource.
func ErrNotFound(resource string) *APIError {
	return &APIError{
		HTTPCode: 404,
		ErrType:  "not_found_error",
		Msg:      fmt.Sprintf("%s not found", resource),
		ErrCode:  "not_found",
	}
}

// ErrRateLimit returns a 429 rate_limit_error.
func ErrRateLimit(msg string) *APIError {
	return &APIError{HTTPCode: 429, ErrType: "rate_limit_error", Msg: msg, ErrCode: "rate_limit_exceeded"}
}

// ErrBudgetExceeded returns a 429 indicating the key's budget has been exhausted.
func ErrBudgetExceeded() *APIError {
	return &APIError{
		HTTPCode: 429,
		ErrType:  "rate_limit_error",
		Msg:      "budget exceeded for this key",
		ErrCode:  "budget_exceeded",
	}
}

// ErrUserBlocked returns a 403 indicating the user associated with the key is blocked.
func ErrUserBlocked() *APIError {
	return &APIError{HTTPCode: 403, ErrType: "permission_error", Msg: "user is blocked", ErrCode: "user_blocked"}
}

// ErrModelNotAllowed returns a 403 indicating the requested model is not permitted for this key.
func ErrModelNotAllowed(model string) *APIError {
	return &APIError{
		HTTPCode: 403,
		ErrType:  "permission_error",
		Msg:      fmt.Sprintf("model %q not allowed for this key", model),
		ErrCode:  "model_not_allowed",
	}
}

// ErrGuardrail returns a 400 content_policy_error for guardrail violations.
func ErrGuardrail(msg string) *APIError {
	return &APIError{HTTPCode: 400, ErrType: "content_policy_error", Msg: msg, ErrCode: "content_filtered"}
}

// ErrProviderFailure returns a 502 upstream_error when a provider call fails.
func ErrProviderFailure(msg string) *APIError {
	return &APIError{HTTPCode: 502, ErrType: "upstream_error", Msg: msg, ErrCode: "provider_error"}
}

// ErrInternal returns a 500 server_error. Use this instead of fmt.Errorf to avoid
// leaking internal details (database errors, stack traces, etc.) to API consumers.
func ErrInternal(msg string) *APIError {
	return &APIError{HTTPCode: 500, ErrType: "server_error", Msg: msg}
}
