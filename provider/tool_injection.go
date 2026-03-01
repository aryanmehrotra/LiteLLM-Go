package provider

import (
	"encoding/json"
	"fmt"
	"strings"

	"examples/llm-gateway/models"
)

// InjectToolsAsPrompt converts tool definitions into a system prompt and clears the tools field.
// Used for models that don't natively support function calling.
func InjectToolsAsPrompt(req *models.ChatCompletionRequest) {
	if len(req.Tools) == 0 {
		return
	}

	var sb strings.Builder

	sb.WriteString("You have access to the following tools. To use a tool, respond with a JSON block in this exact format:\n")
	sb.WriteString("```tool_call\n{\"name\": \"function_name\", \"arguments\": {\"arg1\": \"value1\"}}\n```\n\n")
	sb.WriteString("Available tools:\n")

	for _, t := range req.Tools {
		sb.WriteString(fmt.Sprintf("\n### %s\n", t.Function.Name))

		if t.Function.Description != "" {
			sb.WriteString(t.Function.Description + "\n")
		}

		if t.Function.Parameters != nil {
			paramBytes, err := json.MarshalIndent(t.Function.Parameters, "", "  ")
			if err == nil {
				sb.WriteString("Parameters:\n```json\n")
				sb.Write(paramBytes)
				sb.WriteString("\n```\n")
			}
		}
	}

	// Prepend as system message
	systemMsg := models.Message{
		Role:    "system",
		Content: sb.String(),
	}

	// Check if there's already a system message and append to it
	hasSystem := false

	for i, m := range req.Messages {
		if m.Role == "system" {
			req.Messages[i].Content += "\n\n" + systemMsg.Content
			hasSystem = true

			break
		}
	}

	if !hasSystem {
		req.Messages = append([]models.Message{systemMsg}, req.Messages...)
	}

	req.Tools = nil
	req.ToolChoice = nil
}
