package models

// Assistant represents an AI assistant (OpenAI-compatible Assistants API).
type Assistant struct {
	ID           string            `json:"id"`
	Object       string            `json:"object"` // "assistant"
	CreatedAt    int64             `json:"created_at"`
	Name         string            `json:"name,omitempty"`
	Description  string            `json:"description,omitempty"`
	Model        string            `json:"model"`
	Instructions string            `json:"instructions,omitempty"`
	Tools        []AssistantTool   `json:"tools,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// AssistantTool describes a tool available to an assistant.
type AssistantTool struct {
	Type     string       `json:"type"` // "code_interpreter", "file_search", "function"
	Function *ToolFunction `json:"function,omitempty"`
}

// AssistantRequest is the request body for POST /v1/assistants.
type AssistantRequest struct {
	Model        string            `json:"model"`
	Name         string            `json:"name,omitempty"`
	Description  string            `json:"description,omitempty"`
	Instructions string            `json:"instructions,omitempty"`
	Tools        []AssistantTool   `json:"tools,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// AssistantListResponse is the response for GET /v1/assistants.
type AssistantListResponse struct {
	Object  string      `json:"object"` // "list"
	Data    []Assistant `json:"data"`
	HasMore bool        `json:"has_more"`
}

// Thread represents a conversation thread (OpenAI-compatible Assistants API).
type Thread struct {
	ID        string            `json:"id"`
	Object    string            `json:"object"` // "thread"
	CreatedAt int64             `json:"created_at"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// ThreadRequest is the request body for POST /v1/threads.
type ThreadRequest struct {
	Messages []ThreadMessage   `json:"messages,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ThreadMessage is an initial message for creating a thread.
type ThreadMessage struct {
	Role    string `json:"role"` // "user" or "assistant"
	Content string `json:"content"`
}

// ThreadMessageObject is a message stored in a thread.
type ThreadMessageObject struct {
	ID          string            `json:"id"`
	Object      string            `json:"object"` // "thread.message"
	CreatedAt   int64             `json:"created_at"`
	ThreadID    string            `json:"thread_id"`
	Role        string            `json:"role"`
	Content     []MessageContent  `json:"content"`
	AssistantID string            `json:"assistant_id,omitempty"`
	RunID       string            `json:"run_id,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// MessageContent is a content block within a thread message.
type MessageContent struct {
	Type string       `json:"type"` // "text"
	Text *TextContent `json:"text,omitempty"`
}

// TextContent holds the text value of a message content block.
type TextContent struct {
	Value       string `json:"value"`
	Annotations []any  `json:"annotations"`
}

// ThreadMessageListResponse is the response for GET /v1/threads/{id}/messages.
type ThreadMessageListResponse struct {
	Object  string                `json:"object"` // "list"
	Data    []ThreadMessageObject `json:"data"`
	HasMore bool                  `json:"has_more"`
}

// Run represents an assistant run on a thread.
type Run struct {
	ID           string            `json:"id"`
	Object       string            `json:"object"` // "thread.run"
	CreatedAt    int64             `json:"created_at"`
	ThreadID     string            `json:"thread_id"`
	AssistantID  string            `json:"assistant_id"`
	Status       string            `json:"status"` // "queued","in_progress","completed","failed","cancelled","expired"
	Model        string            `json:"model"`
	Instructions string            `json:"instructions,omitempty"`
	Tools        []AssistantTool   `json:"tools,omitempty"`
	StartedAt    *int64            `json:"started_at,omitempty"`
	CompletedAt  *int64            `json:"completed_at,omitempty"`
	FailedAt     *int64            `json:"failed_at,omitempty"`
	Usage        *Usage            `json:"usage,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// RunRequest is the request body for POST /v1/threads/{id}/runs.
type RunRequest struct {
	AssistantID  string            `json:"assistant_id"`
	Model        string            `json:"model,omitempty"`
	Instructions string            `json:"instructions,omitempty"`
	Tools        []AssistantTool   `json:"tools,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// RunListResponse is the response for GET /v1/threads/{id}/runs.
type RunListResponse struct {
	Object  string `json:"object"` // "list"
	Data    []Run  `json:"data"`
	HasMore bool   `json:"has_more"`
}
