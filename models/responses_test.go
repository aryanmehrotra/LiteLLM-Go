package models

import (
	"encoding/json"
	"testing"
)

func TestResponseRequest_StringInput(t *testing.T) {
	req := ResponseRequest{
		Model: "openai/gpt-4o",
		Input: "What is 2+2?",
	}

	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got ResponseRequest
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Model != req.Model {
		t.Errorf("Model: want %q, got %q", req.Model, got.Model)
	}

	inputStr, ok := got.Input.(string)
	if !ok {
		t.Fatal("Input should be string")
	}

	if inputStr != "What is 2+2?" {
		t.Errorf("Input: want %q, got %q", "What is 2+2?", inputStr)
	}
}

func TestResponseRequest_WithBuiltinTools(t *testing.T) {
	req := ResponseRequest{
		Model: "openai/gpt-4o",
		Input: "Search for latest AI news",
		Tools: []ResponseBuiltinTool{
			{
				Type:              "web_search_preview",
				SearchContextSize: "medium",
			},
		},
	}

	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got ResponseRequest
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(got.Tools) != 1 {
		t.Errorf("Tools: want 1, got %d", len(got.Tools))
	}

	if got.Tools[0].Type != "web_search_preview" {
		t.Errorf("Tools[0].Type: want 'web_search_preview', got %q", got.Tools[0].Type)
	}
}

func TestResponseObject_JSON(t *testing.T) {
	resp := ResponseObject{
		ID:        "resp_abc123",
		Object:    "response",
		CreatedAt: 1700000000,
		Model:     "gpt-4o",
		Status:    "completed",
		Output: []ResponseOutputItem{
			{
				Type:   "message",
				ID:     "msg_123",
				Status: "completed",
				Role:   "assistant",
				Content: []ResponseOutputContent{
					{
						Type: "output_text",
						Text: "The answer is 4.",
					},
				},
			},
		},
		Usage: ResponseUsage{
			InputTokens:  10,
			OutputTokens: 5,
			TotalTokens:  15,
		},
	}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got ResponseObject
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.ID != resp.ID {
		t.Errorf("ID: want %q, got %q", resp.ID, got.ID)
	}

	if got.Status != "completed" {
		t.Errorf("Status: want 'completed', got %q", got.Status)
	}

	if len(got.Output) != 1 {
		t.Errorf("Output: want 1 item, got %d", len(got.Output))
	}

	if got.Output[0].Content[0].Text != "The answer is 4." {
		t.Errorf("Output text: unexpected %q", got.Output[0].Content[0].Text)
	}

	if got.Usage.TotalTokens != 15 {
		t.Errorf("Usage.TotalTokens: want 15, got %d", got.Usage.TotalTokens)
	}
}

func TestResponseObject_WithFunctionCall(t *testing.T) {
	resp := ResponseObject{
		ID:        "resp_func_123",
		Object:    "response",
		CreatedAt: 1700000000,
		Model:     "gpt-4o",
		Status:    "completed",
		Output: []ResponseOutputItem{
			{
				Type:      "function_call",
				ID:        "fc_123",
				Status:    "completed",
				Name:      "my_function",
				CallID:    "call_xyz",
				Arguments: `{"key": "value"}`,
			},
		},
	}

	b, _ := json.Marshal(resp)

	var got ResponseObject
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Output[0].Type != "function_call" {
		t.Errorf("Type: want 'function_call', got %q", got.Output[0].Type)
	}

	if got.Output[0].Name != "my_function" {
		t.Errorf("Name: want 'my_function', got %q", got.Output[0].Name)
	}
}
