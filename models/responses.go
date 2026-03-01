package models

// ResponseInputContent is a single content item in the Responses API input.
type ResponseInputContent struct {
	Type     string `json:"type"`             // "input_text", "input_image", "input_file"
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	FileID   string `json:"file_id,omitempty"`
}

// ResponseInputItem is a single item in the Responses API input array.
type ResponseInputItem struct {
	Role    string                 `json:"role"`
	Content []ResponseInputContent `json:"content"`
}

// ResponseBuiltinTool defines a built-in gateway tool (web_search, file_search, code_interpreter).
type ResponseBuiltinTool struct {
	Type string `json:"type"` // "web_search_preview", "file_search", "code_interpreter"

	// web_search_preview options
	SearchContextSize string `json:"search_context_size,omitempty"` // "low", "medium", "high"

	// file_search options
	VectorStoreIDs []string `json:"vector_store_ids,omitempty"`

	// function tool options (when Type=="function")
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

// ResponseRequest is the request body for POST /v1/responses (OpenAI Responses API).
type ResponseRequest struct {
	Model         string              `json:"model"`
	Input         any                 `json:"input"` // string or []ResponseInputItem
	Instructions  string              `json:"instructions,omitempty"`
	Tools         []ResponseBuiltinTool `json:"tools,omitempty"`
	Temperature   *float64            `json:"temperature,omitempty"`
	MaxOutputTokens *int              `json:"max_output_tokens,omitempty"`
	Stream        bool                `json:"stream,omitempty"`
	Metadata      map[string]string   `json:"metadata,omitempty"`
	// Previous response ID for multi-turn conversations
	PreviousResponseID string `json:"previous_response_id,omitempty"`
}

// ResponseOutputItem is a single output item in a Response.
type ResponseOutputItem struct {
	Type    string `json:"type"` // "message", "web_search_call", "function_call", "reasoning"
	ID      string `json:"id"`
	Status  string `json:"status,omitempty"` // "completed", "in_progress", "incomplete"

	// For type == "message"
	Role    string                  `json:"role,omitempty"`
	Content []ResponseOutputContent `json:"content,omitempty"`

	// For type == "web_search_call"
	Name   string `json:"name,omitempty"`
	Action any    `json:"action,omitempty"`

	// For type == "function_call"
	CallID    string `json:"call_id,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// ResponseOutputContent is a content item in a Response output message.
type ResponseOutputContent struct {
	Type        string            `json:"type"` // "output_text", "refusal"
	Text        string            `json:"text,omitempty"`
	Annotations []ResponseAnnotation `json:"annotations,omitempty"`
}

// ResponseAnnotation is a citation or URL annotation in the Responses API.
type ResponseAnnotation struct {
	Type      string `json:"type"` // "url_citation"
	StartIndex int   `json:"start_index"`
	EndIndex   int   `json:"end_index"`
	URL       string `json:"url,omitempty"`
	Title     string `json:"title,omitempty"`
}

// ResponseUsage contains token usage details for a Response.
type ResponseUsage struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	TotalTokens         int `json:"total_tokens"`
	InputTokensDetails  *ResponseTokenDetails `json:"input_tokens_details,omitempty"`
	OutputTokensDetails *ResponseTokenDetails `json:"output_tokens_details,omitempty"`
}

// ResponseTokenDetails breaks down token counts for the Responses API.
type ResponseTokenDetails struct {
	CachedTokens   int `json:"cached_tokens,omitempty"`
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`
}

// ResponseObject is the top-level response for POST /v1/responses.
type ResponseObject struct {
	ID            string               `json:"id"`
	Object        string               `json:"object"` // "response"
	CreatedAt     int64                `json:"created_at"`
	Model         string               `json:"model"`
	Status        string               `json:"status"` // "completed", "failed", "in_progress", "incomplete"
	Output        []ResponseOutputItem `json:"output"`
	Usage         ResponseUsage        `json:"usage"`
	Instructions  string               `json:"instructions,omitempty"`
	Error         *ResponseError       `json:"error,omitempty"`
	Metadata      map[string]string    `json:"metadata,omitempty"`
}

// ResponseError holds error info for a failed Response.
type ResponseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
