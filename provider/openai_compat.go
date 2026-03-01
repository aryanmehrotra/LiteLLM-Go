package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"gofr.dev/pkg/gofr"

	"aryanmehrotra/litellm-go/models"
)

// OpenAICompatible is a shared base for providers that use the OpenAI API format.
// OpenAI, Groq, DeepSeek, and other compatible providers embed this struct.
type OpenAICompatible struct {
	providerName    string
	serviceName     string
	apiKey          string
	modelList       []string
	embeddingModels []string
	timeout         time.Duration
	// chatPath overrides the default "v1/chat/completions" path (e.g. Perplexity uses "chat/completions").
	chatPath string
}

// NewOpenAICompatible creates a new OpenAI-compatible provider base.
func NewOpenAICompatible(name, serviceName, apiKey string, modelsList []string, timeout time.Duration) *OpenAICompatible {
	return &OpenAICompatible{
		providerName: name,
		serviceName:  serviceName,
		apiKey:       apiKey,
		modelList:    modelsList,
		timeout:      timeout,
	}
}

func (o *OpenAICompatible) Name() string     { return o.providerName }
func (o *OpenAICompatible) Models() []string { return o.modelList }

// SetChatPath overrides the default "v1/chat/completions" path.
func (o *OpenAICompatible) SetChatPath(path string) { o.chatPath = path }

// chatCompletionsPath returns the path used for chat completions requests.
func (o *OpenAICompatible) chatCompletionsPath() string {
	if o.chatPath != "" {
		return o.chatPath
	}

	return "v1/chat/completions"
}

// SetEmbeddingModels sets the list of supported embedding models.
func (o *OpenAICompatible) SetEmbeddingModels(models []string) {
	o.embeddingModels = models
}

// EmbeddingModels returns the list of supported embedding models.
func (o *OpenAICompatible) EmbeddingModels() []string {
	return o.embeddingModels
}

// deriveContext returns a context with timeout applied if configured.
func (o *OpenAICompatible) deriveContext(ctx *gofr.Context) (context.Context, context.CancelFunc) {
	if o.timeout > 0 {
		return context.WithTimeout(ctx, o.timeout)
	}

	return ctx, func() {}
}

// ChatCompletion uses GoFr's HTTP service for non-streaming requests,
// getting circuit breakers, metrics, and tracing automatically.
func (o *OpenAICompatible) ChatCompletion(ctx *gofr.Context, req models.ChatCompletionRequest) (*models.ChatCompletionResponse, error) {
	svc := ctx.GetHTTPService(o.serviceName)
	if svc == nil {
		return nil, fmt.Errorf("HTTP service %q not registered", o.serviceName)
	}

	_, cancel := o.deriveContext(ctx)
	defer cancel()

	req.Stream = false

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + o.apiKey,
	}

	resp, err := svc.PostWithHeaders(ctx, o.chatCompletionsPath(), nil, body, headers)
	if err != nil {
		return nil, fmt.Errorf("%s request: %w", o.providerName, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s response: %w", o.providerName, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s returned status %d: %s", o.providerName, resp.StatusCode, string(respBody))
	}

	var result models.ChatCompletionResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode %s response: %w", o.providerName, err)
	}

	result.Provider = o.providerName

	return &result, nil
}

// ChatCompletionStream uses GoFr's HTTP service with stream:true and parses
// SSE events from the upstream OpenAI-compatible API.
func (o *OpenAICompatible) ChatCompletionStream(ctx *gofr.Context, req models.ChatCompletionRequest, onChunk func(models.StreamChunk)) error {
	svc := ctx.GetHTTPService(o.serviceName)
	if svc == nil {
		return fmt.Errorf("HTTP service %q not registered", o.serviceName)
	}

	_, cancel := o.deriveContext(ctx)
	defer cancel()

	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + o.apiKey,
		"Accept":        "text/event-stream",
	}

	resp, err := svc.PostWithHeaders(ctx, o.chatCompletionsPath(), nil, body, headers)
	if err != nil {
		return fmt.Errorf("%s stream request: %w", o.providerName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s stream returned status %d: %s", o.providerName, resp.StatusCode, string(respBody))
	}

	return parseOpenAISSE(resp.Body, req.Model, onChunk)
}

// Embedding sends an embedding request to the OpenAI-compatible API.
func (o *OpenAICompatible) Embedding(ctx *gofr.Context, req models.EmbeddingRequest) (*models.EmbeddingResponse, error) {
	svc := ctx.GetHTTPService(o.serviceName)
	if svc == nil {
		return nil, fmt.Errorf("HTTP service %q not registered", o.serviceName)
	}

	_, cancel := o.deriveContext(ctx)
	defer cancel()

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}

	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + o.apiKey,
	}

	resp, err := svc.PostWithHeaders(ctx, "v1/embeddings", nil, body, headers)
	if err != nil {
		return nil, fmt.Errorf("%s embedding request: %w", o.providerName, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s embedding response: %w", o.providerName, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s embedding returned status %d: %s", o.providerName, resp.StatusCode, string(respBody))
	}

	var result models.EmbeddingResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode %s embedding response: %w", o.providerName, err)
	}

	return &result, nil
}

// Moderation sends a moderation request to the OpenAI-compatible API.
func (o *OpenAICompatible) Moderation(ctx *gofr.Context, req models.ModerationRequest) (*models.ModerationResponse, error) {
	svc := ctx.GetHTTPService(o.serviceName)
	if svc == nil {
		return nil, fmt.Errorf("HTTP service %q not registered", o.serviceName)
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal moderation request: %w", err)
	}

	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + o.apiKey,
	}

	resp, err := svc.PostWithHeaders(ctx, "v1/moderations", nil, body, headers)
	if err != nil {
		return nil, fmt.Errorf("%s moderation request: %w", o.providerName, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s moderation response: %w", o.providerName, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s moderation returned status %d: %s", o.providerName, resp.StatusCode, string(respBody))
	}

	var result models.ModerationResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode %s moderation response: %w", o.providerName, err)
	}

	return &result, nil
}

// ImageGeneration sends an image generation request to the OpenAI-compatible API.
func (o *OpenAICompatible) ImageGeneration(ctx *gofr.Context, req models.ImageGenerationRequest) (*models.ImageResponse, error) {
	svc := ctx.GetHTTPService(o.serviceName)
	if svc == nil {
		return nil, fmt.Errorf("HTTP service %q not registered", o.serviceName)
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal image generation request: %w", err)
	}

	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + o.apiKey,
	}

	resp, err := svc.PostWithHeaders(ctx, "v1/images/generations", nil, body, headers)
	if err != nil {
		return nil, fmt.Errorf("%s image generation request: %w", o.providerName, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s image generation response: %w", o.providerName, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s image generation returned status %d: %s", o.providerName, resp.StatusCode, string(respBody))
	}

	var result models.ImageResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode %s image generation response: %w", o.providerName, err)
	}

	return &result, nil
}

// parseOpenAISSE reads SSE events from an OpenAI-compatible stream and invokes
// onChunk for each parsed chunk. Returns when [DONE] is received or the stream ends.
func parseOpenAISSE(r io.Reader, model string, onChunk func(models.StreamChunk)) error {
	scanner := bufio.NewScanner(r)
	streamID := "chatcmpl-" + uuid.NewString()
	created := time.Now().Unix()

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		if data == "[DONE]" {
			return nil
		}

		var chunk models.StreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if chunk.ID == "" {
			chunk.ID = streamID
		}

		if chunk.Object == "" {
			chunk.Object = "chat.completion.chunk"
		}

		if chunk.Created == 0 {
			chunk.Created = created
		}

		if chunk.Model == "" {
			chunk.Model = model
		}

		onChunk(chunk)
	}

	return scanner.Err()
}
