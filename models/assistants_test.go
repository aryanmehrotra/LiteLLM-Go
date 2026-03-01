package models

import (
	"encoding/json"
	"testing"
)

func TestAssistant_JSON(t *testing.T) {
	a := Assistant{
		ID:           "asst_abc123",
		Object:       "assistant",
		CreatedAt:    1700000000,
		Name:         "My Assistant",
		Description:  "A helpful assistant",
		Model:        "gpt-4o",
		Instructions: "Be concise and helpful.",
		Tools: []AssistantTool{
			{Type: "code_interpreter"},
		},
		Metadata: map[string]string{"env": "production"},
	}

	b, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Assistant
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.ID != a.ID {
		t.Errorf("ID: want %q, got %q", a.ID, got.ID)
	}

	if got.Name != a.Name {
		t.Errorf("Name: want %q, got %q", a.Name, got.Name)
	}

	if len(got.Tools) != 1 {
		t.Errorf("Tools: want 1, got %d", len(got.Tools))
	}

	if got.Metadata["env"] != "production" {
		t.Errorf("Metadata: want 'production', got %q", got.Metadata["env"])
	}
}

func TestThread_JSON(t *testing.T) {
	thread := Thread{
		ID:        "thread_abc",
		Object:    "thread",
		CreatedAt: 1700000000,
		Metadata:  map[string]string{"user": "u123"},
	}

	b, _ := json.Marshal(thread)

	var got Thread
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.ID != thread.ID {
		t.Errorf("ID: want %q, got %q", thread.ID, got.ID)
	}

	if got.Metadata["user"] != "u123" {
		t.Errorf("Metadata: want 'u123', got %q", got.Metadata["user"])
	}
}

func TestRun_JSON(t *testing.T) {
	startedAt := int64(1700000001)
	completedAt := int64(1700000010)

	run := Run{
		ID:          "run_abc",
		Object:      "thread.run",
		CreatedAt:   1700000000,
		ThreadID:    "thread_xyz",
		AssistantID: "asst_123",
		Status:      "completed",
		Model:       "gpt-4o",
		StartedAt:   &startedAt,
		CompletedAt: &completedAt,
		Usage: &Usage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
	}

	b, _ := json.Marshal(run)

	var got Run
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Status != "completed" {
		t.Errorf("Status: want 'completed', got %q", got.Status)
	}

	if got.StartedAt == nil || *got.StartedAt != startedAt {
		t.Error("StartedAt not preserved")
	}

	if got.Usage == nil || got.Usage.TotalTokens != 150 {
		t.Error("Usage not preserved")
	}
}

func TestThreadMessageObject_JSON(t *testing.T) {
	msg := ThreadMessageObject{
		ID:        "msg_abc",
		Object:    "thread.message",
		CreatedAt: 1700000000,
		ThreadID:  "thread_xyz",
		Role:      "user",
		Content: []MessageContent{
			{
				Type: "text",
				Text: &TextContent{
					Value:       "Hello, world!",
					Annotations: []any{},
				},
			},
		},
	}

	b, _ := json.Marshal(msg)

	var got ThreadMessageObject
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Role != "user" {
		t.Errorf("Role: want 'user', got %q", got.Role)
	}

	if len(got.Content) != 1 {
		t.Errorf("Content: want 1, got %d", len(got.Content))
	}

	if got.Content[0].Text == nil || got.Content[0].Text.Value != "Hello, world!" {
		t.Error("Content text not preserved")
	}
}

func TestAssistantListResponse(t *testing.T) {
	resp := AssistantListResponse{
		Object: "list",
		Data: []Assistant{
			{ID: "asst-1", Model: "gpt-4o"},
			{ID: "asst-2", Model: "gpt-4o-mini"},
		},
		HasMore: false,
	}

	b, _ := json.Marshal(resp)

	var got AssistantListResponse
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(got.Data) != 2 {
		t.Errorf("Data: want 2, got %d", len(got.Data))
	}
}
