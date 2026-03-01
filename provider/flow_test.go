package provider_test

import (
	"errors"
	"testing"
	"time"

	"aryanmehrotra/litellm-go/models"
	"aryanmehrotra/litellm-go/provider"
	"aryanmehrotra/litellm-go/routing"
	"aryanmehrotra/litellm-go/testutil"
)

// TestFullChatFlow_RegistryAndRouter exercises the full flow:
// Registry.ResolveProvider → Router.ChatCompletion → MockProvider
// This mirrors what the ChatCompletion handler does.
func TestFullChatFlow_RegistryAndRouter(t *testing.T) {
	mock := testutil.NewMockProvider("mock",
		testutil.TextResponse("This is a test response from the mock LLM."),
	)

	reg := provider.NewRegistry("mock")
	reg.Register(mock)

	router := routing.NewRouter(
		routing.DefaultRetryPolicy(0, 0),
		routing.NewCooldownTracker(999, time.Millisecond),
		&routing.SimpleStrategy{},
	)

	ctx := testutil.NewGofrCtx()

	// Resolve the provider (as the handler does)
	p, modelName, err := reg.ResolveProvider("mock/mock-model")
	if err != nil {
		t.Fatalf("ResolveProvider: %v", err)
	}

	if p.Name() != "mock" {
		t.Errorf("provider name: want 'mock', got %q", p.Name())
	}

	if modelName != "mock-model" {
		t.Errorf("model name: want 'mock-model', got %q", modelName)
	}

	// Execute through router (as the handler does)
	req := models.ChatCompletionRequest{
		Model: modelName,
		Messages: []models.Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello, how are you?"},
		},
	}

	resp, err := router.ChatCompletion(ctx, p, modelName, req)
	if err != nil {
		t.Fatalf("ChatCompletion: %v", err)
	}

	if len(resp.Choices) == 0 {
		t.Fatal("expected at least 1 choice")
	}

	content := resp.Choices[0].Message.Content
	if content != "This is a test response from the mock LLM." {
		t.Errorf("content: want test response, got %q", content)
	}

	// Verify the mock recorded the request
	call0 := mock.CallAt(0)
	if call0.Model != "mock-model" {
		t.Errorf("recorded model: want 'mock-model', got %q", call0.Model)
	}

	if len(call0.Messages) != 2 {
		t.Errorf("recorded messages: want 2, got %d", len(call0.Messages))
	}
}

// TestFullChatFlow_FallbackProvider tests the fallback chain behavior
// when the primary provider fails.
func TestFullChatFlow_FallbackProvider(t *testing.T) {
	responses := []*models.ChatCompletionResponse{nil}
	errs := []error{&routing.ProviderError{StatusCode: 503, Body: "unavailable", Err: errors.New("service unavailable")}}

	primary := testutil.NewMockProviderWithErrors("primary", responses, errs)
	secondary := testutil.NewMockProvider("secondary",
		testutil.TextResponse("fallback response"),
	)

	reg := provider.NewRegistry("primary")
	reg.Register(primary)
	reg.Register(secondary)

	// Build fallback chain: primary → secondary
	cooldown := routing.NewCooldownTracker(1, time.Millisecond)
	fb := reg.BuildFallbackChain([]string{"primary", "secondary"}, cooldown)

	if fb == nil {
		t.Fatal("BuildFallbackChain returned nil for 2 providers")
	}

	ctx := testutil.NewGofrCtx()

	req := models.ChatCompletionRequest{
		Model: "test-model",
		Messages: []models.Message{
			{Role: "user", Content: "Will the primary fail?"},
		},
	}

	resp, err := fb.ChatCompletion(ctx, req)
	if err != nil {
		t.Fatalf("fallback ChatCompletion: %v", err)
	}

	if len(resp.Choices) == 0 {
		t.Fatal("expected at least 1 choice from fallback")
	}

	if resp.Choices[0].Message.Content != "fallback response" {
		t.Errorf("content: want 'fallback response', got %q", resp.Choices[0].Message.Content)
	}

	// Primary was tried once (and failed)
	if primary.CallCount() != 1 {
		t.Errorf("primary calls: want 1, got %d", primary.CallCount())
	}

	// Secondary was tried once (and succeeded)
	if secondary.CallCount() != 1 {
		t.Errorf("secondary calls: want 1, got %d", secondary.CallCount())
	}
}

// TestFullChatFlow_AgentLoop_ViaRegistryAndRouter is an end-to-end test of
// the agent loop using the Registry+Router+MockProvider pattern.
// This simulates the actual handler code path for /v1/agents/run.
func TestFullChatFlow_AgentLoop_ViaRegistryAndRouter(t *testing.T) {
	// Simulate: LLM returns tool call, then after tool execution returns final answer
	mock := testutil.NewMockProvider("agent-provider",
		testutil.ToolCallResponse("call-99", "calculate", `{"expression": "2+2"}`),
		testutil.TextResponse("2 plus 2 equals 4."),
	)

	reg := provider.NewRegistry("agent-provider")
	reg.Register(mock)

	ctx := testutil.NewGofrCtx()

	// Simulate what runAgentLoop does:
	p, modelName, err := reg.ResolveProvider("agent-provider/agent-provider-model")
	if err != nil {
		t.Fatalf("ResolveProvider: %v", err)
	}

	router := routing.NewRouter(
		routing.DefaultRetryPolicy(0, 0),
		routing.NewCooldownTracker(999, time.Millisecond),
		&routing.SimpleStrategy{},
	)

	messages := []models.Message{
		{Role: "user", Content: "What is 2+2?"},
	}

	// Iteration 1: get tool call
	resp1, err := router.ChatCompletion(ctx, p, modelName, models.ChatCompletionRequest{
		Model:    modelName,
		Messages: messages,
	})
	if err != nil {
		t.Fatalf("first chat completion: %v", err)
	}

	choice1 := resp1.Choices[0]
	if choice1.FinishReason != "tool_calls" {
		t.Errorf("first iteration finish reason: want 'tool_calls', got %q", choice1.FinishReason)
	}

	// Simulate tool execution
	tc := choice1.Message.ToolCalls[0]
	if tc.Function.Name != "calculate" {
		t.Errorf("tool name: want 'calculate', got %q", tc.Function.Name)
	}

	toolResult := `{"result": 4}`

	// Build next conversation
	messages = append(messages,
		models.Message{Role: "assistant", ToolCalls: choice1.Message.ToolCalls},
		models.Message{Role: "tool", Content: toolResult, ToolCallID: tc.ID},
	)

	// Iteration 2: final answer
	resp2, err := router.ChatCompletion(ctx, p, modelName, models.ChatCompletionRequest{
		Model:    modelName,
		Messages: messages,
	})
	if err != nil {
		t.Fatalf("second chat completion: %v", err)
	}

	if resp2.Choices[0].FinishReason != "stop" {
		t.Errorf("second iteration finish reason: want 'stop', got %q", resp2.Choices[0].FinishReason)
	}

	if resp2.Choices[0].Message.Content != "2 plus 2 equals 4." {
		t.Errorf("final content: got %q", resp2.Choices[0].Message.Content)
	}

	// Verify the second call had the tool result
	call1 := mock.CallAt(1)
	foundToolResult := false

	for _, msg := range call1.Messages {
		if msg.Role == "tool" && msg.Content == `{"result": 4}` {
			foundToolResult = true
		}
	}

	if !foundToolResult {
		t.Error("second LLM call should have included the tool result in messages")
	}
}
