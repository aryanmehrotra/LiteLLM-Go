package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"aryanmehrotra/litellm-go/models"
)

func TestTranslateResponseInput_String(t *testing.T) {
	req := models.ResponseRequest{
		Model: "openai/gpt-4o",
		Input: "Hello, world!",
	}

	msgs, err := translateResponseInput(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	if msgs[0].Role != "user" {
		t.Errorf("Role: want 'user', got %q", msgs[0].Role)
	}

	if msgs[0].Content != "Hello, world!" {
		t.Errorf("Content: want 'Hello, world!', got %q", msgs[0].Content)
	}
}

func TestTranslateResponseInput_WithInstructions(t *testing.T) {
	req := models.ResponseRequest{
		Model:        "openai/gpt-4o",
		Input:        "Write a poem",
		Instructions: "You are a creative poet.",
	}

	msgs, err := translateResponseInput(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have system message + user message
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}

	if msgs[0].Role != "system" {
		t.Errorf("msgs[0].Role: want 'system', got %q", msgs[0].Role)
	}

	if msgs[0].Content != "You are a creative poet." {
		t.Errorf("system content mismatch")
	}

	if msgs[1].Role != "user" {
		t.Errorf("msgs[1].Role: want 'user', got %q", msgs[1].Role)
	}
}

func TestTranslateResponseInput_ArrayInput(t *testing.T) {
	input := []any{
		map[string]any{
			"role":    "user",
			"content": "What is Go?",
		},
	}

	req := models.ResponseRequest{
		Model: "openai/gpt-4o",
		Input: input,
	}

	msgs, err := translateResponseInput(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(msgs) < 1 {
		t.Fatal("expected at least 1 message")
	}

	userMsg := msgs[len(msgs)-1]
	if userMsg.Role != "user" {
		t.Errorf("Role: want 'user', got %q", userMsg.Role)
	}

	if userMsg.Content != "What is Go?" {
		t.Errorf("Content: want 'What is Go?', got %q", userMsg.Content)
	}
}

func TestExtractSearchQuery(t *testing.T) {
	messages := []models.Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "What is the weather in Paris?"},
	}

	query := extractSearchQuery(messages)
	if query != "What is the weather in Paris?" {
		t.Errorf("extractSearchQuery: want 'What is the weather in Paris?', got %q", query)
	}
}

func TestExtractSearchQuery_EmptyMessages(t *testing.T) {
	query := extractSearchQuery(nil)
	if query != "" {
		t.Errorf("expected empty query for nil messages, got %q", query)
	}
}

func TestExtractSearchQuery_NoUserMessage(t *testing.T) {
	messages := []models.Message{
		{Role: "system", Content: "You are a helpful assistant."},
	}

	query := extractSearchQuery(messages)
	if query != "" {
		t.Errorf("expected empty query when no user message, got %q", query)
	}
}

func TestCallWebhook_Success(t *testing.T) {
	expectedResult := `{"result": "tool executed successfully"}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}

		if body["arguments"] != `{"key": "value"}` {
			http.Error(w, "unexpected arguments", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(expectedResult))
	}))
	defer server.Close()

	result := callWebhook(server.URL, `{"key": "value"}`)
	if result != expectedResult {
		t.Errorf("webhook result: want %q, got %q", expectedResult, result)
	}
}

func TestCallWebhook_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer server.Close()

	result := callWebhook(server.URL, `{}`)

	// Should return error JSON
	if result == "" {
		t.Error("expected non-empty result on error")
	}

	// Should contain "error"
	var errObj map[string]any
	if json.Unmarshal([]byte(result), &errObj) == nil {
		if _, ok := errObj["error"]; !ok {
			t.Error("expected 'error' key in result")
		}
	}
}

func TestCallWebhook_InvalidURL(t *testing.T) {
	result := callWebhook("http://localhost:99999/invalid", `{}`)

	if result == "" {
		t.Error("expected non-empty result on invalid URL")
	}

	// Should contain error info
	if result == `{}` {
		t.Error("expected error message, not empty JSON")
	}
}
