package routing

import (
	"context"
	"errors"
	"strings"
)

// ErrorKind classifies upstream errors into actionable categories for routing decisions.
type ErrorKind int

const (
	ErrUnknown       ErrorKind = iota
	ErrRateLimit               // 429 from upstream
	ErrTokenLimit              // context length exceeded
	ErrContentPolicy           // content moderation rejection
	ErrAuth                    // 401/403
	ErrServerError             // 5xx
	ErrTimeout                 // deadline exceeded
)

func (k ErrorKind) String() string {
	switch k {
	case ErrRateLimit:
		return "rate_limit"
	case ErrTokenLimit:
		return "token_limit"
	case ErrContentPolicy:
		return "content_policy"
	case ErrAuth:
		return "auth"
	case ErrServerError:
		return "server_error"
	case ErrTimeout:
		return "timeout"
	default:
		return "unknown"
	}
}

// ClassifiedError wraps an error with its classification.
type ClassifiedError struct {
	Kind       ErrorKind
	StatusCode int
	Err        error
}

func (e *ClassifiedError) Error() string {
	return e.Err.Error()
}

func (e *ClassifiedError) Unwrap() error {
	return e.Err
}

// ClassifyError categorizes an upstream error based on status code and response body.
func ClassifyError(err error, statusCode int, body string) ErrorKind {
	if err != nil && errors.Is(err, context.DeadlineExceeded) {
		return ErrTimeout
	}

	if err != nil && strings.Contains(err.Error(), "deadline exceeded") {
		return ErrTimeout
	}

	switch {
	case statusCode == 429:
		return ErrRateLimit
	case statusCode == 401 || statusCode == 403:
		return ErrAuth
	case statusCode >= 500:
		// Check for Anthropic overloaded error (529 maps here, but also check body)
		if containsAny(body, "overloaded_error", "overloaded") {
			return ErrRateLimit // treat overloaded as rate limit (transient)
		}
		return ErrServerError
	}

	// Pattern-match on known error strings from providers
	lower := strings.ToLower(body)

	switch {
	case containsAny(lower,
		"context_length_exceeded",
		"maximum context length",
		"max_tokens",
		"token limit",
		"context window",
		"too many tokens",
		"maximum number of tokens",
	):
		return ErrTokenLimit

	case containsAny(lower,
		"content_policy_violation",
		"content_filter",
		"safety",
		"blocked",
		"harm_category",
		"moderation",
		"responsible_ai",
	):
		return ErrContentPolicy

	case containsAny(lower,
		"rate_limit",
		"rate limit",
		"too many requests",
		"quota exceeded",
		"resource_exhausted",
	):
		return ErrRateLimit

	case containsAny(lower,
		"invalid_api_key",
		"invalid api key",
		"authentication",
		"unauthorized",
		"forbidden",
		"permission denied",
	):
		return ErrAuth
	}

	if statusCode >= 400 {
		return ErrUnknown
	}

	return ErrUnknown
}

// ProviderError carries structured error info from provider calls.
// Providers can optionally return this for better classification.
type ProviderError struct {
	Provider   string
	StatusCode int
	Body       string
	Err        error
}

func (e *ProviderError) Error() string {
	return e.Err.Error()
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}

// ClassifyFromError extracts status code and body from an error (either ProviderError
// or by parsing the common provider error format) and classifies it.
func ClassifyFromError(err error) ErrorKind {
	if err == nil {
		return ErrUnknown
	}

	// Try to extract structured ProviderError
	var pe *ProviderError
	if errors.As(err, &pe) {
		return ClassifyError(pe.Err, pe.StatusCode, pe.Body)
	}

	// Parse the common format: "<provider> returned status <code>: <body>"
	statusCode, body := parseProviderErrorString(err.Error())

	return ClassifyError(err, statusCode, body)
}

// parseProviderErrorString extracts status code and body from error strings
// matching the pattern: "... returned status <code>: <body>" or "... status <code>: <body>"
func parseProviderErrorString(msg string) (int, string) {
	// Look for "status <code>:" pattern
	_, after, found := strings.Cut(msg, "status ")
	if !found {
		return 0, msg
	}

	codeStr, body, found := strings.Cut(after, ":")
	if !found {
		return 0, msg
	}

	codeStr = strings.TrimSpace(codeStr)

	var code int

	for _, c := range codeStr {
		if c >= '0' && c <= '9' {
			code = code*10 + int(c-'0')
		} else {
			return 0, msg
		}
	}

	return code, strings.TrimSpace(body)
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}

	return false
}
