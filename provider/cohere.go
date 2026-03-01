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

const cohereServiceName = "cohere"

// Cohere implements Provider and StreamingProvider for Cohere's Chat API v2.
type Cohere struct {
	apiKey  string
	timeout time.Duration
}

// NewCohere creates a Cohere provider with the given API key and timeout.
func NewCohere(apiKey string, timeout time.Duration) *Cohere {
	return &Cohere{apiKey: apiKey, timeout: timeout}
}

func (*Cohere) Name() string { return "cohere" }

func (*Cohere) Models() []string {
	return []string{"command-r-plus", "command-r", "command-a-03-2025", "command-light"}
}

// --- Cohere v2 request/response types ---

type cohereRequest struct {
	Model       string          `json:"model"`
	Messages    []cohereMessage `json:"messages"`
	Temperature *float64        `json:"temperature,omitempty"`
	MaxTokens   *int            `json:"max_tokens,omitempty"`
	P           *float64        `json:"p,omitempty"`
	StopSeqs    []string        `json:"stop_sequences,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
	Tools       []cohereTool    `json:"tools,omitempty"`
}

type cohereMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type cohereTool struct {
	Type     string       `json:"type"`
	Function cohereToolFn `json:"function"`
}

type cohereToolFn struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type cohereResponse struct {
	ID           string        `json:"id"`
	Message      cohereRespMsg `json:"message"`
	FinishReason string        `json:"finish_reason"`
	Usage        cohereUsage   `json:"usage"`
}

type cohereRespMsg struct {
	Role      string               `json:"role"`
	Content   []cohereContentBlock `json:"content"`
	ToolCalls []cohereToolCall     `json:"tool_calls"`
}

type cohereContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type cohereToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function cohereToolFnCall `json:"function"`
}

type cohereToolFnCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type cohereUsage struct {
	BilledUnits cohereTokenCounts `json:"billed_units"`
	Tokens      cohereTokenCounts `json:"tokens"`
}

type cohereTokenCounts struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// cohereStreamEvent is a single SSE event from the Cohere streaming API.
type cohereStreamEvent struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Index int    `json:"index"`
	Delta struct {
		Type         string `json:"type"`
		Text         string `json:"text"`
		FinishReason string `json:"finish_reason"`
		ToolCall     *struct {
			ID       string           `json:"id"`
			Type     string           `json:"type"`
			Function cohereToolFnCall `json:"function"`
		} `json:"tool_call"`
		Usage *cohereUsage `json:"usage"`
	} `json:"delta"`
}

func (c *Cohere) ChatCompletion(ctx *gofr.Context, req models.ChatCompletionRequest) (*models.ChatCompletionResponse, error) {
	svc := ctx.GetHTTPService(cohereServiceName)
	if svc == nil {
		return nil, fmt.Errorf("HTTP service %q not registered", cohereServiceName)
	}

	cReq := translateToCohere(req)

	body, err := json.Marshal(cReq)
	if err != nil {
		return nil, fmt.Errorf("marshal cohere request: %w", err)
	}

	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + c.apiKey,
	}

	resp, err := svc.PostWithHeaders(ctx, "v2/chat", nil, body, headers)
	if err != nil {
		return nil, fmt.Errorf("cohere request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read cohere response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("cohere returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var cResp cohereResponse
	if err := json.Unmarshal(respBody, &cResp); err != nil {
		return nil, fmt.Errorf("decode cohere response: %w", err)
	}

	return translateFromCohere(cResp, req.Model), nil
}

// ChatCompletionStream sends a streaming request to Cohere and translates SSE events
// into OpenAI-compatible StreamChunks.
func (c *Cohere) ChatCompletionStream(ctx *gofr.Context, req models.ChatCompletionRequest, onChunk func(models.StreamChunk)) error {
	svc := ctx.GetHTTPService(cohereServiceName)
	if svc == nil {
		return fmt.Errorf("HTTP service %q not registered", cohereServiceName)
	}

	cReq := translateToCohere(req)
	cReq.Stream = true

	body, err := json.Marshal(cReq)
	if err != nil {
		return fmt.Errorf("marshal cohere request: %w", err)
	}

	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + c.apiKey,
		"Accept":        "text/event-stream",
	}

	resp, err := svc.PostWithHeaders(ctx, "v2/chat", nil, body, headers)
	if err != nil {
		return fmt.Errorf("cohere stream request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cohere stream returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return parseCohereSSE(resp.Body, req.Model, onChunk)
}

// parseCohereSSE reads SSE events from the Cohere streaming API and invokes
// onChunk for each parsed token. Cohere uses "event: {type}" + "data: {json}" pairs.
func parseCohereSSE(r io.Reader, model string, onChunk func(models.StreamChunk)) error {
	scanner := bufio.NewScanner(r)
	streamID := "chatcmpl-" + uuid.NewString()
	created := time.Now().Unix()

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

	var lastEventType string

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "event: ") {
			lastEventType = strings.TrimPrefix(line, "event: ")
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		var event cohereStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		// Use the event type from the event line if the JSON field is missing
		if event.Type == "" {
			event.Type = lastEventType
		}

		switch event.Type {
		case "content-delta":
			if event.Delta.Type == "text" && event.Delta.Text != "" {
				onChunk(models.StreamChunk{
					ID:      streamID,
					Object:  "chat.completion.chunk",
					Created: created,
					Model:   model,
					Choices: []models.StreamChoice{
						{Index: 0, Delta: models.StreamDelta{Content: event.Delta.Text}},
					},
				})
			} else if event.Delta.Type == "tool-call" && event.Delta.ToolCall != nil {
				onChunk(models.StreamChunk{
					ID:      streamID,
					Object:  "chat.completion.chunk",
					Created: created,
					Model:   model,
					Choices: []models.StreamChoice{
						{Index: 0, Delta: models.StreamDelta{
							ToolCalls: []models.StreamToolCall{{
								Index: event.Index,
								ID:    event.Delta.ToolCall.ID,
								Type:  "function",
								Function: models.FunctionCall{
									Name:      event.Delta.ToolCall.Function.Name,
									Arguments: event.Delta.ToolCall.Function.Arguments,
								},
							}},
						}},
					},
				})
			}

		case "message-end":
			finishReason := translateCohereFinishReason(event.Delta.FinishReason)

			chunk := models.StreamChunk{
				ID:      streamID,
				Object:  "chat.completion.chunk",
				Created: created,
				Model:   model,
				Choices: []models.StreamChoice{
					{Index: 0, FinishReason: &finishReason},
				},
			}

			if event.Delta.Usage != nil {
				chunk.Usage = &models.Usage{
					PromptTokens:     event.Delta.Usage.BilledUnits.InputTokens,
					CompletionTokens: event.Delta.Usage.BilledUnits.OutputTokens,
					TotalTokens:      event.Delta.Usage.BilledUnits.InputTokens + event.Delta.Usage.BilledUnits.OutputTokens,
				}
			}

			onChunk(chunk)

			return nil
		}
	}

	return scanner.Err()
}

func translateToCohere(req models.ChatCompletionRequest) cohereRequest {
	cReq := cohereRequest{
		Model:       req.Model,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		P:           req.TopP,
		StopSeqs:    req.Stop,
	}

	// Convert tools
	for _, t := range req.Tools {
		cReq.Tools = append(cReq.Tools, cohereTool{
			Type: "function",
			Function: cohereToolFn{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				Parameters:  t.Function.Parameters,
			},
		})
	}

	for _, m := range req.Messages {
		switch m.Role {
		case "system":
			cReq.Messages = append(cReq.Messages, cohereMessage{
				Role:    "system",
				Content: m.Content,
			})

		case "tool":
			// Cohere tool results use role "tool" with a content list
			cReq.Messages = append(cReq.Messages, cohereMessage{
				Role: "tool",
				Content: []map[string]any{
					{"type": "tool_result", "tool_use_id": m.ToolCallID, "content": m.Content},
				},
			})

		case "assistant":
			if len(m.ToolCalls) > 0 {
				// Assistant message with tool calls: send tool_calls field
				var toolCalls []cohereToolCall
				for _, tc := range m.ToolCalls {
					toolCalls = append(toolCalls, cohereToolCall{
						ID:   tc.ID,
						Type: "function",
						Function: cohereToolFnCall{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					})
				}

				cReq.Messages = append(cReq.Messages, cohereMessage{
					Role:    "assistant",
					Content: map[string]any{"tool_calls": toolCalls, "text": m.Content},
				})
			} else {
				cReq.Messages = append(cReq.Messages, cohereMessage{
					Role:    "assistant",
					Content: m.Content,
				})
			}

		default:
			cReq.Messages = append(cReq.Messages, cohereMessage{
				Role:    m.Role,
				Content: m.Content,
			})
		}
	}

	return cReq
}

func translateFromCohere(resp cohereResponse, model string) *models.ChatCompletionResponse {
	// Extract text content from the content blocks
	var content string
	for _, block := range resp.Message.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	// Extract tool calls
	var toolCalls []models.ToolCall
	for _, tc := range resp.Message.ToolCalls {
		toolCalls = append(toolCalls, models.ToolCall{
			ID:   tc.ID,
			Type: "function",
			Function: models.FunctionCall{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		})
	}

	finishReason := translateCohereFinishReason(resp.FinishReason)
	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}

	return &models.ChatCompletionResponse{
		ID:      "chatcmpl-" + uuid.NewString(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []models.Choice{
			{
				Index:        0,
				Message:      models.Message{Role: "assistant", Content: content},
				FinishReason: finishReason,
				ToolCalls:    toolCalls,
			},
		},
		Usage: models.Usage{
			PromptTokens:     resp.Usage.BilledUnits.InputTokens,
			CompletionTokens: resp.Usage.BilledUnits.OutputTokens,
			TotalTokens:      resp.Usage.BilledUnits.InputTokens + resp.Usage.BilledUnits.OutputTokens,
		},
		Provider: "cohere",
	}
}

func translateCohereFinishReason(reason string) string {
	switch reason {
	case "COMPLETE":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "TOOL_CALL":
		return "tool_calls"
	case "ERROR", "ERROR_TOXIC":
		return "content_filter"
	default:
		return "stop"
	}
}
