package guardrails

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"examples/llm-gateway/models"
)

// --- PII tests ---

func TestDetectPII_Email(t *testing.T) {
	matches := DetectPII("Contact me at user@example.com please")
	require.Len(t, matches, 1)
	assert.Equal(t, PIIEmail, matches[0].Type)
	assert.Equal(t, "user@example.com", matches[0].Value)
}

func TestDetectPII_Phone(t *testing.T) {
	matches := DetectPII("Call me at (555) 123-4567")
	require.Len(t, matches, 1)
	assert.Equal(t, PIIPhone, matches[0].Type)
}

func TestDetectPII_SSN(t *testing.T) {
	matches := DetectPII("My SSN is 123-45-6789")
	require.Len(t, matches, 1)
	assert.Equal(t, PIISSN, matches[0].Type)
	assert.Equal(t, "123-45-6789", matches[0].Value)
}

func TestDetectPII_CreditCard(t *testing.T) {
	matches := DetectPII("Card: 4111 1111 1111 1111")
	require.Len(t, matches, 1)
	assert.Equal(t, PIICreditCard, matches[0].Type)
}

func TestDetectPII_IPAddress(t *testing.T) {
	matches := DetectPII("Server at 192.168.1.100")
	require.Len(t, matches, 1)
	assert.Equal(t, PIIIPAddress, matches[0].Type)
	assert.Equal(t, "192.168.1.100", matches[0].Value)
}

func TestDetectPII_Multiple(t *testing.T) {
	text := "Email: test@mail.com, SSN: 123-45-6789, IP: 10.0.0.1"
	matches := DetectPII(text)
	assert.GreaterOrEqual(t, len(matches), 3)
}

func TestDetectPII_NoMatch(t *testing.T) {
	matches := DetectPII("This is a normal text with no PII")
	assert.Empty(t, matches)
}

func TestContainsPII(t *testing.T) {
	assert.True(t, ContainsPII("My email is a@b.com"))
	assert.True(t, ContainsPII("SSN: 123-45-6789"))
	assert.False(t, ContainsPII("Hello world"))
}

func TestRedactPII(t *testing.T) {
	text := "Contact user@example.com or call 555-123-4567, SSN 123-45-6789"
	redacted := RedactPII(text)
	assert.Contains(t, redacted, "[REDACTED_EMAIL]")
	assert.Contains(t, redacted, "[REDACTED_PHONE]")
	assert.Contains(t, redacted, "[REDACTED_SSN]")
	assert.NotContains(t, redacted, "user@example.com")
	assert.NotContains(t, redacted, "123-45-6789")
}

// --- Keyword tests ---

func TestCheckKeywords_Match(t *testing.T) {
	err := CheckKeywords([]string{"badword", "forbidden"}, []string{"This contains a badword in it"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "badword")
}

func TestCheckKeywords_CaseInsensitive(t *testing.T) {
	err := CheckKeywords([]string{"BadWord"}, []string{"this has badword here"})
	require.Error(t, err)
}

func TestCheckKeywords_NoMatch(t *testing.T) {
	err := CheckKeywords([]string{"badword"}, []string{"This is clean text"})
	assert.NoError(t, err)
}

func TestCheckKeywords_EmptyBlocked(t *testing.T) {
	err := CheckKeywords(nil, []string{"anything"})
	assert.NoError(t, err)
}

func TestCheckKeywords_EmptyContent(t *testing.T) {
	err := CheckKeywords([]string{"word"}, nil)
	assert.NoError(t, err)
}

// --- Guardrail Check/Filter tests ---

func TestCheck_Disabled(t *testing.T) {
	cfg := &GuardrailConfig{Enabled: false}
	err := Check(cfg, []models.Message{{Role: "user", Content: "anything"}})
	assert.NoError(t, err)
}

func TestCheck_Nil(t *testing.T) {
	err := Check(nil, []models.Message{{Role: "user", Content: "anything"}})
	assert.NoError(t, err)
}

func TestCheck_BlockedKeyword(t *testing.T) {
	cfg := &GuardrailConfig{
		Enabled:         true,
		BlockedKeywords: []string{"hack", "exploit"},
	}
	err := Check(cfg, []models.Message{{Role: "user", Content: "How to hack a system"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "guardrail")
	assert.Contains(t, err.Error(), "hack")
}

func TestCheck_PIIBlock(t *testing.T) {
	cfg := &GuardrailConfig{
		Enabled:   true,
		PIIAction: "block",
	}
	err := Check(cfg, []models.Message{{Role: "user", Content: "My SSN is 123-45-6789"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PII")
	assert.Contains(t, err.Error(), "SSN")
}

func TestCheck_PIIRedactDoesNotBlock(t *testing.T) {
	cfg := &GuardrailConfig{
		Enabled:   true,
		PIIAction: "redact",
	}
	err := Check(cfg, []models.Message{{Role: "user", Content: "My SSN is 123-45-6789"}})
	assert.NoError(t, err) // redact mode only filters output, doesn't block input
}

func TestCheck_InputTokenLimit(t *testing.T) {
	cfg := &GuardrailConfig{
		Enabled:        true,
		MaxInputTokens: 10,
	}
	// ~10 tokens = ~40 chars, so 200 chars >> 10 tokens
	longContent := strings.Repeat("word ", 50)
	err := Check(cfg, []models.Message{{Role: "user", Content: longContent}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "input too long")
}

func TestCheck_InputTokenLimit_Under(t *testing.T) {
	cfg := &GuardrailConfig{
		Enabled:        true,
		MaxInputTokens: 1000,
	}
	err := Check(cfg, []models.Message{{Role: "user", Content: "Short message"}})
	assert.NoError(t, err)
}

func TestFilter_RedactOutput(t *testing.T) {
	cfg := &GuardrailConfig{
		Enabled:   true,
		PIIAction: "redact",
	}
	resp := &models.ChatCompletionResponse{
		Choices: []models.Choice{
			{Message: models.Message{Content: "Your email is test@example.com"}},
		},
	}
	result := Filter(nil, cfg, resp)
	assert.Contains(t, result.Choices[0].Message.Content, "[REDACTED_EMAIL]")
	assert.NotContains(t, result.Choices[0].Message.Content, "test@example.com")
}

func TestFilter_OutputTokenLimit(t *testing.T) {
	cfg := &GuardrailConfig{
		Enabled:         true,
		MaxOutputTokens: 5,
	}
	resp := &models.ChatCompletionResponse{
		Choices: []models.Choice{
			{Message: models.Message{Content: strings.Repeat("a", 100)}},
		},
	}
	result := Filter(nil, cfg, resp)
	assert.Contains(t, result.Choices[0].Message.Content, "[truncated by guardrail]")
	assert.Equal(t, "length", result.Choices[0].FinishReason)
}

func TestFilter_Disabled(t *testing.T) {
	cfg := &GuardrailConfig{Enabled: false}
	resp := &models.ChatCompletionResponse{
		Choices: []models.Choice{
			{Message: models.Message{Content: "test@email.com"}},
		},
	}
	result := Filter(nil, cfg, resp)
	assert.Equal(t, "test@email.com", result.Choices[0].Message.Content)
}

func TestFilter_Nil(t *testing.T) {
	result := Filter(nil, nil, nil)
	assert.Nil(t, result)
}

// --- ParseGlobalConfig tests ---

func TestParseGlobalConfig_Disabled(t *testing.T) {
	cfg := ParseGlobalConfig(func(k, d string) string { return d })
	assert.False(t, cfg.Enabled)
}

func TestParseGlobalConfig_Enabled(t *testing.T) {
	env := map[string]string{
		"GUARDRAIL_ENABLED":          "true",
		"GUARDRAIL_BLOCKED_KEYWORDS": "hack,exploit",
		"GUARDRAIL_PII_ACTION":       "block",
		"GUARDRAIL_MAX_INPUT_TOKENS": "1000",
	}
	cfg := ParseGlobalConfig(func(k, d string) string {
		if v, ok := env[k]; ok {
			return v
		}

		return d
	})
	assert.True(t, cfg.Enabled)
	assert.Equal(t, []string{"hack", "exploit"}, cfg.BlockedKeywords)
	assert.Equal(t, "block", cfg.PIIAction)
	assert.Equal(t, 1000, cfg.MaxInputTokens)
}
