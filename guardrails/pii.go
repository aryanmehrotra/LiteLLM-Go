package guardrails

import (
	"regexp"
	"strings"
)

// PIIType identifies the category of detected PII.
type PIIType string

const (
	PIIEmail      PIIType = "EMAIL"
	PIIPhone      PIIType = "PHONE"
	PIISSN        PIIType = "SSN"
	PIICreditCard PIIType = "CREDIT_CARD"
	PIIIPAddress  PIIType = "IP_ADDRESS"
)

// PIIMatch represents a single PII detection.
type PIIMatch struct {
	Type  PIIType
	Value string
	Start int
	End   int
}

var piiPatterns = []struct {
	Type    PIIType
	Pattern *regexp.Regexp
}{
	{PIIEmail, regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)},
	{PIIPhone, regexp.MustCompile(`\b(?:\+1[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}\b`)},
	{PIISSN, regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)},
	{PIICreditCard, regexp.MustCompile(`\b(?:4\d{3}|5[1-5]\d{2}|3[47]\d{2}|6(?:011|5\d{2}))[-\s]?\d{4}[-\s]?\d{4}[-\s]?\d{4}\b`)},
	{PIIIPAddress, regexp.MustCompile(`\b(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\.(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\.(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\.(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\b`)},
}

var piiReplacements = map[PIIType]string{
	PIIEmail:      "[REDACTED_EMAIL]",
	PIIPhone:      "[REDACTED_PHONE]",
	PIISSN:        "[REDACTED_SSN]",
	PIICreditCard: "[REDACTED_CREDIT_CARD]",
	PIIIPAddress:  "[REDACTED_IP]",
}

// DetectPII scans text and returns all PII matches found.
func DetectPII(text string) []PIIMatch {
	var matches []PIIMatch

	for _, p := range piiPatterns {
		locs := p.Pattern.FindAllStringIndex(text, -1)
		for _, loc := range locs {
			matches = append(matches, PIIMatch{
				Type:  p.Type,
				Value: text[loc[0]:loc[1]],
				Start: loc[0],
				End:   loc[1],
			})
		}
	}

	return matches
}

// ContainsPII returns true if the text contains any PII patterns.
func ContainsPII(text string) bool {
	for _, p := range piiPatterns {
		if p.Pattern.MatchString(text) {
			return true
		}
	}

	return false
}

// RedactPII replaces all detected PII with redaction placeholders.
func RedactPII(text string) string {
	result := text

	for _, p := range piiPatterns {
		replacement := piiReplacements[p.Type]
		result = p.Pattern.ReplaceAllString(result, replacement)
	}

	return result
}

// redactMessagesContent applies PII redaction to all message content strings.
func redactMessagesContent(messages []messageRef) {
	for _, m := range messages {
		if ContainsPII(m.Content()) {
			m.SetContent(RedactPII(m.Content()))
		}
	}
}

// messageRef is an interface for accessing message content generically.
type messageRef interface {
	Content() string
	SetContent(s string)
}

// stringRef wraps a *string to implement messageRef.
type stringRef struct {
	s *string
}

func (r stringRef) Content() string      { return *r.s }
func (r stringRef) SetContent(s string)  { *r.s = s }

// messagesAsRefs extracts content references from a slice of messages (any struct with Content field).
func messagesAsRefs(contents []*string) []messageRef {
	refs := make([]messageRef, 0, len(contents))
	for _, c := range contents {
		if c != nil && strings.TrimSpace(*c) != "" {
			refs = append(refs, stringRef{s: c})
		}
	}

	return refs
}
