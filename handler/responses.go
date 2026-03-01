package handler

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gofr.dev/pkg/gofr"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/response"

	"aryanmehrotra/litellm-go/models"
	"aryanmehrotra/litellm-go/websearch"
)

// CreateResponse handles POST /v1/responses (OpenAI Responses API).
// It translates the Responses API format into chat completions internally,
// executes built-in tools (web_search_preview), and returns the result
// in Responses API format.
func (h *APIHandler) CreateResponse() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		var req models.ResponseRequest
		if err := ctx.Bind(&req); err != nil {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"request body"}}
		}

		if req.Model == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"model"}}
		}

		if req.Input == nil {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"input"}}
		}

		// Translate Responses API input to chat messages
		messages, err := translateResponseInput(req)
		if err != nil {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"input"}}
		}

		// Build the internal chat completion request
		chatReq := models.ChatCompletionRequest{
			Model:       req.Model,
			Messages:    messages,
			Temperature: req.Temperature,
			MaxTokens:   req.MaxOutputTokens,
		}

		// Translate built-in tools to function tools where possible
		var hasWebSearch bool
		var functionTools []models.Tool

		for _, t := range req.Tools {
			switch t.Type {
			case "web_search_preview":
				hasWebSearch = true
			case "function":
				functionTools = append(functionTools, models.Tool{
					Type: "function",
					Function: models.ToolFunction{
						Name:        t.Name,
						Description: t.Description,
						Parameters:  t.Parameters,
					},
				})
			}
		}

		chatReq.Tools = functionTools

		responseID := "resp_" + uuid.New().String()
		now := time.Now().Unix()

		var outputItems []models.ResponseOutputItem
		totalUsage := models.ResponseUsage{}

		// Execute web search if enabled and query detected
		if hasWebSearch && h.Search != nil {
			searchQuery := extractSearchQuery(messages)
			if searchQuery != "" {
				results := h.Search.SearchDirect(ctx, searchQuery, 5)
				if len(results) > 0 {
					searchOutputID := "ws_" + uuid.New().String()
					outputItems = append(outputItems, models.ResponseOutputItem{
						Type:   "web_search_call",
						ID:     searchOutputID,
						Status: "completed",
						Name:   "web_search_preview",
					})

					// Inject search results into context
					searchContext := websearch.FormatResults(results)
					chatReq.Messages = append([]models.Message{{
						Role:    "system",
						Content: "Web search results:\n" + searchContext,
					}}, chatReq.Messages...)
				}
			}
		}

		// Resolve provider
		p, modelName, err := h.Registry.ResolveProvider(req.Model)
		if err != nil {
			return nil, ErrInvalidParam("model", fmt.Sprintf("model %q not found", req.Model))
		}

		chatReq.Model = modelName

		// Route the request
		resp, err := h.Router.ChatCompletion(ctx, p, modelName, chatReq)
		if err != nil {
			ctx.Errorf("responses api provider error: %v", err)
			return nil, err
		}

		totalUsage.InputTokens = resp.Usage.PromptTokens
		totalUsage.OutputTokens = resp.Usage.CompletionTokens
		totalUsage.TotalTokens = resp.Usage.TotalTokens

		// Build output message
		if len(resp.Choices) > 0 {
			choice := resp.Choices[0]
			msgContent := []models.ResponseOutputContent{}

			if choice.Message.Content != "" {
				msgContent = append(msgContent, models.ResponseOutputContent{
					Type: "output_text",
					Text: choice.Message.Content,
				})
			}

			outputMsg := models.ResponseOutputItem{
				Type:    "message",
				ID:      "msg_" + uuid.New().String(),
				Status:  "completed",
				Role:    "assistant",
				Content: msgContent,
			}

			// Handle tool calls as function_call output items
			for _, tc := range choice.ToolCalls {
				outputItems = append(outputItems, models.ResponseOutputItem{
					Type:      "function_call",
					ID:        "fc_" + uuid.New().String(),
					Status:    "completed",
					Name:      tc.Function.Name,
					CallID:    tc.ID,
					Arguments: tc.Function.Arguments,
				})
			}

			outputItems = append(outputItems, outputMsg)
		}

		result := models.ResponseObject{
			ID:           responseID,
			Object:       "response",
			CreatedAt:    now,
			Model:        resp.Model,
			Status:       "completed",
			Output:       outputItems,
			Usage:        totalUsage,
			Instructions: req.Instructions,
			Metadata:     req.Metadata,
		}

		return response.Raw{Data: result}, nil
	}
}

// translateResponseInput converts a Responses API input to chat messages.
func translateResponseInput(req models.ResponseRequest) ([]models.Message, error) {
	var messages []models.Message

	// Add system/instructions message if present
	if req.Instructions != "" {
		messages = append(messages, models.Message{
			Role:    "system",
			Content: req.Instructions,
		})
	}

	switch v := req.Input.(type) {
	case string:
		messages = append(messages, models.Message{
			Role:    "user",
			Content: v,
		})

	case []any:
		for _, item := range v {
			itemMap, ok := item.(map[string]any)
			if !ok {
				continue
			}

			role, _ := itemMap["role"].(string)
			if role == "" {
				role = "user"
			}

			var content string

			// Content can be a string or array of content blocks
			switch c := itemMap["content"].(type) {
			case string:
				content = c
			case []any:
				for _, block := range c {
					blockMap, ok := block.(map[string]any)
					if !ok {
						continue
					}

					blockType, _ := blockMap["type"].(string)
					if blockType == "input_text" {
						if text, ok := blockMap["text"].(string); ok {
							content += text
						}
					}
				}
			}

			messages = append(messages, models.Message{
				Role:    role,
				Content: content,
			})
		}

	case map[string]any:
		// Single item object
		role, _ := v["role"].(string)
		if role == "" {
			role = "user"
		}

		content, _ := v["content"].(string)
		messages = append(messages, models.Message{
			Role:    role,
			Content: content,
		})

	default:
		// Try JSON re-marshal for interface{} types
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}

		var items []models.ResponseInputItem
		if err := json.Unmarshal(b, &items); err == nil {
			for _, item := range items {
				var content string
				for _, c := range item.Content {
					if c.Type == "input_text" {
						content += c.Text
					}
				}

				messages = append(messages, models.Message{
					Role:    item.Role,
					Content: content,
				})
			}
		} else {
			// Last resort: treat as plain text
			messages = append(messages, models.Message{
				Role:    "user",
				Content: string(b),
			})
		}
	}

	return messages, nil
}

// extractSearchQuery extracts a search query from the user messages.
func extractSearchQuery(messages []models.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" && messages[i].Content != "" {
			return messages[i].Content
		}
	}

	return ""
}
