package provider

import (
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	"aryanmehrotra/llm-gateway/models"
)

// ExtractToolCalls parses tool call JSON blocks from response text content.
// Looks for blocks delimited by ```tool_call ... ``` markers.
func ExtractToolCalls(content string) []models.ToolCall {
	var calls []models.ToolCall

	remaining := content

	for {
		start := strings.Index(remaining, "```tool_call")
		if start == -1 {
			break
		}

		// Skip the marker line
		jsonStart := strings.Index(remaining[start:], "\n")
		if jsonStart == -1 {
			break
		}

		jsonStart += start + 1

		end := strings.Index(remaining[jsonStart:], "```")
		if end == -1 {
			break
		}

		jsonBlock := strings.TrimSpace(remaining[jsonStart : jsonStart+end])

		var parsed struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}

		if err := json.Unmarshal([]byte(jsonBlock), &parsed); err == nil && parsed.Name != "" {
			argsBytes, _ := json.Marshal(parsed.Arguments)
			calls = append(calls, models.ToolCall{
				ID:   "call_" + uuid.NewString()[:8],
				Type: "function",
				Function: models.FunctionCall{
					Name:      parsed.Name,
					Arguments: string(argsBytes),
				},
			})
		}

		remaining = remaining[jsonStart+end+3:]
	}

	return calls
}
