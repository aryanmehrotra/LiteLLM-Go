package models

import (
	"encoding/json"
	"testing"
)

func TestAgentRunRequest_Defaults(t *testing.T) {
	req := AgentRunRequest{
		Model: "openai/gpt-4o",
		Messages: []Message{
			{Role: "user", Content: "What is the weather in NYC?"},
		},
	}

	if req.MaxIterations != 0 {
		t.Error("expected MaxIterations to default to 0 (handler applies default)")
	}
}

func TestAgentRunResponse_JSON(t *testing.T) {
	resp := AgentRunResponse{
		ID:     "agent-run-123",
		Object: "agent.run",
		Model:  "openai/gpt-4o",
		Steps: []AgentStep{
			{
				Iteration:    1,
				FinishReason: "stop",
			},
		},
		FinalMessage: Message{Role: "assistant", Content: "The weather is sunny."},
		Usage: Usage{
			PromptTokens:     50,
			CompletionTokens: 20,
			TotalTokens:      70,
		},
		FinishReason: "stop",
		Iterations:   1,
	}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got AgentRunResponse
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.ID != resp.ID {
		t.Errorf("ID: want %q, got %q", resp.ID, got.ID)
	}

	if len(got.Steps) != 1 {
		t.Errorf("Steps: want 1, got %d", len(got.Steps))
	}

	if got.FinalMessage.Content != "The weather is sunny." {
		t.Errorf("FinalMessage.Content: unexpected %q", got.FinalMessage.Content)
	}

	if got.Usage.TotalTokens != 70 {
		t.Errorf("Usage.TotalTokens: want 70, got %d", got.Usage.TotalTokens)
	}
}

func TestAgentStep_WithToolCalls(t *testing.T) {
	step := AgentStep{
		Iteration: 1,
		ToolCalls: []ToolCall{
			{
				ID:   "call_123",
				Type: "function",
				Function: FunctionCall{
					Name:      "web_search",
					Arguments: `{"query": "NYC weather"}`,
				},
			},
		},
		ToolResults: []Message{
			{
				Role:       "tool",
				Content:    `{"results": []}`,
				ToolCallID: "call_123",
			},
		},
		FinishReason: "tool_calls",
	}

	b, _ := json.Marshal(step)

	var got AgentStep
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(got.ToolCalls) != 1 {
		t.Errorf("ToolCalls: want 1, got %d", len(got.ToolCalls))
	}

	if got.ToolCalls[0].Function.Name != "web_search" {
		t.Errorf("ToolCall.Function.Name: want 'web_search', got %q", got.ToolCalls[0].Function.Name)
	}

	if len(got.ToolResults) != 1 {
		t.Errorf("ToolResults: want 1, got %d", len(got.ToolResults))
	}
}

func TestAgentRunRequest_WithWebhooks(t *testing.T) {
	req := AgentRunRequest{
		Model: "openai/gpt-4o",
		Messages: []Message{
			{Role: "user", Content: "Run a task"},
		},
		Tools: []Tool{
			{
				Type: "function",
				Function: ToolFunction{
					Name:        "my_tool",
					Description: "Does something",
				},
			},
		},
		ToolWebhooks: map[string]string{
			"my_tool": "https://example.com/webhook",
		},
		MaxIterations: 5,
	}

	b, _ := json.Marshal(req)

	var got AgentRunRequest
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.MaxIterations != 5 {
		t.Errorf("MaxIterations: want 5, got %d", got.MaxIterations)
	}

	if got.ToolWebhooks["my_tool"] != "https://example.com/webhook" {
		t.Errorf("ToolWebhooks not preserved")
	}
}
