package provider

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"gofr.dev/pkg/gofr"

	"aryanmehrotra/llm-gateway/models"
)

const anthropicServiceName = "anthropic"

// Anthropic implements Provider and StreamingProvider for Anthropic's Messages API.
type Anthropic struct {
	apiKey  string
	timeout time.Duration
}

// NewAnthropic creates an Anthropic provider with the given API key and timeout.
func NewAnthropic(apiKey string, timeout time.Duration) *Anthropic {
	return &Anthropic{apiKey: apiKey, timeout: timeout}
}

func (*Anthropic) Name() string { return "anthropic" }

func (*Anthropic) Models() []string {
	return []string{"claude-sonnet-4-20250514", "claude-haiku-4-20250414", "claude-3-5-sonnet-20241022"}
}

// anthropicRequest is the Anthropic Messages API request format.
type anthropicRequest struct {
	Model       string               `json:"model"`
	MaxTokens   int                  `json:"max_tokens"`
	System      string               `json:"system,omitempty"`
	Messages    []anthropicMsg       `json:"messages"`
	Temperature *float64             `json:"temperature,omitempty"`
	TopP        *float64             `json:"top_p,omitempty"`
	Stop        []string             `json:"stop_sequences,omitempty"`
	Stream      bool                 `json:"stream,omitempty"`
	Tools       []anthropicTool      `json:"tools,omitempty"`
	ToolChoice  *anthropicToolChoice `json:"tool_choice,omitempty"`
}

type anthropicMsg struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type anthropicTool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"input_schema"`
}

type anthropicToolChoice struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

// anthropicContentBlock is used for structured content arrays in messages
// (e.g. tool_result blocks, tool_use blocks).
type anthropicContentBlock struct {
	Type      string `json:"type"`
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
	// For tool_use in requests (sending tool calls back)
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Input any    `json:"input,omitempty"`
	// For text blocks
	Text string `json:"text,omitempty"`
}

// anthropicResponse is the Anthropic Messages API response format.
type anthropicResponse struct {
	ID         string             `json:"id"`
	Type       string             `json:"type"`
	Role       string             `json:"role"`
	Content    []anthropicContent `json:"content"`
	Model      string             `json:"model"`
	StopReason string             `json:"stop_reason"`
	Usage      anthropicUsage     `json:"usage"`
}

type anthropicContent struct {
	Type  string `json:"type"`
	Text  string `json:"text,omitempty"`
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Input any    `json:"input,omitempty"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

func (a *Anthropic) ChatCompletion(ctx *gofr.Context, req models.ChatCompletionRequest) (*models.ChatCompletionResponse, error) {
	svc := ctx.GetHTTPService(anthropicServiceName)
	if svc == nil {
		return nil, fmt.Errorf("HTTP service %q not registered", anthropicServiceName)
	}

	aReq := translateToAnthropic(req)

	body, err := json.Marshal(aReq)
	if err != nil {
		return nil, fmt.Errorf("marshal anthropic request: %w", err)
	}

	headers := map[string]string{
		"Content-Type":      "application/json",
		"x-api-key":         a.apiKey,
		"anthropic-version": "2023-06-01",
	}

	resp, err := svc.PostWithHeaders(ctx, "v1/messages", nil, body, headers)
	if err != nil {
		return nil, fmt.Errorf("anthropic request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read anthropic response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("anthropic returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var aResp anthropicResponse
	if err := json.Unmarshal(respBody, &aResp); err != nil {
		return nil, fmt.Errorf("decode anthropic response: %w", err)
	}

	return translateFromAnthropic(aResp), nil
}

// ChatCompletionStream sends a streaming request to Anthropic via GoFr's HTTP
// service and translates SSE events into OpenAI-compatible StreamChunks.
func (a *Anthropic) ChatCompletionStream(ctx *gofr.Context, req models.ChatCompletionRequest, onChunk func(models.StreamChunk)) error {
	svc := ctx.GetHTTPService(anthropicServiceName)
	if svc == nil {
		return fmt.Errorf("HTTP service %q not registered", anthropicServiceName)
	}

	aReq := translateToAnthropic(req)
	aReq.Stream = true

	body, err := json.Marshal(aReq)
	if err != nil {
		return fmt.Errorf("marshal anthropic request: %w", err)
	}

	headers := map[string]string{
		"Content-Type":      "application/json",
		"x-api-key":         a.apiKey,
		"anthropic-version": "2023-06-01",
		"Accept":            "text/event-stream",
	}

	resp, err := svc.PostWithHeaders(ctx, "v1/messages", nil, body, headers)
	if err != nil {
		return fmt.Errorf("anthropic stream request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("anthropic stream returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return parseAnthropicSSE(resp.Body, req.Model, onChunk)
}

// anthropicSSEEvent represents a parsed SSE event from Anthropic's streaming API.
type anthropicSSEEvent struct {
	Type         string `json:"type"`
	Index        int    `json:"index"`
	ContentBlock struct {
		Type string `json:"type"`
		ID   string `json:"id,omitempty"`
		Name string `json:"name,omitempty"`
	} `json:"content_block"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		PartialJSON string `json:"partial_json"`
		StopReason  string `json:"stop_reason"`
	} `json:"delta"`
}

func parseAnthropicSSE(r io.Reader, model string, onChunk func(models.StreamChunk)) error {
	scanner := bufio.NewScanner(r)
	streamID := "chatcmpl-" + uuid.NewString()
	created := time.Now().Unix()

	// Track tool call index for streaming
	toolCallIndex := 0

	// Send initial role chunk
	onChunk(models.StreamChunk{
		ID:      streamID,
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   model,
		Choices: []models.StreamChoice{
			{Index: 0, Delta: models.StreamDelta{Role: "assistant"}},
		},
	})

	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		var event anthropicSSEEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "content_block_start":
			if event.ContentBlock.Type == "tool_use" {
				onChunk(models.StreamChunk{
					ID:      streamID,
					Object:  "chat.completion.chunk",
					Created: created,
					Model:   model,
					Choices: []models.StreamChoice{
						{Index: 0, Delta: models.StreamDelta{
							ToolCalls: []models.StreamToolCall{{
								Index: toolCallIndex,
								ID:    event.ContentBlock.ID,
								Type:  "function",
								Function: models.FunctionCall{
									Name:      event.ContentBlock.Name,
									Arguments: "",
								},
							}},
						}},
					},
				})
			}

		case "content_block_delta":
			if event.Delta.Type == "text_delta" && event.Delta.Text != "" {
				onChunk(models.StreamChunk{
					ID:      streamID,
					Object:  "chat.completion.chunk",
					Created: created,
					Model:   model,
					Choices: []models.StreamChoice{
						{Index: 0, Delta: models.StreamDelta{Content: event.Delta.Text}},
					},
				})
			} else if event.Delta.Type == "input_json_delta" && event.Delta.PartialJSON != "" {
				onChunk(models.StreamChunk{
					ID:      streamID,
					Object:  "chat.completion.chunk",
					Created: created,
					Model:   model,
					Choices: []models.StreamChoice{
						{Index: 0, Delta: models.StreamDelta{
							ToolCalls: []models.StreamToolCall{{
								Index:    toolCallIndex,
								Function: models.FunctionCall{Arguments: event.Delta.PartialJSON},
							}},
						}},
					},
				})
			}

		case "content_block_stop":
			// If we were streaming a tool_use block, increment the index for the next one
			if event.Index >= 0 {
				toolCallIndex++
			}

		case "message_delta":
			finishReason := "stop"

			switch event.Delta.StopReason {
			case "max_tokens":
				finishReason = "length"
			case "tool_use":
				finishReason = "tool_calls"
			}

			onChunk(models.StreamChunk{
				ID:      streamID,
				Object:  "chat.completion.chunk",
				Created: created,
				Model:   model,
				Choices: []models.StreamChoice{
					{Index: 0, FinishReason: &finishReason},
				},
			})

		case "message_stop":
			return nil
		}
	}

	return scanner.Err()
}

func translateToAnthropic(req models.ChatCompletionRequest) anthropicRequest {
	aReq := anthropicRequest{
		Model:       req.Model,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stop:        req.Stop,
	}

	if req.MaxTokens != nil {
		aReq.MaxTokens = *req.MaxTokens
	} else {
		aReq.MaxTokens = 1024
	}

	// Convert tools
	for _, t := range req.Tools {
		aReq.Tools = append(aReq.Tools, anthropicTool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			InputSchema: t.Function.Parameters,
		})
	}

	// Convert tool_choice
	if req.ToolChoice != nil && len(aReq.Tools) > 0 {
		switch v := req.ToolChoice.(type) {
		case string:
			switch v {
			case "auto":
				aReq.ToolChoice = &anthropicToolChoice{Type: "auto"}
			case "none":
				// Anthropic doesn't have "none" — omit tools instead
				aReq.Tools = nil
				aReq.ToolChoice = nil
			case "required":
				aReq.ToolChoice = &anthropicToolChoice{Type: "any"}
			}
		case map[string]any:
			if fn, ok := v["function"].(map[string]any); ok {
				if name, ok := fn["name"].(string); ok {
					aReq.ToolChoice = &anthropicToolChoice{Type: "tool", Name: name}
				}
			}
		}
	}

	for _, m := range req.Messages {
		if m.Role == "system" {
			aReq.System = m.Content
			continue
		}

		// Handle tool result messages
		if m.Role == "tool" {
			aReq.Messages = append(aReq.Messages, anthropicMsg{
				Role: "user",
				Content: []anthropicContentBlock{{
					Type:      "tool_result",
					ToolUseID: m.ToolCallID,
					Content:   m.Content,
				}},
			})

			continue
		}

		// Handle assistant messages with tool_calls
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			var blocks []anthropicContentBlock
			if m.Content != "" {
				blocks = append(blocks, anthropicContentBlock{Type: "text", Text: m.Content})
			}

			for _, tc := range m.ToolCalls {
				var input any

				_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)

				blocks = append(blocks, anthropicContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: input,
				})
			}

			aReq.Messages = append(aReq.Messages, anthropicMsg{
				Role:    "assistant",
				Content: blocks,
			})

			continue
		}

		aReq.Messages = append(aReq.Messages, anthropicMsg{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	return aReq
}

func translateFromAnthropic(resp anthropicResponse) *models.ChatCompletionResponse {
	var content string
	var toolCalls []models.ToolCall

	for _, c := range resp.Content {
		switch c.Type {
		case "text":
			content = c.Text
		case "tool_use":
			argsBytes, _ := json.Marshal(c.Input)
			toolCalls = append(toolCalls, models.ToolCall{
				ID:   c.ID,
				Type: "function",
				Function: models.FunctionCall{
					Name:      c.Name,
					Arguments: string(argsBytes),
				},
			})
		}
	}

	finishReason := "stop"

	switch resp.StopReason {
	case "max_tokens":
		finishReason = "length"
	case "tool_use":
		finishReason = "tool_calls"
	}

	choice := models.Choice{
		Index:        0,
		Message:      models.Message{Role: "assistant", Content: content},
		FinishReason: finishReason,
		ToolCalls:    toolCalls,
	}

	return &models.ChatCompletionResponse{
		ID:      "chatcmpl-" + uuid.NewString(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   resp.Model,
		Choices: []models.Choice{choice},
		Usage: models.Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
		Provider: "anthropic",
	}
}
