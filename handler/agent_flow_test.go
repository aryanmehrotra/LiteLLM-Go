package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aryanmehrotra/litellm-go/models"
	"aryanmehrotra/litellm-go/provider"
	"aryanmehrotra/litellm-go/routing"
	"aryanmehrotra/litellm-go/testutil"
)

// buildAgentTestHandler creates an APIHandler wired to a MockProvider.
// It uses zero retries and a disabled cooldown so tests run fast.
func buildAgentTestHandler(mock *testutil.MockProvider) *APIHandler {
	reg := provider.NewRegistry(mock.Name())
	reg.Register(mock)

	router := routing.NewRouter(
		routing.DefaultRetryPolicy(0, 0),
		routing.NewCooldownTracker(999, time.Millisecond),
		&routing.SimpleStrategy{},
	)

	return &APIHandler{
		Registry: reg,
		Router:   router,
	}
}

// --------------------------------------------------------------------------
// runAgentLoop tests
// --------------------------------------------------------------------------

func TestRunAgentLoop_SingleTurn_Stop(t *testing.T) {
	mock := testutil.NewMockProvider("mock",
		testutil.TextResponse("The capital of France is Paris."),
	)
	h := buildAgentTestHandler(mock)
	ctx := testutil.NewGofrCtx()

	req := models.AgentRunRequest{
		Model: "mock/mock-model",
		Messages: []models.Message{
			{Role: "user", Content: "What is the capital of France?"},
		},
		MaxIterations: 5,
	}

	result, err := h.runAgentLoop(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.FinishReason != "stop" {
		t.Errorf("FinishReason: want 'stop', got %q", result.FinishReason)
	}

	if result.FinalMessage.Content != "The capital of France is Paris." {
		t.Errorf("FinalMessage.Content: want 'The capital of France is Paris.', got %q", result.FinalMessage.Content)
	}

	if result.Iterations != 1 {
		t.Errorf("Iterations: want 1, got %d", result.Iterations)
	}

	if result.Usage.TotalTokens != 30 {
		t.Errorf("Usage.TotalTokens: want 30, got %d", result.Usage.TotalTokens)
	}

	if mock.CallCount() != 1 {
		t.Errorf("LLM calls: want 1, got %d", mock.CallCount())
	}

	// Verify the request sent to the LLM contained the original message
	call0 := mock.CallAt(0)
	if len(call0.Messages) < 1 || call0.Messages[0].Content != "What is the capital of France?" {
		t.Errorf("first LLM call did not include original user message")
	}
}

func TestRunAgentLoop_AccumulatesUsage(t *testing.T) {
	mock := testutil.NewMockProvider("mock",
		testutil.ToolCallResponse("call-1", "get_info", `{"id": "1"}`),
		testutil.TextResponse("Done."),
	)
	h := buildAgentTestHandler(mock)
	// Register a webhook for "get_info" using an in-process test server
	webhookSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"result": "info retrieved"}`))
	}))
	defer webhookSrv.Close()

	ctx := testutil.NewGofrCtx()

	req := models.AgentRunRequest{
		Model: "mock/mock-model",
		Messages: []models.Message{
			{Role: "user", Content: "Get some info"},
		},
		Tools: []models.Tool{{
			Type:     "function",
			Function: models.ToolFunction{Name: "get_info"},
		}},
		ToolWebhooks: map[string]string{
			"get_info": webhookSrv.URL,
		},
		MaxIterations: 5,
	}

	result, err := h.runAgentLoop(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// First call: 15 prompt + 10 completion = 25
	// Second call: 10 prompt + 20 completion = 30
	// Total: 55
	if result.Usage.TotalTokens != 55 {
		t.Errorf("Usage.TotalTokens: want 55 (25+30), got %d", result.Usage.TotalTokens)
	}

	if mock.CallCount() != 2 {
		t.Errorf("LLM calls: want 2, got %d", mock.CallCount())
	}
}

func TestRunAgentLoop_WebhookToolExecution(t *testing.T) {
	webhookSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"temperature": "22°C", "city": "Berlin"}`))
	}))
	defer webhookSrv.Close()

	mock := testutil.NewMockProvider("mock",
		testutil.ToolCallResponse("call-weather", "get_weather", `{"location": "Berlin"}`),
		testutil.TextResponse("The temperature in Berlin is 22°C."),
	)
	h := buildAgentTestHandler(mock)
	ctx := testutil.NewGofrCtx()

	req := models.AgentRunRequest{
		Model: "mock/mock-model",
		Messages: []models.Message{
			{Role: "user", Content: "What's the weather in Berlin?"},
		},
		Tools: []models.Tool{{
			Type:     "function",
			Function: models.ToolFunction{Name: "get_weather"},
		}},
		ToolWebhooks: map[string]string{
			"get_weather": webhookSrv.URL,
		},
		MaxIterations: 3,
	}

	result, err := h.runAgentLoop(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.FinishReason != "stop" {
		t.Errorf("FinishReason: want 'stop', got %q", result.FinishReason)
	}

	if mock.CallCount() != 2 {
		t.Errorf("LLM calls: want 2, got %d", mock.CallCount())
	}

	// Second LLM call must have received the tool result in conversation history
	call1 := mock.CallAt(1)
	foundToolResult := false

	for _, msg := range call1.Messages {
		if msg.Role == "tool" && msg.ToolCallID == "call-weather" {
			foundToolResult = true
			if !strings.Contains(msg.Content, "22") {
				t.Errorf("tool result message should contain '22', got %q", msg.Content)
			}
		}
	}

	if !foundToolResult {
		t.Error("second LLM call did not include tool result message")
	}

	// Final answer should be present
	if result.FinalMessage.Content != "The temperature in Berlin is 22°C." {
		t.Errorf("FinalMessage.Content: got %q", result.FinalMessage.Content)
	}

	// Step trace: step 1 has tool call, step 2 has final answer
	if len(result.Steps) != 2 {
		t.Errorf("Steps: want 2, got %d", len(result.Steps))
	}

	if len(result.Steps[0].ToolCalls) != 1 {
		t.Error("step[0] should have 1 tool call")
	}

	if result.Steps[0].ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("step[0].ToolCalls[0].Function.Name: want 'get_weather', got %q",
			result.Steps[0].ToolCalls[0].Function.Name)
	}
}

func TestRunAgentLoop_MaxIterationsHardCap(t *testing.T) {
	// Provide 60 tool-call responses; the agent should stop at 50 (hard cap).
	responses := make([]*models.ChatCompletionResponse, 60)
	for i := range responses {
		responses[i] = testutil.ToolCallResponse(
			fmt.Sprintf("call-%d", i),
			"get_info",
			`{"n": 1}`,
		)
	}

	mock := testutil.NewMockProvider("mock", responses...)
	h := buildAgentTestHandler(mock)
	ctx := testutil.NewGofrCtx()

	req := models.AgentRunRequest{
		Model: "mock/mock-model",
		Messages: []models.Message{
			{Role: "user", Content: "Loop forever"},
		},
		Tools: []models.Tool{{
			Type:     "function",
			Function: models.ToolFunction{Name: "get_info"},
		}},
		MaxIterations: 100, // will be capped at 50
	}

	result, err := h.runAgentLoop(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.FinishReason != "max_iterations" {
		t.Errorf("FinishReason: want 'max_iterations', got %q", result.FinishReason)
	}

	// Must not have exceeded the hard cap of 50 iterations
	if mock.CallCount() > 50 {
		t.Errorf("LLM calls: exceeded 50 (hard cap), got %d", mock.CallCount())
	}
}

func TestRunAgentLoop_MaxIterationsRespected(t *testing.T) {
	// Supply enough tool-call responses to fill max=3 iterations
	responses := []*models.ChatCompletionResponse{
		testutil.ToolCallResponse("c1", "tool_a", `{}`),
		testutil.ToolCallResponse("c2", "tool_a", `{}`),
		testutil.ToolCallResponse("c3", "tool_a", `{}`),
		testutil.TextResponse("done"),
	}

	mock := testutil.NewMockProvider("mock", responses...)
	h := buildAgentTestHandler(mock)
	ctx := testutil.NewGofrCtx()

	req := models.AgentRunRequest{
		Model: "mock/mock-model",
		Messages: []models.Message{
			{Role: "user", Content: "Do three things"},
		},
		Tools: []models.Tool{{
			Type:     "function",
			Function: models.ToolFunction{Name: "tool_a"},
		}},
		MaxIterations: 3,
	}

	result, err := h.runAgentLoop(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.FinishReason != "max_iterations" {
		t.Errorf("FinishReason: want 'max_iterations', got %q", result.FinishReason)
	}

	if mock.CallCount() != 3 {
		t.Errorf("LLM calls: want 3, got %d", mock.CallCount())
	}
}

func TestRunAgentLoop_UnknownToolNoWebhook(t *testing.T) {
	mock := testutil.NewMockProvider("mock",
		testutil.ToolCallResponse("call-1", "unknown_tool", `{}`),
		testutil.TextResponse("I tried the tool."),
	)
	h := buildAgentTestHandler(mock)
	ctx := testutil.NewGofrCtx()

	req := models.AgentRunRequest{
		Model: "mock/mock-model",
		Messages: []models.Message{
			{Role: "user", Content: "Use unknown tool"},
		},
		Tools: []models.Tool{{
			Type:     "function",
			Function: models.ToolFunction{Name: "unknown_tool"},
		}},
		MaxIterations: 3,
	}

	_, err := h.runAgentLoop(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Agent should continue even when tool is unknown (error message is passed back as tool result)
	if mock.CallCount() != 2 {
		t.Errorf("LLM calls: want 2, got %d", mock.CallCount())
	}

	// Second call should include the error message for the unknown tool
	call1 := mock.CallAt(1)
	foundToolMsg := false

	for _, msg := range call1.Messages {
		if msg.Role == "tool" && strings.Contains(msg.Content, "not registered") {
			foundToolMsg = true
		}
	}

	if !foundToolMsg {
		t.Error("second call should include 'not registered' tool error message")
	}
}

func TestRunAgentLoop_WebSearchBuiltin_NoSearchService(t *testing.T) {
	mock := testutil.NewMockProvider("mock",
		testutil.ToolCallResponse("call-1", "web_search", `{"query": "golang testing"}`),
		testutil.TextResponse("I searched and found some results."),
	)
	h := buildAgentTestHandler(mock)
	h.Search = nil // no search service configured
	ctx := testutil.NewGofrCtx()

	req := models.AgentRunRequest{
		Model: "mock/mock-model",
		Messages: []models.Message{
			{Role: "user", Content: "Search for golang testing"},
		},
		Tools: []models.Tool{{
			Type:     "function",
			Function: models.ToolFunction{Name: "web_search"},
		}},
		MaxIterations: 5,
	}

	result, err := h.runAgentLoop(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should gracefully handle missing search service
	if mock.CallCount() != 2 {
		t.Errorf("LLM calls: want 2, got %d", mock.CallCount())
	}

	// Tool result should indicate web search is not configured
	call1 := mock.CallAt(1)
	foundSearchError := false

	for _, msg := range call1.Messages {
		if msg.Role == "tool" && strings.Contains(msg.Content, "not configured") {
			foundSearchError = true
		}
	}

	if !foundSearchError {
		t.Error("second call should include web search not-configured error")
	}

	// The agent should still reach a final answer
	if result.FinalMessage.Content == "" {
		t.Error("expected non-empty final message")
	}
}

func TestRunAgentLoop_MultipleToolCallsInOneTurn(t *testing.T) {
	// Single LLM response that requests 2 tool calls at once
	twoToolCalls := testutil.MultiToolCallResponse([]models.ToolCall{
		{
			ID:       "call-a",
			Type:     "function",
			Function: models.FunctionCall{Name: "tool_one", Arguments: `{"x": 1}`},
		},
		{
			ID:       "call-b",
			Type:     "function",
			Function: models.FunctionCall{Name: "tool_two", Arguments: `{"y": 2}`},
		},
	})
	finalResp := testutil.TextResponse("Both tools executed.")

	callCountA, callCountB := 0, 0

	webhookA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCountA++
		w.Write([]byte(`{"result": "tool_one done"}`))
	}))
	defer webhookA.Close()

	webhookB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCountB++
		w.Write([]byte(`{"result": "tool_two done"}`))
	}))
	defer webhookB.Close()

	mock := testutil.NewMockProvider("mock", twoToolCalls, finalResp)
	h := buildAgentTestHandler(mock)
	ctx := testutil.NewGofrCtx()

	req := models.AgentRunRequest{
		Model: "mock/mock-model",
		Messages: []models.Message{
			{Role: "user", Content: "Run both tools"},
		},
		Tools: []models.Tool{
			{Type: "function", Function: models.ToolFunction{Name: "tool_one"}},
			{Type: "function", Function: models.ToolFunction{Name: "tool_two"}},
		},
		ToolWebhooks: map[string]string{
			"tool_one": webhookA.URL,
			"tool_two": webhookB.URL,
		},
		MaxIterations: 3,
	}

	result, err := h.runAgentLoop(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both webhooks called
	if callCountA != 1 {
		t.Errorf("webhook A called %d times, want 1", callCountA)
	}

	if callCountB != 1 {
		t.Errorf("webhook B called %d times, want 1", callCountB)
	}

	// Second LLM call receives both tool results
	call1 := mock.CallAt(1)
	toolMsgCount := 0

	for _, msg := range call1.Messages {
		if msg.Role == "tool" {
			toolMsgCount++
		}
	}

	if toolMsgCount != 2 {
		t.Errorf("second call should have 2 tool result messages, got %d", toolMsgCount)
	}

	if result.FinalMessage.Content != "Both tools executed." {
		t.Errorf("FinalMessage: want 'Both tools executed.', got %q", result.FinalMessage.Content)
	}
}

func TestRunAgentLoop_DefaultMaxIterations(t *testing.T) {
	// When MaxIterations is 0, default (10) applies
	// Provide exactly 1 stop response
	mock := testutil.NewMockProvider("mock",
		testutil.TextResponse("Answer with default iterations"),
	)
	h := buildAgentTestHandler(mock)
	ctx := testutil.NewGofrCtx()

	req := models.AgentRunRequest{
		Model: "mock/mock-model",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
		MaxIterations: 0, // should default to 10
	}

	result, err := h.runAgentLoop(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.FinishReason != "stop" {
		t.Errorf("FinishReason: want 'stop', got %q", result.FinishReason)
	}

	if mock.CallCount() != 1 {
		t.Errorf("LLM calls: want 1, got %d", mock.CallCount())
	}
}

func TestRunAgentLoop_InvalidModel(t *testing.T) {
	mock := testutil.NewMockProvider("mock")
	h := buildAgentTestHandler(mock)
	ctx := testutil.NewGofrCtx()

	req := models.AgentRunRequest{
		Model: "nonexistent-provider/model",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	_, err := h.runAgentLoop(ctx, req)
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
}

func TestRunAgentLoop_ConversationHistory_PreservesContext(t *testing.T) {
	mock := testutil.NewMockProvider("mock",
		testutil.ToolCallResponse("c1", "lookup", `{"term": "photosynthesis"}`),
		testutil.TextResponse("Photosynthesis is the process plants use to convert sunlight to energy."),
	)
	h := buildAgentTestHandler(mock)

	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"definition": "the process by which plants synthesize food from sunlight"}`))
	}))
	defer webhook.Close()

	ctx := testutil.NewGofrCtx()

	req := models.AgentRunRequest{
		Model: "mock/mock-model",
		Messages: []models.Message{
			{Role: "system", Content: "You are a biology tutor."},
			{Role: "user", Content: "Explain photosynthesis"},
		},
		Tools: []models.Tool{{
			Type:     "function",
			Function: models.ToolFunction{Name: "lookup"},
		}},
		ToolWebhooks: map[string]string{
			"lookup": webhook.URL,
		},
		MaxIterations: 5,
	}

	result, err := h.runAgentLoop(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// First LLM call should include both system and user messages
	call0 := mock.CallAt(0)
	if len(call0.Messages) < 2 {
		t.Errorf("first call: want at least 2 messages, got %d", len(call0.Messages))
	}

	if call0.Messages[0].Role != "system" {
		t.Errorf("first message role: want 'system', got %q", call0.Messages[0].Role)
	}

	// Second LLM call should include system + user + assistant (tool call) + tool result
	call1 := mock.CallAt(1)
	if len(call1.Messages) < 4 {
		t.Errorf("second call: want at least 4 messages (system, user, assistant, tool), got %d", len(call1.Messages))
	}

	if result.FinishReason != "stop" {
		t.Errorf("FinishReason: want 'stop', got %q", result.FinishReason)
	}
}

// --------------------------------------------------------------------------
// executeToolCall tests
// --------------------------------------------------------------------------

func TestExecuteToolCall_UnknownTool_NoWebhook(t *testing.T) {
	h := &APIHandler{}
	ctx := testutil.NewGofrCtx()

	tc := models.ToolCall{
		ID:   "call-1",
		Type: "function",
		Function: models.FunctionCall{
			Name:      "mystery_tool",
			Arguments: `{}`,
		},
	}

	result := h.executeToolCall(ctx, tc, nil)

	if !strings.Contains(result, "not registered") {
		t.Errorf("expected 'not registered' in result, got %q", result)
	}
}

func TestExecuteToolCall_WebhookTool_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status": "ok", "value": 42}`))
	}))
	defer srv.Close()

	h := &APIHandler{}
	ctx := testutil.NewGofrCtx()

	tc := models.ToolCall{
		ID:   "call-42",
		Type: "function",
		Function: models.FunctionCall{
			Name:      "my_tool",
			Arguments: `{"param": "value"}`,
		},
	}

	result := h.executeToolCall(ctx, tc, map[string]string{"my_tool": srv.URL})

	if !strings.Contains(result, "42") {
		t.Errorf("expected '42' in webhook result, got %q", result)
	}
}

func TestExecuteToolCall_WebSearch_NotConfigured(t *testing.T) {
	h := &APIHandler{Search: nil}
	ctx := testutil.NewGofrCtx()

	tc := models.ToolCall{
		ID:   "call-search",
		Type: "function",
		Function: models.FunctionCall{
			Name:      "web_search",
			Arguments: `{"query": "golang"}`,
		},
	}

	result := h.executeToolCall(ctx, tc, nil)

	if !strings.Contains(result, "not configured") {
		t.Errorf("expected 'not configured', got %q", result)
	}
}

func TestExecuteWebSearch_InvalidJSON(t *testing.T) {
	h := &APIHandler{Search: nil}
	ctx := testutil.NewGofrCtx()

	result := h.executeWebSearch(ctx, `not valid json`)

	if !strings.Contains(result, "error") {
		t.Errorf("expected error for invalid JSON args, got %q", result)
	}
}

func TestExecuteWebSearch_EmptyQuery(t *testing.T) {
	h := &APIHandler{Search: nil}
	ctx := testutil.NewGofrCtx()

	result := h.executeWebSearch(ctx, `{"query": ""}`)

	if !strings.Contains(result, "error") {
		t.Errorf("expected error for empty query, got %q", result)
	}
}
