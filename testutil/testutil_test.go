package testutil

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"aryanmehrotra/litellm-go/models"
)

// TestMockLLMServer_ServesQueuedTextResponse verifies that the mock server
// returns a queued text response and records the incoming request.
func TestMockLLMServer_ServesQueuedTextResponse(t *testing.T) {
	srv := NewMockLLMServer(t)
	srv.QueueText("Hello from the mock LLM!")

	body := `{"model":"test-model","messages":[{"role":"user","content":"hi"}]}`
	resp, err := http.Post(srv.URL()+"/v1/chat/completions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: want 200, got %d", resp.StatusCode)
	}

	var result models.ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(result.Choices) == 0 {
		t.Fatal("expected at least 1 choice")
	}

	if result.Choices[0].Message.Content != "Hello from the mock LLM!" {
		t.Errorf("content: want 'Hello from the mock LLM!', got %q", result.Choices[0].Message.Content)
	}
}

// TestMockLLMServer_RecordsRequests verifies that request bodies are recorded.
func TestMockLLMServer_RecordsRequests(t *testing.T) {
	srv := NewMockLLMServer(t)
	srv.QueueText("Response 1")
	srv.QueueText("Response 2")

	for i, msg := range []string{"first message", "second message"} {
		body := `{"model":"test","messages":[{"role":"user","content":"` + msg + `"}]}`
		resp, err := http.Post(srv.URL()+"/chat", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}

		resp.Body.Close()
	}

	if srv.RequestCount() != 2 {
		t.Errorf("RequestCount: want 2, got %d", srv.RequestCount())
	}

	// Verify first request body contains "first message"
	req0 := srv.RequestBodyAt(0)
	if req0 == nil {
		t.Fatal("RequestBodyAt(0) is nil")
	}

	if !strings.Contains(string(req0), "first message") {
		t.Errorf("RequestBodyAt(0) should contain 'first message', got %q", string(req0))
	}

	// Verify last request body contains "second message"
	last := srv.LastRequestBody()
	if !strings.Contains(string(last), "second message") {
		t.Errorf("LastRequestBody() should contain 'second message', got %q", string(last))
	}
}

// TestMockLLMServer_SequentialResponses verifies multi-turn: first a tool call
// response, then a text response.
func TestMockLLMServer_SequentialResponses(t *testing.T) {
	srv := NewMockLLMServer(t)
	srv.QueueToolCall("call-1", "lookup_definition", `{"word": "serendipity"}`)
	srv.QueueText("Serendipity means a happy accident.")

	doPost := func(body string) models.ChatCompletionResponse {
		resp, err := http.Post(srv.URL()+"/chat/completions", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("POST failed: %v", err)
		}
		defer resp.Body.Close()

		var result models.ChatCompletionResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decode: %v", err)
		}

		return result
	}

	// First call should return tool_calls
	resp1 := doPost(`{"model":"test","messages":[{"role":"user","content":"define serendipity"}]}`)
	if resp1.Choices[0].FinishReason != "tool_calls" {
		t.Errorf("first response FinishReason: want 'tool_calls', got %q", resp1.Choices[0].FinishReason)
	}

	if len(resp1.Choices[0].Message.ToolCalls) != 1 {
		t.Fatal("expected 1 tool call in first response")
	}

	tc := resp1.Choices[0].Message.ToolCalls[0]
	if tc.Function.Name != "lookup_definition" {
		t.Errorf("tool name: want 'lookup_definition', got %q", tc.Function.Name)
	}

	// Second call should return the final text
	resp2 := doPost(`{"model":"test","messages":[{"role":"tool","content":"..."}]}`)
	if resp2.Choices[0].FinishReason != "stop" {
		t.Errorf("second response FinishReason: want 'stop', got %q", resp2.Choices[0].FinishReason)
	}

	if resp2.Choices[0].Message.Content != "Serendipity means a happy accident." {
		t.Errorf("second response content: got %q", resp2.Choices[0].Message.Content)
	}
}

// TestMockLLMServer_UnexpectedRequest verifies that extra requests beyond the
// queue return an HTTP 500 error.
func TestMockLLMServer_UnexpectedRequest(t *testing.T) {
	srv := NewMockLLMServer(t)
	// No responses queued

	resp, err := http.Post(srv.URL()+"/v1/chat/completions", "application/json",
		strings.NewReader(`{"model":"test","messages":[]}`))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: want 500, got %d", resp.StatusCode)
	}
}

// TestMockLLMServer_QueueError verifies that error responses are served correctly.
func TestMockLLMServer_QueueError(t *testing.T) {
	srv := NewMockLLMServer(t)
	srv.QueueError(http.StatusTooManyRequests, "rate limit exceeded")

	resp, err := http.Post(srv.URL()+"/v1/chat/completions", "application/json",
		strings.NewReader(`{"model":"test","messages":[]}`))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("status: want 429, got %d", resp.StatusCode)
	}
}

// --------------------------------------------------------------------------
// MockProvider tests
// --------------------------------------------------------------------------

// TestMockProvider_ReturnsResponsesInOrder verifies response sequencing.
func TestMockProvider_ReturnsResponsesInOrder(t *testing.T) {
	mock := NewMockProvider("test",
		TextResponse("first"),
		TextResponse("second"),
		TextResponse("third"),
	)

	ctx := NewGofrCtx()

	for i, want := range []string{"first", "second", "third"} {
		req := models.ChatCompletionRequest{
			Model:    "test-model",
			Messages: []models.Message{{Role: "user", Content: "msg"}},
		}

		resp, err := mock.ChatCompletion(ctx, req)
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}

		got := resp.Choices[0].Message.Content
		if got != want {
			t.Errorf("call %d: want %q, got %q", i, want, got)
		}
	}

	if mock.CallCount() != 3 {
		t.Errorf("CallCount: want 3, got %d", mock.CallCount())
	}
}

// TestMockProvider_RecordsRequests verifies that requests are recorded.
func TestMockProvider_RecordsRequests(t *testing.T) {
	mock := NewMockProvider("test",
		TextResponse("answer"),
	)

	ctx := NewGofrCtx()

	req := models.ChatCompletionRequest{
		Model: "test-model",
		Messages: []models.Message{
			{Role: "system", Content: "You are helpful"},
			{Role: "user", Content: "Hello"},
		},
	}

	_, err := mock.ChatCompletion(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	calls := mock.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 recorded call, got %d", len(calls))
	}

	if len(calls[0].Messages) != 2 {
		t.Errorf("recorded call: want 2 messages, got %d", len(calls[0].Messages))
	}

	if calls[0].Messages[0].Role != "system" {
		t.Errorf("recorded messages[0].Role: want 'system', got %q", calls[0].Messages[0].Role)
	}
}

// TestMockProvider_ExhaustsQueue verifies that an error is returned when
// there are no more queued responses.
func TestMockProvider_ExhaustsQueue(t *testing.T) {
	mock := NewMockProvider("test", TextResponse("only one"))
	ctx := NewGofrCtx()

	req := models.ChatCompletionRequest{Model: "test-model"}

	// First call succeeds
	if _, err := mock.ChatCompletion(ctx, req); err != nil {
		t.Fatalf("first call: unexpected error: %v", err)
	}

	// Second call fails (queue exhausted)
	_, err := mock.ChatCompletion(ctx, req)
	if err == nil {
		t.Error("second call: expected error, got nil")
	}
}

// TestMockProvider_ToolCallResponse verifies ToolCallResponse helper.
func TestMockProvider_ToolCallResponse(t *testing.T) {
	resp := ToolCallResponse("call-xyz", "my_function", `{"arg": "val"}`)

	if resp.Choices[0].FinishReason != "tool_calls" {
		t.Errorf("FinishReason: want 'tool_calls', got %q", resp.Choices[0].FinishReason)
	}

	tcs := resp.Choices[0].Message.ToolCalls
	if len(tcs) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(tcs))
	}

	if tcs[0].ID != "call-xyz" {
		t.Errorf("ID: want 'call-xyz', got %q", tcs[0].ID)
	}

	if tcs[0].Function.Name != "my_function" {
		t.Errorf("Function.Name: want 'my_function', got %q", tcs[0].Function.Name)
	}

	if tcs[0].Function.Arguments != `{"arg": "val"}` {
		t.Errorf("Function.Arguments: want %q, got %q", `{"arg": "val"}`, tcs[0].Function.Arguments)
	}
}

// --------------------------------------------------------------------------
// NewGofrCtx test
// --------------------------------------------------------------------------

// TestNewGofrCtx_NotNil verifies that NewGofrCtx returns a non-nil context.
func TestNewGofrCtx_NotNil(t *testing.T) {
	ctx := NewGofrCtx()
	if ctx == nil {
		t.Fatal("NewGofrCtx() returned nil")
	}
}

// TestNewGofrCtx_HasBaseContext verifies the embedded context is not nil.
func TestNewGofrCtx_HasBaseContext(t *testing.T) {
	ctx := NewGofrCtx()
	if ctx.Context == nil {
		t.Fatal("Context.Context is nil")
	}
}

// TestNewGofrCtx_LoggingWorks verifies that calling Logf does not panic.
func TestNewGofrCtx_LoggingWorks(t *testing.T) {
	ctx := NewGofrCtx()
	// Should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Logf panicked: %v", r)
		}
	}()

	ctx.Logf("test log message: %s", "hello")
}
