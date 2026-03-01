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

	"examples/llm-gateway/models"
)

const geminiServiceName = "gemini"

// Gemini implements Provider and StreamingProvider for Google's Gemini API.
type Gemini struct {
	apiKey  string
	timeout time.Duration
}

// NewGemini creates a Gemini provider with the given API key and timeout.
func NewGemini(apiKey string, timeout time.Duration) *Gemini {
	return &Gemini{apiKey: apiKey, timeout: timeout}
}

func (*Gemini) Name() string { return "gemini" }

func (*Gemini) Models() []string {
	return []string{"gemini-2.0-flash", "gemini-2.0-flash-lite", "gemini-1.5-pro", "gemini-1.5-flash"}
}

// --- Gemini request/response types ---

type geminiRequest struct {
	Contents          []geminiContent          `json:"contents"`
	SystemInstruction *geminiSystemInstruction `json:"systemInstruction,omitempty"`
	GenerationConfig  *geminiGenConfig         `json:"generationConfig,omitempty"`
	Tools             []geminiToolDeclaration  `json:"tools,omitempty"`
	ToolConfig        *geminiToolConfig        `json:"toolConfig,omitempty"`
}

type geminiToolDeclaration struct {
	FunctionDeclarations []geminiFunctionDecl `json:"functionDeclarations"`
}

type geminiFunctionDecl struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type geminiToolConfig struct {
	FunctionCallingConfig *geminiFunctionCallingConfig `json:"functionCallingConfig,omitempty"`
}

type geminiFunctionCallingConfig struct {
	Mode                 string   `json:"mode"`
	AllowedFunctionNames []string `json:"allowedFunctionNames,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text             string                `json:"text,omitempty"`
	FunctionCall     *geminiFunctionCall   `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResp   `json:"functionResponse,omitempty"`
}

type geminiFunctionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

type geminiFunctionResp struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type geminiSystemInstruction struct {
	Parts []geminiPart `json:"parts"`
}

type geminiGenConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
}

type geminiResponse struct {
	Candidates    []geminiCandidate   `json:"candidates"`
	UsageMetadata *geminiUsageMetadata `json:"usageMetadata,omitempty"`
}

type geminiCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason"`
}

type geminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

func (g *Gemini) ChatCompletion(ctx *gofr.Context, req models.ChatCompletionRequest) (*models.ChatCompletionResponse, error) {
	svc := ctx.GetHTTPService(geminiServiceName)
	if svc == nil {
		return nil, fmt.Errorf("HTTP service %q not registered", geminiServiceName)
	}

	gReq := translateToGemini(req)

	body, err := json.Marshal(gReq)
	if err != nil {
		return nil, fmt.Errorf("marshal gemini request: %w", err)
	}

	path := fmt.Sprintf("v1beta/models/%s:generateContent", req.Model)

	headers := map[string]string{
		"Content-Type":  "application/json",
		"x-goog-api-key": g.apiKey,
	}

	resp, err := svc.PostWithHeaders(ctx, path, nil, body, headers)
	if err != nil {
		return nil, fmt.Errorf("gemini request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read gemini response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gemini returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var gResp geminiResponse
	if err := json.Unmarshal(respBody, &gResp); err != nil {
		return nil, fmt.Errorf("decode gemini response: %w", err)
	}

	return translateFromGemini(gResp, req.Model), nil
}

// ChatCompletionStream sends a streaming request to Gemini via GoFr's HTTP
// service and translates SSE events into OpenAI-compatible StreamChunks.
func (g *Gemini) ChatCompletionStream(ctx *gofr.Context, req models.ChatCompletionRequest, onChunk func(models.StreamChunk)) error {
	svc := ctx.GetHTTPService(geminiServiceName)
	if svc == nil {
		return fmt.Errorf("HTTP service %q not registered", geminiServiceName)
	}

	gReq := translateToGemini(req)

	body, err := json.Marshal(gReq)
	if err != nil {
		return fmt.Errorf("marshal gemini request: %w", err)
	}

	// Gemini streaming uses ?alt=sse query parameter
	path := fmt.Sprintf("v1beta/models/%s:streamGenerateContent?alt=sse", req.Model)

	headers := map[string]string{
		"Content-Type":  "application/json",
		"x-goog-api-key": g.apiKey,
		"Accept":        "text/event-stream",
	}

	resp, err := svc.PostWithHeaders(ctx, path, nil, body, headers)
	if err != nil {
		return fmt.Errorf("gemini stream request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("gemini stream returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return parseGeminiSSE(resp.Body, req.Model, onChunk)
}

func parseGeminiSSE(r io.Reader, model string, onChunk func(models.StreamChunk)) error {
	scanner := bufio.NewScanner(r)
	streamID := "chatcmpl-" + uuid.NewString()
	created := time.Now().Unix()
	firstChunk := true

	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		var gResp geminiResponse
		if err := json.Unmarshal([]byte(data), &gResp); err != nil {
			continue
		}

		if len(gResp.Candidates) == 0 {
			continue
		}

		candidate := gResp.Candidates[0]

		// Send role on first chunk
		if firstChunk {
			onChunk(models.StreamChunk{
				ID:      streamID,
				Object:  "chat.completion.chunk",
				Created: created,
				Model:   model,
				Choices: []models.StreamChoice{
					{Index: 0, Delta: models.StreamDelta{Role: "assistant"}},
				},
			})

			firstChunk = false
		}

		// Extract text and function calls from parts
		var text string
		var streamToolCalls []models.StreamToolCall
		toolIdx := 0

		for _, part := range candidate.Content.Parts {
			if part.FunctionCall != nil {
				argsBytes, _ := json.Marshal(part.FunctionCall.Args)
				streamToolCalls = append(streamToolCalls, models.StreamToolCall{
					Index: toolIdx,
					ID:    "call_" + uuid.NewString()[:8],
					Type:  "function",
					Function: models.FunctionCall{
						Name:      part.FunctionCall.Name,
						Arguments: string(argsBytes),
					},
				})
				toolIdx++
			} else {
				text += part.Text
			}
		}

		if text != "" {
			onChunk(models.StreamChunk{
				ID:      streamID,
				Object:  "chat.completion.chunk",
				Created: created,
				Model:   model,
				Choices: []models.StreamChoice{
					{Index: 0, Delta: models.StreamDelta{Content: text}},
				},
			})
		}

		if len(streamToolCalls) > 0 {
			onChunk(models.StreamChunk{
				ID:      streamID,
				Object:  "chat.completion.chunk",
				Created: created,
				Model:   model,
				Choices: []models.StreamChoice{
					{Index: 0, Delta: models.StreamDelta{ToolCalls: streamToolCalls}},
				},
			})
		}

		// Check for finish
		if candidate.FinishReason != "" {
			finishReason := translateGeminiFinishReason(candidate.FinishReason)
			if len(streamToolCalls) > 0 {
				finishReason = "tool_calls"
			}

			chunk := models.StreamChunk{
				ID:      streamID,
				Object:  "chat.completion.chunk",
				Created: created,
				Model:   model,
				Choices: []models.StreamChoice{
					{Index: 0, FinishReason: &finishReason},
				},
			}

			if gResp.UsageMetadata != nil {
				chunk.Usage = &models.Usage{
					PromptTokens:     gResp.UsageMetadata.PromptTokenCount,
					CompletionTokens: gResp.UsageMetadata.CandidatesTokenCount,
					TotalTokens:      gResp.UsageMetadata.TotalTokenCount,
				}
			}

			onChunk(chunk)
		}
	}

	return scanner.Err()
}

func translateToGemini(req models.ChatCompletionRequest) geminiRequest {
	gReq := geminiRequest{
		GenerationConfig: &geminiGenConfig{
			Temperature:     req.Temperature,
			TopP:            req.TopP,
			MaxOutputTokens: req.MaxTokens,
			StopSequences:   req.Stop,
		},
	}

	// Convert tools
	if len(req.Tools) > 0 {
		var decls []geminiFunctionDecl
		for _, t := range req.Tools {
			decls = append(decls, geminiFunctionDecl{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				Parameters:  t.Function.Parameters,
			})
		}

		gReq.Tools = []geminiToolDeclaration{{FunctionDeclarations: decls}}
	}

	// Convert tool_choice
	if req.ToolChoice != nil && len(gReq.Tools) > 0 {
		switch v := req.ToolChoice.(type) {
		case string:
			switch v {
			case "auto":
				gReq.ToolConfig = &geminiToolConfig{
					FunctionCallingConfig: &geminiFunctionCallingConfig{Mode: "AUTO"},
				}
			case "none":
				gReq.ToolConfig = &geminiToolConfig{
					FunctionCallingConfig: &geminiFunctionCallingConfig{Mode: "NONE"},
				}
			case "required":
				gReq.ToolConfig = &geminiToolConfig{
					FunctionCallingConfig: &geminiFunctionCallingConfig{Mode: "ANY"},
				}
			}
		case map[string]any:
			if fn, ok := v["function"].(map[string]any); ok {
				if name, ok := fn["name"].(string); ok {
					gReq.ToolConfig = &geminiToolConfig{
						FunctionCallingConfig: &geminiFunctionCallingConfig{
							Mode:                 "ANY",
							AllowedFunctionNames: []string{name},
						},
					}
				}
			}
		}
	}

	for _, m := range req.Messages {
		if m.Role == "system" {
			gReq.SystemInstruction = &geminiSystemInstruction{
				Parts: []geminiPart{{Text: m.Content}},
			}

			continue
		}

		// Handle tool result messages
		if m.Role == "tool" {
			var result map[string]any

			_ = json.Unmarshal([]byte(m.Content), &result)

			if result == nil {
				result = map[string]any{"result": m.Content}
			}

			// Find the function name from preceding assistant tool_calls
			fnName := findToolCallName(req.Messages, m.ToolCallID)

			gReq.Contents = append(gReq.Contents, geminiContent{
				Role: "user",
				Parts: []geminiPart{{
					FunctionResponse: &geminiFunctionResp{
						Name:     fnName,
						Response: result,
					},
				}},
			})

			continue
		}

		role := m.Role
		if role == "assistant" {
			role = "model"
		}

		// Handle assistant messages with tool_calls
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			var parts []geminiPart
			if m.Content != "" {
				parts = append(parts, geminiPart{Text: m.Content})
			}

			for _, tc := range m.ToolCalls {
				var args map[string]any

				_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)

				parts = append(parts, geminiPart{
					FunctionCall: &geminiFunctionCall{
						Name: tc.Function.Name,
						Args: args,
					},
				})
			}

			gReq.Contents = append(gReq.Contents, geminiContent{
				Role:  role,
				Parts: parts,
			})

			continue
		}

		gReq.Contents = append(gReq.Contents, geminiContent{
			Role:  role,
			Parts: []geminiPart{{Text: m.Content}},
		})
	}

	return gReq
}

// findToolCallName looks up the function name for a given tool_call_id in the message history.
func findToolCallName(messages []models.Message, toolCallID string) string {
	for _, m := range messages {
		for _, tc := range m.ToolCalls {
			if tc.ID == toolCallID {
				return tc.Function.Name
			}
		}
	}

	return ""
}

func translateFromGemini(resp geminiResponse, model string) *models.ChatCompletionResponse {
	var content string
	var toolCalls []models.ToolCall
	var finishReason string

	if len(resp.Candidates) > 0 {
		candidate := resp.Candidates[0]
		for _, part := range candidate.Content.Parts {
			if part.FunctionCall != nil {
				argsBytes, _ := json.Marshal(part.FunctionCall.Args)
				toolCalls = append(toolCalls, models.ToolCall{
					ID:   "call_" + uuid.NewString()[:8],
					Type: "function",
					Function: models.FunctionCall{
						Name:      part.FunctionCall.Name,
						Arguments: string(argsBytes),
					},
				})
			} else {
				content += part.Text
			}
		}

		finishReason = translateGeminiFinishReason(candidate.FinishReason)
		if len(toolCalls) > 0 {
			finishReason = "tool_calls"
		}
	}

	choice := models.Choice{
		Index:        0,
		Message:      models.Message{Role: "assistant", Content: content},
		FinishReason: finishReason,
		ToolCalls:    toolCalls,
	}

	result := &models.ChatCompletionResponse{
		ID:       "chatcmpl-" + uuid.NewString(),
		Object:   "chat.completion",
		Created:  time.Now().Unix(),
		Model:    model,
		Choices:  []models.Choice{choice},
		Provider: "gemini",
	}

	if resp.UsageMetadata != nil {
		result.Usage = models.Usage{
			PromptTokens:     resp.UsageMetadata.PromptTokenCount,
			CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      resp.UsageMetadata.TotalTokenCount,
		}
	}

	return result
}

func translateGeminiFinishReason(reason string) string {
	switch reason {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY":
		return "content_filter"
	default:
		return "stop"
	}
}
