package handler

import (
	"fmt"
	"strings"

	"examples/llm-gateway/models"
)

// ValidateTools checks that tool definitions are well-formed.
func ValidateTools(tools []models.Tool) error {
	var errs []string

	for i, t := range tools {
		if t.Type != "function" {
			errs = append(errs, fmt.Sprintf("tools[%d]: type must be \"function\", got %q", i, t.Type))
		}

		if t.Function.Name == "" {
			errs = append(errs, fmt.Sprintf("tools[%d]: function.name is required", i))
		}

		if t.Function.Parameters != nil {
			if m, ok := t.Function.Parameters.(map[string]any); ok {
				if _, hasType := m["type"]; !hasType {
					errs = append(errs, fmt.Sprintf("tools[%d]: function.parameters should have a \"type\" field", i))
				}
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("tool validation: %s", strings.Join(errs, "; "))
	}

	return nil
}
