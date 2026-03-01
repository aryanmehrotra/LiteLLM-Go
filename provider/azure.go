package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"gofr.dev/pkg/gofr"

	"aryanmehrotra/llm-gateway/models"
)

const azureServiceName = "azure"

// Azure implements Provider and StreamingProvider for Azure OpenAI Service.
// Azure uses deployment-specific paths and an api-key header instead of Bearer auth.
type Azure struct {
	apiKey     string
	apiVersion string
	modelList  []string
	timeout    time.Duration
}

// NewAzure creates an Azure OpenAI provider.
// deployments is a comma-separated list of deployment names (used as model IDs).
// If empty, defaults to "gpt-4o,gpt-4o-mini".
func NewAzure(apiKey, apiVersion, deployments string, timeout time.Duration) *Azure {
	var modelList []string

	for _, d := range strings.Split(deployments, ",") {
		d = strings.TrimSpace(d)

		if d != "" {
			modelList = append(modelList, d)
		}
	}

	if len(modelList) == 0 {
		modelList = []string{"gpt-4o", "gpt-4o-mini"}
	}

	if apiVersion == "" {
		apiVersion = "2024-02-01"
	}

	return &Azure{
		apiKey:     apiKey,
		apiVersion: apiVersion,
		modelList:  modelList,
		timeout:    timeout,
	}
}

func (*Azure) Name() string       { return "azure" }
func (a *Azure) Models() []string { return a.modelList }

// ChatCompletion sends a request to Azure OpenAI.
// The model field is treated as the deployment name.
func (a *Azure) ChatCompletion(ctx *gofr.Context, req models.ChatCompletionRequest) (*models.ChatCompletionResponse, error) {
	svc := ctx.GetHTTPService(azureServiceName)
	if svc == nil {
		return nil, fmt.Errorf("HTTP service %q not registered", azureServiceName)
	}

	req.Stream = false

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal azure request: %w", err)
	}

	path := fmt.Sprintf("openai/deployments/%s/chat/completions?api-version=%s", req.Model, a.apiVersion)
	headers := map[string]string{
		"Content-Type": "application/json",
		"api-key":      a.apiKey,
	}

	resp, err := svc.PostWithHeaders(ctx, path, nil, body, headers)
	if err != nil {
		return nil, fmt.Errorf("azure request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read azure response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("azure returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result models.ChatCompletionResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode azure response: %w", err)
	}

	result.Provider = "azure"

	return &result, nil
}

// ChatCompletionStream sends a streaming request to Azure OpenAI.
func (a *Azure) ChatCompletionStream(ctx *gofr.Context, req models.ChatCompletionRequest, onChunk func(models.StreamChunk)) error {
	svc := ctx.GetHTTPService(azureServiceName)
	if svc == nil {
		return fmt.Errorf("HTTP service %q not registered", azureServiceName)
	}

	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal azure request: %w", err)
	}

	path := fmt.Sprintf("openai/deployments/%s/chat/completions?api-version=%s", req.Model, a.apiVersion)
	headers := map[string]string{
		"Content-Type": "application/json",
		"api-key":      a.apiKey,
		"Accept":       "text/event-stream",
	}

	resp, err := svc.PostWithHeaders(ctx, path, nil, body, headers)
	if err != nil {
		return fmt.Errorf("azure stream request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("azure stream returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return parseOpenAISSE(resp.Body, req.Model, onChunk)
}
