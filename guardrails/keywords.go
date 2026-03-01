package guardrails

import (
	"fmt"
	"strings"
)

// CheckKeywords scans message contents for blocked keywords (case-insensitive substring match).
// Returns an error listing the first matched keyword if found.
func CheckKeywords(blocked []string, contents []string) error {
	if len(blocked) == 0 {
		return nil
	}

	for _, content := range contents {
		lower := strings.ToLower(content)

		for _, kw := range blocked {
			kw = strings.TrimSpace(kw)
			if kw == "" {
				continue
			}

			if strings.Contains(lower, strings.ToLower(kw)) {
				return fmt.Errorf("content contains blocked keyword: %q", kw)
			}
		}
	}

	return nil
}
