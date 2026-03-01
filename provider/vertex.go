package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"gofr.dev/pkg/gofr"

	"examples/llm-gateway/models"
)

const vertexServiceName = "vertex"

// Vertex implements Provider and StreamingProvider for Google Cloud Vertex AI.
// It reuses the same generateContent API format as the Gemini provider, but
// targets Vertex AI endpoints and authenticates with a Bearer token instead of
// an API key.
//
// Authentication: provide a Google access token via VERTEX_ACCESS_TOKEN.
// Refresh it periodically with: gcloud auth print-access-token
//
// Endpoint pattern:
//
//	https://{location}-aiplatform.googleapis.com/v1/projects/{project}/locations/{location}/publishers/google/models/{model}:generateContent
type Vertex struct {
	projectID   string
	location    string
	accessToken string
	timeout     time.Duration
}

// NewVertex creates a Vertex AI provider.
// projectID is the GCP project ID.
// location is the Vertex AI region (e.g. "us-central1").
// accessToken is a Google OAuth2 access token (Bearer).
func NewVertex(projectID, location, accessToken string, timeout time.Duration) *Vertex {
	if location == "" {
		location = "us-central1"
	}

	return &Vertex{
		projectID:   projectID,
		location:    location,
		accessToken: accessToken,
		timeout:     timeout,
	}
}

func (*Vertex) Name() string { return "vertex" }

func (*Vertex) Models() []string {
	return []string{
		"gemini-2.0-flash-001",
		"gemini-1.5-pro-002",
		"gemini-1.5-flash-002",
		"gemini-1.0-pro-002",
	}
}

// vertexPath builds the Vertex AI generateContent path for a model.
func (v *Vertex) vertexPath(model string) string {
	return fmt.Sprintf(
		"v1/projects/%s/locations/%s/publishers/google/models/%s:generateContent",
		v.projectID, v.location, model,
	)
}

// vertexStreamPath builds the Vertex AI streamGenerateContent path for a model.
func (v *Vertex) vertexStreamPath(model string) string {
	return fmt.Sprintf(
		"v1/projects/%s/locations/%s/publishers/google/models/%s:streamGenerateContent?alt=sse",
		v.projectID, v.location, model,
	)
}

func (v *Vertex) ChatCompletion(ctx *gofr.Context, req models.ChatCompletionRequest) (*models.ChatCompletionResponse, error) {
	svc := ctx.GetHTTPService(vertexServiceName)
	if svc == nil {
		return nil, fmt.Errorf("HTTP service %q not registered", vertexServiceName)
	}

	gReq := translateToGemini(req)

	body, err := json.Marshal(gReq)
	if err != nil {
		return nil, fmt.Errorf("marshal vertex request: %w", err)
	}

	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + v.accessToken,
	}

	resp, err := svc.PostWithHeaders(ctx, v.vertexPath(req.Model), nil, body, headers)
	if err != nil {
		return nil, fmt.Errorf("vertex request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read vertex response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("vertex returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var gResp geminiResponse
	if err := json.Unmarshal(respBody, &gResp); err != nil {
		return nil, fmt.Errorf("decode vertex response: %w", err)
	}

	result := translateFromGemini(gResp, req.Model)
	result.Provider = "vertex"

	return result, nil
}

// ChatCompletionStream sends a streaming request to Vertex AI and translates
// SSE events into OpenAI-compatible StreamChunks.
func (v *Vertex) ChatCompletionStream(ctx *gofr.Context, req models.ChatCompletionRequest, onChunk func(models.StreamChunk)) error {
	svc := ctx.GetHTTPService(vertexServiceName)
	if svc == nil {
		return fmt.Errorf("HTTP service %q not registered", vertexServiceName)
	}

	gReq := translateToGemini(req)

	body, err := json.Marshal(gReq)
	if err != nil {
		return fmt.Errorf("marshal vertex request: %w", err)
	}

	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + v.accessToken,
		"Accept":        "text/event-stream",
	}

	resp, err := svc.PostWithHeaders(ctx, v.vertexStreamPath(req.Model), nil, body, headers)
	if err != nil {
		return fmt.Errorf("vertex stream request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("vertex stream returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return parseGeminiSSE(resp.Body, req.Model, onChunk)
}
