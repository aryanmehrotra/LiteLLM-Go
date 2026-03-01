package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/service"

	"aryanmehrotra/litellm-go/models"
	"aryanmehrotra/litellm-go/testutil"
)

// noopLogger implements service.Logger with a no-op implementation.
type noopLogger struct{}

func (noopLogger) Log(...any) {}

// noopMetrics implements service.Metrics with no-op implementations.
type noopMetrics struct{}

func (noopMetrics) NewCounter(_, _ string)                                     {}
func (noopMetrics) NewGauge(_, _ string)                                       {}
func (noopMetrics) IncrementCounter(_ context.Context, _ string, _ ...string)  {}
func (noopMetrics) RecordHistogram(_ context.Context, _ string, _ float64, _ ...string) {}
func (noopMetrics) SetGauge(_ string, _ float64, _ ...string)                  {}

// newGofrCtxWithHTTPService creates a *gofr.Context with the given HTTP service
// registered under the provided service name. This allows OpenAICompatible
// providers to make real HTTP calls to the mock LLM server.
func newGofrCtxWithHTTPService(svcName, baseURL string) *gofr.Context {
	svc := service.NewHTTPService(baseURL, noopLogger{}, noopMetrics{})

	c := &container.Container{
		Services: map[string]service.HTTP{
			svcName: svc,
		},
	}
	c.Logger = logging.NewLogger(logging.INFO)

	logger := logging.NewLogger(logging.INFO)
	cl := logging.NewContextLogger(context.Background(), logger)

	ctx := &gofr.Context{
		Context:   context.Background(),
		Container: c,
	}
	ctx.ContextLogger = *cl

	return ctx
}

// --------------------------------------------------------------------------
// OpenAI-compatible provider against mock LLM server
// --------------------------------------------------------------------------

// TestOpenAICompatible_ChatCompletion_MockServer tests that OpenAI provider
// sends correct HTTP requests to the LLM endpoint and parses responses.
func TestOpenAICompatible_ChatCompletion_MockServer(t *testing.T) {
	llm := testutil.NewMockLLMServer(t)
	llm.QueueText("The answer is 42.")

	// Create an OpenAI provider pointed at the mock server.
	// The service name must match what we register in the container.
	const svcName = "openai-test"
	p := NewOpenAICompatible("openai-test", svcName, "sk-test-key",
		[]string{"gpt-4o"}, 5*time.Second)

	ctx := newGofrCtxWithHTTPService(svcName, llm.URL())

	req := models.ChatCompletionRequest{
		Model: "gpt-4o",
		Messages: []models.Message{
			{Role: "user", Content: "What is the answer to everything?"},
		},
	}

	resp, err := p.ChatCompletion(ctx, req)
	if err != nil {
		t.Fatalf("ChatCompletion: unexpected error: %v", err)
	}

	if len(resp.Choices) == 0 {
		t.Fatal("expected at least 1 choice")
	}

	if resp.Choices[0].Message.Content != "The answer is 42." {
		t.Errorf("content: want 'The answer is 42.', got %q", resp.Choices[0].Message.Content)
	}

	// Verify the mock server received exactly 1 request
	if llm.RequestCount() != 1 {
		t.Errorf("RequestCount: want 1, got %d", llm.RequestCount())
	}

	// Verify the request body sent to the mock server
	sentReq := llm.LastRequest()
	if sentReq == nil {
		t.Fatal("LastRequest() returned nil")
	}

	if sentReq.Model != "gpt-4o" {
		t.Errorf("sent model: want 'gpt-4o', got %q", sentReq.Model)
	}

	if len(sentReq.Messages) != 1 {
		t.Errorf("sent messages count: want 1, got %d", len(sentReq.Messages))
	}
}

// TestOpenAICompatible_ChatCompletion_ToolCallResponse tests that the provider
// correctly parses a tool_calls response.
func TestOpenAICompatible_ChatCompletion_ToolCallResponse(t *testing.T) {
	llm := testutil.NewMockLLMServer(t)
	llm.QueueToolCall("call-abc", "get_weather", `{"location": "Tokyo"}`)

	const svcName = "openai-tool-test"
	p := NewOpenAICompatible("openai-tool-test", svcName, "sk-key",
		[]string{"gpt-4o"}, 5*time.Second)

	ctx := newGofrCtxWithHTTPService(svcName, llm.URL())

	req := models.ChatCompletionRequest{
		Model: "gpt-4o",
		Messages: []models.Message{
			{Role: "user", Content: "What's the weather in Tokyo?"},
		},
		Tools: []models.Tool{{
			Type: "function",
			Function: models.ToolFunction{
				Name:        "get_weather",
				Description: "Get weather for a location",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"location": map[string]any{"type": "string"},
					},
				},
			},
		}},
	}

	resp, err := p.ChatCompletion(ctx, req)
	if err != nil {
		t.Fatalf("ChatCompletion: unexpected error: %v", err)
	}

	if len(resp.Choices) == 0 {
		t.Fatal("expected at least 1 choice")
	}

	choice := resp.Choices[0]
	if choice.FinishReason != "tool_calls" {
		t.Errorf("FinishReason: want 'tool_calls', got %q", choice.FinishReason)
	}

	if len(choice.Message.ToolCalls) != 1 {
		t.Fatalf("ToolCalls: want 1, got %d", len(choice.Message.ToolCalls))
	}

	tc := choice.Message.ToolCalls[0]
	if tc.Function.Name != "get_weather" {
		t.Errorf("tool name: want 'get_weather', got %q", tc.Function.Name)
	}

	if !strings.Contains(tc.Function.Arguments, "Tokyo") {
		t.Errorf("tool args should contain 'Tokyo', got %q", tc.Function.Arguments)
	}
}

// TestOpenAICompatible_ChatCompletion_MultiTurn tests a multi-turn conversation
// with the mock server, simulating an agent-like flow.
func TestOpenAICompatible_ChatCompletion_MultiTurn(t *testing.T) {
	llm := testutil.NewMockLLMServer(t)
	llm.QueueToolCall("call-1", "search", `{"query": "golang"}`)
	llm.QueueText("Go is a statically typed language developed by Google.")

	const svcName = "openai-multiturn"
	p := NewOpenAICompatible("openai-multiturn", svcName, "sk-key",
		[]string{"gpt-4o"}, 5*time.Second)

	ctx := newGofrCtxWithHTTPService(svcName, llm.URL())

	// First call
	req1 := models.ChatCompletionRequest{
		Model: "gpt-4o",
		Messages: []models.Message{
			{Role: "user", Content: "Tell me about golang"},
		},
	}

	resp1, err := p.ChatCompletion(ctx, req1)
	if err != nil {
		t.Fatalf("first ChatCompletion: %v", err)
	}

	if resp1.Choices[0].FinishReason != "tool_calls" {
		t.Errorf("first response: want 'tool_calls', got %q", resp1.Choices[0].FinishReason)
	}

	// Second call (with tool result added)
	req2 := models.ChatCompletionRequest{
		Model: "gpt-4o",
		Messages: []models.Message{
			{Role: "user", Content: "Tell me about golang"},
			{
				Role:      "assistant",
				ToolCalls: resp1.Choices[0].Message.ToolCalls,
			},
			{
				Role:       "tool",
				Content:    `{"results": ["Go homepage", "Go docs"]}`,
				ToolCallID: resp1.Choices[0].Message.ToolCalls[0].ID,
			},
		},
	}

	resp2, err := p.ChatCompletion(ctx, req2)
	if err != nil {
		t.Fatalf("second ChatCompletion: %v", err)
	}

	if resp2.Choices[0].FinishReason != "stop" {
		t.Errorf("second response: want 'stop', got %q", resp2.Choices[0].FinishReason)
	}

	if resp2.Choices[0].Message.Content != "Go is a statically typed language developed by Google." {
		t.Errorf("second response content: got %q", resp2.Choices[0].Message.Content)
	}

	// Verify the second request sent to LLM includes the tool result
	req2Body := llm.RequestBodyAt(1)
	var parsedReq2 models.ChatCompletionRequest
	if err := json.Unmarshal(req2Body, &parsedReq2); err != nil {
		t.Fatalf("parse second request body: %v", err)
	}

	hasToolMsg := false
	for _, msg := range parsedReq2.Messages {
		if msg.Role == "tool" {
			hasToolMsg = true
		}
	}

	if !hasToolMsg {
		t.Error("second request should include a tool result message")
	}
}

// TestOpenAICompatible_ChatCompletion_Error tests that HTTP error responses
// from the provider are returned as errors.
func TestOpenAICompatible_ChatCompletion_Error(t *testing.T) {
	llm := testutil.NewMockLLMServer(t)
	llm.QueueError(429, "rate limit exceeded")

	const svcName = "openai-error-test"
	p := NewOpenAICompatible("openai-error-test", svcName, "sk-key",
		[]string{"gpt-4o"}, 5*time.Second)

	ctx := newGofrCtxWithHTTPService(svcName, llm.URL())

	req := models.ChatCompletionRequest{
		Model:    "gpt-4o",
		Messages: []models.Message{{Role: "user", Content: "hi"}},
	}

	_, err := p.ChatCompletion(ctx, req)
	if err == nil {
		t.Fatal("expected error for 429 response, got nil")
	}
}

// TestOpenAICompatible_ChatCompletion_SendsAuthHeader verifies that the
// Authorization header with the API key is sent to the mock server.
func TestOpenAICompatible_ChatCompletion_SendsAuthHeader(t *testing.T) {
	var capturedHeader string

	// Use a simple httptest server to capture the Authorization header
	headerCaptureSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testutil.TextResponse("auth-test"))
	}))
	defer headerCaptureSrv.Close()

	const svcName = "openai-auth-test"
	const apiKey = "sk-super-secret-test-key"
	p := NewOpenAICompatible("openai-auth-test", svcName, apiKey,
		[]string{"gpt-4o"}, 5*time.Second)

	ctx := newGofrCtxWithHTTPService(svcName, headerCaptureSrv.URL)

	req := models.ChatCompletionRequest{
		Model:    "gpt-4o",
		Messages: []models.Message{{Role: "user", Content: "hi"}},
	}

	_, err := p.ChatCompletion(ctx, req)
	if err != nil {
		t.Fatalf("ChatCompletion: %v", err)
	}

	if capturedHeader != "Bearer "+apiKey {
		t.Errorf("Authorization header: want 'Bearer %s', got %q", apiKey, capturedHeader)
	}
}

// TestOpenAICompatible_ChatCompletion_StreamFalse verifies that stream is
// set to false when calling the non-streaming endpoint.
func TestOpenAICompatible_ChatCompletion_StreamFalse(t *testing.T) {
	srv := testutil.NewMockLLMServer(t)
	srv.QueueText("response")

	const svcName = "openai-stream-test"
	p := NewOpenAICompatible("openai-stream-test", svcName, "sk-key",
		[]string{"gpt-4o"}, 5*time.Second)

	ctx := newGofrCtxWithHTTPService(svcName, srv.URL())

	req := models.ChatCompletionRequest{
		Model:    "gpt-4o",
		Stream:   true, // should be overridden to false
		Messages: []models.Message{{Role: "user", Content: "hi"}},
	}

	_, err := p.ChatCompletion(ctx, req)
	if err != nil {
		t.Fatalf("ChatCompletion: %v", err)
	}

	// The request sent to LLM should have stream=false
	sentReq := srv.LastRequest()
	if sentReq.Stream {
		t.Error("provider should set stream=false for non-streaming ChatCompletion call")
	}
}
