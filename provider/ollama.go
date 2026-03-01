package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"gofr.dev/pkg/gofr"

	"aryanmehrotra/litellm-go/models"
)

const ollamaServiceName = "ollama"

// Ollama implements Provider and StreamingProvider for a local Ollama instance.
type Ollama struct {
	timeout         time.Duration
	chatModels      []string
	embeddingModels []string
}

// NewOllama creates an Ollama provider.
func NewOllama(timeout time.Duration) *Ollama {
	return &Ollama{timeout: timeout}
}

func (*Ollama) Name() string { return "ollama" }

func (o *Ollama) Models() []string {
	return o.chatModels
}

// ollamaTagsResponse is the response from GET /api/tags.
type ollamaTagsResponse struct {
	Models []struct {
		Name    string `json:"name"`
		Details struct {
			Family string `json:"family"`
		} `json:"details"`
	} `json:"models"`
}

// knownEmbeddingFamilies lists model families that are embedding models.
var knownEmbeddingFamilies = map[string]bool{
	"nomic-bert": true, "bert": true,
}

// knownEmbeddingPrefixes lists model name prefixes that indicate embedding models.
var knownEmbeddingPrefixes = []string{
	"nomic-embed", "all-minilm", "mxbai-embed", "snowflake-arctic-embed",
	"bge-", "gte-", "e5-", "paraphrase-",
}

// isEmbeddingModel returns true if the model name/family indicates an embedding model.
func isEmbeddingModel(name, family string) bool {
	if knownEmbeddingFamilies[family] {
		return true
	}
	lower := strings.ToLower(name)
	for _, prefix := range knownEmbeddingPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// RefreshModels queries the local Ollama instance for available models and
// classifies them as chat or embedding models.
func (o *Ollama) RefreshModels(ctx *gofr.Context) {
	svc := ctx.GetHTTPService(ollamaServiceName)
	if svc == nil {
		return
	}

	resp, err := svc.Get(ctx, "api/tags", nil)
	if err != nil {
		ctx.Errorf("ollama: failed to list models: %v", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var tags ollamaTagsResponse
	if err := json.Unmarshal(body, &tags); err != nil {
		return
	}

	var chat, embed []string
	for _, m := range tags.Models {
		// Strip :latest suffix for cleaner names
		name := strings.TrimSuffix(m.Name, ":latest")
		if isEmbeddingModel(name, m.Details.Family) {
			embed = append(embed, name)
		} else {
			chat = append(chat, name)
		}
	}

	o.chatModels = chat
	o.embeddingModels = embed
}

// ollamaRequest is the Ollama /api/chat request format.
type ollamaRequest struct {
	Model    string       `json:"model"`
	Messages []ollamaMsg  `json:"messages"`
	Stream   bool         `json:"stream"`
	Options  ollamaOpts   `json:"options,omitempty"`
	Tools    []ollamaTool `json:"tools,omitempty"`
}

type ollamaMsg struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}

type ollamaTool struct {
	Type     string             `json:"type"`
	Function ollamaToolFunction `json:"function"`
}

type ollamaToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type ollamaToolCall struct {
	Function ollamaToolCallFunction `json:"function"`
}

type ollamaToolCallFunction struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type ollamaOpts struct {
	Temperature *float64 `json:"temperature,omitempty"`
	TopP        *float64 `json:"top_p,omitempty"`
	NumPredict  *int     `json:"num_predict,omitempty"`
	Stop        []string `json:"stop,omitempty"`
}

// ollamaResponse is the Ollama /api/chat response format.
type ollamaResponse struct {
	Model           string    `json:"model"`
	Message         ollamaMsg `json:"message"`
	Done            bool      `json:"done"`
	DoneReason      string    `json:"done_reason"`
	PromptEvalCount int       `json:"prompt_eval_count"`
	EvalCount       int       `json:"eval_count"`
}

// ollamaToolCallsToOpenAI converts Ollama tool calls to OpenAI format.
func ollamaToolCallsToOpenAI(calls []ollamaToolCall) []models.ToolCall {
	if len(calls) == 0 {
		return nil
	}

	result := make([]models.ToolCall, 0, len(calls))

	for _, c := range calls {
		argsBytes, _ := json.Marshal(c.Function.Arguments)
		result = append(result, models.ToolCall{
			ID:   "call_" + uuid.NewString()[:8],
			Type: "function",
			Function: models.FunctionCall{
				Name:      c.Function.Name,
				Arguments: string(argsBytes),
			},
		})
	}

	return result
}

func (o *Ollama) ChatCompletion(ctx *gofr.Context, req models.ChatCompletionRequest) (*models.ChatCompletionResponse, error) {
	svc := ctx.GetHTTPService(ollamaServiceName)
	if svc == nil {
		return nil, fmt.Errorf("HTTP service %q not registered", ollamaServiceName)
	}

	oReq := translateToOllama(req)

	body, err := json.Marshal(oReq)
	if err != nil {
		return nil, fmt.Errorf("marshal ollama request: %w", err)
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	resp, err := svc.PostWithHeaders(ctx, "api/chat", nil, body, headers)
	if err != nil {
		return nil, fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read ollama response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var oResp ollamaResponse
	if err := json.Unmarshal(respBody, &oResp); err != nil {
		return nil, fmt.Errorf("decode ollama response: %w", err)
	}

	return translateFromOllama(oResp), nil
}

// ChatCompletionStream sends a streaming request to Ollama via GoFr's HTTP
// service and translates NDJSON responses into OpenAI-compatible StreamChunks.
func (o *Ollama) ChatCompletionStream(ctx *gofr.Context, req models.ChatCompletionRequest, onChunk func(models.StreamChunk)) error {
	svc := ctx.GetHTTPService(ollamaServiceName)
	if svc == nil {
		return fmt.Errorf("HTTP service %q not registered", ollamaServiceName)
	}

	oReq := translateToOllama(req)
	oReq.Stream = true

	body, err := json.Marshal(oReq)
	if err != nil {
		return fmt.Errorf("marshal ollama request: %w", err)
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	resp, err := svc.PostWithHeaders(ctx, "api/chat", nil, body, headers)
	if err != nil {
		return fmt.Errorf("ollama stream request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ollama stream returned status %d: %s", resp.StatusCode, string(respBody))
	}

	streamID := "chatcmpl-" + uuid.NewString()
	created := time.Now().Unix()

	// Send initial role chunk
	onChunk(models.StreamChunk{
		ID:      streamID,
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   req.Model,
		Choices: []models.StreamChoice{
			{Index: 0, Delta: models.StreamDelta{Role: "assistant"}},
		},
	})

	dec := json.NewDecoder(resp.Body)

	for dec.More() {
		var chunk ollamaResponse
		if err := dec.Decode(&chunk); err != nil {
			if err == io.EOF {
				break
			}

			return fmt.Errorf("decode ollama stream: %w", err)
		}

		if chunk.Done {
			finishReason := "stop"
			if chunk.DoneReason == "length" {
				finishReason = "length"
			}

			onChunk(models.StreamChunk{
				ID:      streamID,
				Object:  "chat.completion.chunk",
				Created: created,
				Model:   req.Model,
				Choices: []models.StreamChoice{
					{Index: 0, FinishReason: &finishReason},
				},
				Usage: &models.Usage{
					PromptTokens:     chunk.PromptEvalCount,
					CompletionTokens: chunk.EvalCount,
					TotalTokens:      chunk.PromptEvalCount + chunk.EvalCount,
				},
			})

			return nil
		}

		if len(chunk.Message.ToolCalls) > 0 {
			var stcs []models.StreamToolCall
			for i, tc := range chunk.Message.ToolCalls {
				argsBytes, _ := json.Marshal(tc.Function.Arguments)
				stcs = append(stcs, models.StreamToolCall{
					Index: i,
					ID:    "call_" + uuid.NewString()[:8],
					Type:  "function",
					Function: models.FunctionCall{
						Name:      tc.Function.Name,
						Arguments: string(argsBytes),
					},
				})
			}

			onChunk(models.StreamChunk{
				ID:      streamID,
				Object:  "chat.completion.chunk",
				Created: created,
				Model:   req.Model,
				Choices: []models.StreamChoice{
					{Index: 0, Delta: models.StreamDelta{ToolCalls: stcs}},
				},
			})
		} else if chunk.Message.Content != "" {
			onChunk(models.StreamChunk{
				ID:      streamID,
				Object:  "chat.completion.chunk",
				Created: created,
				Model:   req.Model,
				Choices: []models.StreamChoice{
					{Index: 0, Delta: models.StreamDelta{Content: chunk.Message.Content}},
				},
			})
		}
	}

	return nil
}

// ollamaEmbedRequest is the Ollama /api/embed request format.
type ollamaEmbedRequest struct {
	Model string `json:"model"`
	Input any    `json:"input"`
}

// ollamaEmbedResponse is the Ollama /api/embed response format.
type ollamaEmbedResponse struct {
	Model      string      `json:"model"`
	Embeddings [][]float64 `json:"embeddings"`
}

// EmbeddingModels returns the list of locally available embedding models.
func (o *Ollama) EmbeddingModels() []string {
	return o.embeddingModels
}

// Embedding sends an embedding request to Ollama.
func (o *Ollama) Embedding(ctx *gofr.Context, req models.EmbeddingRequest) (*models.EmbeddingResponse, error) {
	svc := ctx.GetHTTPService(ollamaServiceName)
	if svc == nil {
		return nil, fmt.Errorf("HTTP service %q not registered", ollamaServiceName)
	}

	oReq := ollamaEmbedRequest{
		Model: req.Model,
		Input: req.Input,
	}

	body, err := json.Marshal(oReq)
	if err != nil {
		return nil, fmt.Errorf("marshal ollama embed request: %w", err)
	}

	headers := map[string]string{"Content-Type": "application/json"}

	resp, err := svc.PostWithHeaders(ctx, "api/embed", nil, body, headers)
	if err != nil {
		return nil, fmt.Errorf("ollama embed request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read ollama embed response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ollama embed returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var oResp ollamaEmbedResponse
	if err := json.Unmarshal(respBody, &oResp); err != nil {
		return nil, fmt.Errorf("decode ollama embed response: %w", err)
	}

	// Translate to OpenAI format
	var data []models.EmbeddingData
	for i, emb := range oResp.Embeddings {
		data = append(data, models.EmbeddingData{
			Object:    "embedding",
			Embedding: emb,
			Index:     i,
		})
	}

	return &models.EmbeddingResponse{
		Object: "list",
		Data:   data,
		Model:  req.Model,
	}, nil
}

func translateToOllama(req models.ChatCompletionRequest) ollamaRequest {
	oReq := ollamaRequest{
		Model:  req.Model,
		Stream: false,
		Options: ollamaOpts{
			Temperature: req.Temperature,
			TopP:        req.TopP,
			NumPredict:  req.MaxTokens,
			Stop:        req.Stop,
		},
	}

	// Convert tools
	for _, t := range req.Tools {
		oReq.Tools = append(oReq.Tools, ollamaTool{
			Type: t.Type,
			Function: ollamaToolFunction{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				Parameters:  t.Function.Parameters,
			},
		})
	}

	for _, m := range req.Messages {
		msg := ollamaMsg{
			Role:    m.Role,
			Content: m.Content,
		}

		// Convert assistant tool_calls to Ollama format
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			for _, tc := range m.ToolCalls {
				var args map[string]any

				_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)

				msg.ToolCalls = append(msg.ToolCalls, ollamaToolCall{
					Function: ollamaToolCallFunction{
						Name:      tc.Function.Name,
						Arguments: args,
					},
				})
			}
		}

		oReq.Messages = append(oReq.Messages, msg)
	}

	return oReq
}

func translateFromOllama(resp ollamaResponse) *models.ChatCompletionResponse {
	finishReason := "stop"
	if resp.DoneReason == "length" {
		finishReason = "length"
	}

	toolCalls := ollamaToolCallsToOpenAI(resp.Message.ToolCalls)
	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}

	return &models.ChatCompletionResponse{
		ID:      "chatcmpl-" + uuid.NewString(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   resp.Model,
		Choices: []models.Choice{
			{
				Index:        0,
				Message:      models.Message{Role: resp.Message.Role, Content: resp.Message.Content},
				FinishReason: finishReason,
				ToolCalls:    toolCalls,
			},
		},
		Usage: models.Usage{
			PromptTokens:     resp.PromptEvalCount,
			CompletionTokens: resp.EvalCount,
			TotalTokens:      resp.PromptEvalCount + resp.EvalCount,
		},
		Provider: "ollama",
	}
}
