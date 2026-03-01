package models

import (
	"encoding/json"
	"testing"
)

func TestChatCompletionRequest_JSONMarshal(t *testing.T) {
	temp := 0.7
	maxTokens := 100

	tests := []struct {
		name     string
		req      ChatCompletionRequest
		wantKeys []string // keys expected in JSON output
	}{
		{
			name: "basic request marshals correctly",
			req: ChatCompletionRequest{
				Model: "gpt-4o",
				Messages: []Message{
					{Role: "user", Content: "Hello"},
				},
			},
			wantKeys: []string{"model", "messages"},
		},
		{
			name: "request with optional fields",
			req: ChatCompletionRequest{
				Model: "gpt-4o",
				Messages: []Message{
					{Role: "system", Content: "You are a helper"},
					{Role: "user", Content: "Hi"},
				},
				Temperature: &temp,
				MaxTokens:   &maxTokens,
				Stream:      true,
			},
			wantKeys: []string{"model", "messages", "temperature", "max_tokens", "stream"},
		},
		{
			name: "request with tools",
			req: ChatCompletionRequest{
				Model: "gpt-4o",
				Messages: []Message{
					{Role: "user", Content: "What is the weather?"},
				},
				Tools: []Tool{
					{
						Type: "function",
						Function: ToolFunction{
							Name:        "get_weather",
							Description: "Get weather for a location",
							Parameters: map[string]any{
								"type": "object",
								"properties": map[string]any{
									"location": map[string]any{"type": "string"},
								},
							},
						},
					},
				},
			},
			wantKeys: []string{"model", "messages", "tools"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.req)
			if err != nil {
				t.Fatalf("json.Marshal() error: %v", err)
			}

			var m map[string]any
			if err := json.Unmarshal(data, &m); err != nil {
				t.Fatalf("json.Unmarshal() error: %v", err)
			}

			for _, key := range tt.wantKeys {
				if _, ok := m[key]; !ok {
					t.Errorf("expected key %q in JSON output, not found", key)
				}
			}
		})
	}
}

func TestChatCompletionRequest_JSONUnmarshal(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantModel   string
		wantMsgLen  int
		wantRole    string
		wantContent string
	}{
		{
			name:        "basic request unmarshals correctly",
			input:       `{"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}`,
			wantModel:   "gpt-4o",
			wantMsgLen:  1,
			wantRole:    "user",
			wantContent: "Hello",
		},
		{
			name:        "multi-message request",
			input:       `{"model":"claude-sonnet-4-20250514","messages":[{"role":"system","content":"Be nice"},{"role":"user","content":"Hi"}]}`,
			wantModel:   "claude-sonnet-4-20250514",
			wantMsgLen:  2,
			wantRole:    "system",
			wantContent: "Be nice",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req ChatCompletionRequest
			if err := json.Unmarshal([]byte(tt.input), &req); err != nil {
				t.Fatalf("json.Unmarshal() error: %v", err)
			}

			if req.Model != tt.wantModel {
				t.Errorf("Model = %q, want %q", req.Model, tt.wantModel)
			}

			if len(req.Messages) != tt.wantMsgLen {
				t.Fatalf("len(Messages) = %d, want %d", len(req.Messages), tt.wantMsgLen)
			}

			if req.Messages[0].Role != tt.wantRole {
				t.Errorf("Messages[0].Role = %q, want %q", req.Messages[0].Role, tt.wantRole)
			}

			if req.Messages[0].Content != tt.wantContent {
				t.Errorf("Messages[0].Content = %q, want %q", req.Messages[0].Content, tt.wantContent)
			}
		})
	}
}

func TestChatCompletionRequest_RoundTrip(t *testing.T) {
	temp := 0.5
	topP := 0.9
	maxTokens := 256

	original := ChatCompletionRequest{
		Model: "gpt-4o",
		Messages: []Message{
			{Role: "system", Content: "You are a helpful assistant"},
			{Role: "user", Content: "Translate hello to French"},
		},
		Temperature: &temp,
		TopP:        &topP,
		MaxTokens:   &maxTokens,
		Stop:        []string{"\n", "END"},
		Tools: []Tool{
			{
				Type: "function",
				Function: ToolFunction{
					Name:        "translate",
					Description: "Translate text",
				},
			},
		},
		ToolChoice: "auto",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded ChatCompletionRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Model != original.Model {
		t.Errorf("Model = %q, want %q", decoded.Model, original.Model)
	}

	if len(decoded.Messages) != len(original.Messages) {
		t.Fatalf("Messages length = %d, want %d", len(decoded.Messages), len(original.Messages))
	}

	if *decoded.Temperature != *original.Temperature {
		t.Errorf("Temperature = %v, want %v", *decoded.Temperature, *original.Temperature)
	}

	if *decoded.TopP != *original.TopP {
		t.Errorf("TopP = %v, want %v", *decoded.TopP, *original.TopP)
	}

	if *decoded.MaxTokens != *original.MaxTokens {
		t.Errorf("MaxTokens = %v, want %v", *decoded.MaxTokens, *original.MaxTokens)
	}

	if len(decoded.Stop) != 2 {
		t.Fatalf("Stop length = %d, want 2", len(decoded.Stop))
	}

	if len(decoded.Tools) != 1 {
		t.Fatalf("Tools length = %d, want 1", len(decoded.Tools))
	}

	if decoded.Tools[0].Function.Name != "translate" {
		t.Errorf("Tools[0].Function.Name = %q, want %q", decoded.Tools[0].Function.Name, "translate")
	}
}

func TestMessage_WithToolCalls(t *testing.T) {
	tests := []struct {
		name           string
		msg            Message
		wantToolCalls  int
		wantToolCallID string
	}{
		{
			name: "message with tool calls marshals correctly",
			msg: Message{
				Role:    "assistant",
				Content: "",
				ToolCalls: []ToolCall{
					{
						ID:   "call_123",
						Type: "function",
						Function: FunctionCall{
							Name:      "get_weather",
							Arguments: `{"location":"Paris"}`,
						},
					},
					{
						ID:   "call_456",
						Type: "function",
						Function: FunctionCall{
							Name:      "get_time",
							Arguments: `{"timezone":"CET"}`,
						},
					},
				},
			},
			wantToolCalls: 2,
		},
		{
			name: "tool result message with tool_call_id",
			msg: Message{
				Role:       "tool",
				Content:    `{"temperature": 22, "unit": "celsius"}`,
				ToolCallID: "call_123",
			},
			wantToolCallID: "call_123",
		},
		{
			name: "regular message without tool calls omits field",
			msg: Message{
				Role:    "user",
				Content: "Hello",
			},
			wantToolCalls: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.msg)
			if err != nil {
				t.Fatalf("json.Marshal() error: %v", err)
			}

			var decoded Message
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("json.Unmarshal() error: %v", err)
			}

			if len(decoded.ToolCalls) != tt.wantToolCalls {
				t.Errorf("len(ToolCalls) = %d, want %d", len(decoded.ToolCalls), tt.wantToolCalls)
			}

			if tt.wantToolCallID != "" && decoded.ToolCallID != tt.wantToolCallID {
				t.Errorf("ToolCallID = %q, want %q", decoded.ToolCallID, tt.wantToolCallID)
			}

			// Verify omitempty: tool_calls should not appear in JSON for empty slices
			if tt.wantToolCalls == 0 {
				var m map[string]any
				if err := json.Unmarshal(data, &m); err != nil {
					t.Fatalf("json.Unmarshal to map error: %v", err)
				}

				if _, ok := m["tool_calls"]; ok {
					t.Error("tool_calls should be omitted from JSON when empty")
				}
			}

			// Verify tool call content round-trips
			if tt.wantToolCalls > 0 {
				if decoded.ToolCalls[0].Function.Name != tt.msg.ToolCalls[0].Function.Name {
					t.Errorf("ToolCalls[0].Function.Name = %q, want %q",
						decoded.ToolCalls[0].Function.Name, tt.msg.ToolCalls[0].Function.Name)
				}

				if decoded.ToolCalls[0].Function.Arguments != tt.msg.ToolCalls[0].Function.Arguments {
					t.Errorf("ToolCalls[0].Function.Arguments = %q, want %q",
						decoded.ToolCalls[0].Function.Arguments, tt.msg.ToolCalls[0].Function.Arguments)
				}
			}
		})
	}
}

func TestStreamChunk_Structure(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		wantID           string
		wantObject       string
		wantModel        string
		wantChoicesLen   int
		wantDeltaContent string
		wantFinish       bool // whether finish_reason is non-nil
	}{
		{
			name: "content chunk",
			input: `{
				"id": "chatcmpl-abc123",
				"object": "chat.completion.chunk",
				"created": 1700000000,
				"model": "gpt-4o",
				"choices": [
					{
						"index": 0,
						"delta": {"content": "Hello"},
						"finish_reason": null
					}
				]
			}`,
			wantID:           "chatcmpl-abc123",
			wantObject:       "chat.completion.chunk",
			wantModel:        "gpt-4o",
			wantChoicesLen:   1,
			wantDeltaContent: "Hello",
			wantFinish:       false,
		},
		{
			name: "role-only first chunk",
			input: `{
				"id": "chatcmpl-def456",
				"object": "chat.completion.chunk",
				"created": 1700000001,
				"model": "gpt-4o-mini",
				"choices": [
					{
						"index": 0,
						"delta": {"role": "assistant"},
						"finish_reason": null
					}
				]
			}`,
			wantID:         "chatcmpl-def456",
			wantObject:     "chat.completion.chunk",
			wantModel:      "gpt-4o-mini",
			wantChoicesLen: 1,
			wantFinish:     false,
		},
		{
			name: "final chunk with finish reason and usage",
			input: `{
				"id": "chatcmpl-ghi789",
				"object": "chat.completion.chunk",
				"created": 1700000002,
				"model": "gpt-4o",
				"choices": [
					{
						"index": 0,
						"delta": {},
						"finish_reason": "stop"
					}
				],
				"usage": {
					"prompt_tokens": 10,
					"completion_tokens": 20,
					"total_tokens": 30
				}
			}`,
			wantID:         "chatcmpl-ghi789",
			wantObject:     "chat.completion.chunk",
			wantModel:      "gpt-4o",
			wantChoicesLen: 1,
			wantFinish:     true,
		},
		{
			name: "chunk with streaming tool call delta",
			input: `{
				"id": "chatcmpl-tool1",
				"object": "chat.completion.chunk",
				"created": 1700000003,
				"model": "gpt-4o",
				"choices": [
					{
						"index": 0,
						"delta": {
							"tool_calls": [
								{
									"index": 0,
									"id": "call_abc",
									"type": "function",
									"function": {"name": "get_weather", "arguments": "{\"loc"}
								}
							]
						},
						"finish_reason": null
					}
				]
			}`,
			wantID:         "chatcmpl-tool1",
			wantChoicesLen: 1,
			wantFinish:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var chunk StreamChunk
			if err := json.Unmarshal([]byte(tt.input), &chunk); err != nil {
				t.Fatalf("json.Unmarshal() error: %v", err)
			}

			if chunk.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", chunk.ID, tt.wantID)
			}

			if tt.wantObject != "" && chunk.Object != tt.wantObject {
				t.Errorf("Object = %q, want %q", chunk.Object, tt.wantObject)
			}

			if tt.wantModel != "" && chunk.Model != tt.wantModel {
				t.Errorf("Model = %q, want %q", chunk.Model, tt.wantModel)
			}

			if len(chunk.Choices) != tt.wantChoicesLen {
				t.Fatalf("len(Choices) = %d, want %d", len(chunk.Choices), tt.wantChoicesLen)
			}

			if tt.wantDeltaContent != "" && chunk.Choices[0].Delta.Content != tt.wantDeltaContent {
				t.Errorf("Delta.Content = %q, want %q",
					chunk.Choices[0].Delta.Content, tt.wantDeltaContent)
			}

			if tt.wantFinish && chunk.Choices[0].FinishReason == nil {
				t.Error("expected FinishReason to be non-nil")
			}

			if !tt.wantFinish && chunk.Choices[0].FinishReason != nil {
				t.Errorf("expected FinishReason to be nil, got %q", *chunk.Choices[0].FinishReason)
			}
		})
	}
}

func TestStreamChunk_WithUsage(t *testing.T) {
	input := `{
		"id": "chatcmpl-usage",
		"object": "chat.completion.chunk",
		"created": 1700000000,
		"model": "gpt-4o",
		"choices": [],
		"usage": {
			"prompt_tokens": 50,
			"completion_tokens": 100,
			"total_tokens": 150
		}
	}`

	var chunk StreamChunk
	if err := json.Unmarshal([]byte(input), &chunk); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}

	if chunk.Usage == nil {
		t.Fatal("Usage should not be nil")
	}

	if chunk.Usage.PromptTokens != 50 {
		t.Errorf("Usage.PromptTokens = %d, want 50", chunk.Usage.PromptTokens)
	}

	if chunk.Usage.CompletionTokens != 100 {
		t.Errorf("Usage.CompletionTokens = %d, want 100", chunk.Usage.CompletionTokens)
	}

	if chunk.Usage.TotalTokens != 150 {
		t.Errorf("Usage.TotalTokens = %d, want 150", chunk.Usage.TotalTokens)
	}
}

func TestChatCompletionResponse_JSONRoundTrip(t *testing.T) {
	resp := ChatCompletionResponse{
		ID:      "chatcmpl-test",
		Object:  "chat.completion",
		Created: 1700000000,
		Model:   "gpt-4o",
		Choices: []Choice{
			{
				Index: 0,
				Message: Message{
					Role:    "assistant",
					Content: "Hello! How can I help?",
				},
				FinishReason: "stop",
			},
		},
		Usage: Usage{
			PromptTokens:     10,
			CompletionTokens: 8,
			TotalTokens:      18,
		},
		Provider: "openai",
		Cost:     0.0075,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded ChatCompletionResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.ID != resp.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, resp.ID)
	}

	if decoded.Model != resp.Model {
		t.Errorf("Model = %q, want %q", decoded.Model, resp.Model)
	}

	if len(decoded.Choices) != 1 {
		t.Fatalf("len(Choices) = %d, want 1", len(decoded.Choices))
	}

	if decoded.Choices[0].Message.Content != "Hello! How can I help?" {
		t.Errorf("Choices[0].Message.Content = %q, want %q",
			decoded.Choices[0].Message.Content, "Hello! How can I help?")
	}

	if decoded.Provider != "openai" {
		t.Errorf("Provider = %q, want %q", decoded.Provider, "openai")
	}

	if decoded.Usage.TotalTokens != 18 {
		t.Errorf("Usage.TotalTokens = %d, want 18", decoded.Usage.TotalTokens)
	}
}

func TestOmitEmptyFields(t *testing.T) {
	tests := []struct {
		name     string
		req      ChatCompletionRequest
		notWant  []string // keys that should NOT appear in JSON
	}{
		{
			name: "omits temperature when nil",
			req: ChatCompletionRequest{
				Model:    "gpt-4o",
				Messages: []Message{{Role: "user", Content: "Hi"}},
			},
			notWant: []string{"temperature", "top_p", "max_tokens", "stream", "stop",
				"presence_penalty", "frequency_penalty", "tools", "tool_choice"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.req)
			if err != nil {
				t.Fatalf("json.Marshal() error: %v", err)
			}

			var m map[string]any
			if err := json.Unmarshal(data, &m); err != nil {
				t.Fatalf("json.Unmarshal() error: %v", err)
			}

			for _, key := range tt.notWant {
				if _, ok := m[key]; ok {
					t.Errorf("key %q should be omitted from JSON when not set", key)
				}
			}
		})
	}
}
