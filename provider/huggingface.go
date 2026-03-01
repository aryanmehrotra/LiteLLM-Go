package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"gofr.dev/pkg/gofr"

	"aryanmehrotra/llm-gateway/models"
)

const huggingfaceServiceName = "huggingface"

// HuggingFace implements Provider and StreamingProvider for Hugging Face's
// Serverless Inference API (TGI / Messages API).
//
// Each model is served at a unique path:
//
//	POST https://api-inference.huggingface.co/models/{model}/v1/chat/completions
//
// The model field of the request is used verbatim as the HF model ID
// (e.g. "mistralai/Mistral-7B-Instruct-v0.3").
type HuggingFace struct {
	apiKey    string
	modelList []string
	timeout   time.Duration
}

// NewHuggingFace creates a HuggingFace provider with the given API token and timeout.
// Popular open-source models that support the Messages API are listed by default.
func NewHuggingFace(apiKey string, timeout time.Duration) *HuggingFace {
	return &HuggingFace{
		apiKey: apiKey,
		modelList: []string{
			"mistralai/Mistral-7B-Instruct-v0.3",
			"mistralai/Mixtral-8x7B-Instruct-v0.1",
			"meta-llama/Llama-3.3-70B-Instruct",
			"meta-llama/Llama-3.1-8B-Instruct",
			"Qwen/Qwen2.5-72B-Instruct",
			"microsoft/Phi-3.5-mini-instruct",
		},
		timeout: timeout,
	}
}

func (*HuggingFace) Name() string       { return "huggingface" }
func (h *HuggingFace) Models() []string { return h.modelList }

// ChatCompletion sends an OpenAI-compatible request to the HuggingFace
// Serverless Inference API for the given model.
func (h *HuggingFace) ChatCompletion(ctx *gofr.Context, req models.ChatCompletionRequest) (*models.ChatCompletionResponse, error) {
	svc := ctx.GetHTTPService(huggingfaceServiceName)
	if svc == nil {
		return nil, fmt.Errorf("HTTP service %q not registered", huggingfaceServiceName)
	}

	req.Stream = false

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal huggingface request: %w", err)
	}

	// Path embeds the model ID: models/{model}/v1/chat/completions
	path := fmt.Sprintf("models/%s/v1/chat/completions", req.Model)
	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + h.apiKey,
	}

	resp, err := svc.PostWithHeaders(ctx, path, nil, body, headers)
	if err != nil {
		return nil, fmt.Errorf("huggingface request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read huggingface response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("huggingface returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result models.ChatCompletionResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode huggingface response: %w", err)
	}

	result.Provider = "huggingface"

	return &result, nil
}

// ChatCompletionStream sends a streaming request to HuggingFace and
// parses the OpenAI-compatible SSE events.
func (h *HuggingFace) ChatCompletionStream(ctx *gofr.Context, req models.ChatCompletionRequest, onChunk func(models.StreamChunk)) error {
	svc := ctx.GetHTTPService(huggingfaceServiceName)
	if svc == nil {
		return fmt.Errorf("HTTP service %q not registered", huggingfaceServiceName)
	}

	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal huggingface request: %w", err)
	}

	path := fmt.Sprintf("models/%s/v1/chat/completions", req.Model)
	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + h.apiKey,
		"Accept":        "text/event-stream",
	}

	resp, err := svc.PostWithHeaders(ctx, path, nil, body, headers)
	if err != nil {
		return fmt.Errorf("huggingface stream request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("huggingface stream returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return parseOpenAISSE(resp.Body, req.Model, onChunk)
}
