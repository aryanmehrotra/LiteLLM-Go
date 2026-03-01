package models

// Message represents a single message in a chat conversation.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ChatCompletionRequest is the OpenAI-compatible request format used as the
// universal input for all providers.
type ChatCompletionRequest struct {
	Model            string    `json:"model"`
	Messages         []Message `json:"messages"`
	Temperature      *float64  `json:"temperature,omitempty"`
	TopP             *float64  `json:"top_p,omitempty"`
	MaxTokens        *int      `json:"max_tokens,omitempty"`
	Stream           bool      `json:"stream,omitempty"`
	Stop             []string  `json:"stop,omitempty"`
	PresencePenalty  *float64  `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64  `json:"frequency_penalty,omitempty"`
	Tools            []Tool    `json:"tools,omitempty"`
	ToolChoice       any       `json:"tool_choice,omitempty"`

	// WebSearchOptions enables gateway-level web search augmentation.
	// When set, the gateway performs a web search and injects results as context.
	WebSearchOptions *WebSearchOptions `json:"web_search_options,omitempty"`
}

// ChatCompletionResponse is the OpenAI-compatible response format returned by
// all providers through the gateway.
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`

	Provider    string         `json:"provider,omitempty"`
	Cached      bool           `json:"cached,omitempty"`
	Cost        float64        `json:"cost,omitempty"`
	RateLimit   *RateLimitInfo `json:"rate_limit,omitempty"`
	Annotations []Annotation   `json:"annotations,omitempty"`
}

// RateLimitInfo contains rate limit details included in API responses.
type RateLimitInfo struct {
	Limit     int   `json:"limit"`
	Remaining int   `json:"remaining"`
	ResetAt   int64 `json:"reset_at"`
}

// Choice represents one completion choice in the response.
type Choice struct {
	Index        int        `json:"index"`
	Message      Message    `json:"message"`
	FinishReason string     `json:"finish_reason"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
}

// Usage reports token consumption for a request.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ModelInfo describes a model available through the gateway.
type ModelInfo struct {
	ID       string `json:"id"`
	Object   string `json:"object"`
	OwnedBy  string `json:"owned_by"`
	Provider string `json:"provider"`
	Type     string `json:"type,omitempty"` // "chat" or "embedding"
}

// ModelListResponse is the response format for GET /v1/models.
type ModelListResponse struct {
	Object string      `json:"object"`
	Data   []ModelInfo `json:"data"`
}

// --- Streaming types (chat.completion.chunk format) ---

// StreamChunk is a single streaming chunk in the OpenAI chat.completion.chunk format.
type StreamChunk struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []StreamChoice `json:"choices"`
	Usage   *Usage         `json:"usage,omitempty"`
}

// StreamChoice represents one choice within a streaming chunk.
type StreamChoice struct {
	Index        int          `json:"index"`
	Delta        StreamDelta  `json:"delta"`
	FinishReason *string      `json:"finish_reason"`
}

// StreamDelta contains the incremental content in a streaming chunk.
type StreamDelta struct {
	Role      string           `json:"role,omitempty"`
	Content   string           `json:"content,omitempty"`
	ToolCalls []StreamToolCall `json:"tool_calls,omitempty"`
}

// StreamToolCall represents a tool call within a streaming delta.
type StreamToolCall struct {
	Index    int          `json:"index"`
	ID       string       `json:"id,omitempty"`
	Type     string       `json:"type,omitempty"`
	Function FunctionCall `json:"function"`
}

// --- Function calling / tool use types ---

// Tool describes a tool available to the model.
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction describes a function tool's schema.
type ToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

// ToolCall represents a tool invocation in a model response.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall contains the function name and arguments string.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// --- Web search types ---

// WebSearchOptions controls gateway-level web search augmentation.
type WebSearchOptions struct {
	SearchContextSize string        `json:"search_context_size,omitempty"` // "low", "medium", "high"
	UserLocation      *UserLocation `json:"user_location,omitempty"`
}

// UserLocation holds geographic context for search regionalization.
type UserLocation struct {
	Country string `json:"country,omitempty"`
}

// Annotation represents a citation in the response (OpenAI-compatible).
type Annotation struct {
	Type        string       `json:"type"`
	URLCitation *URLCitation `json:"url_citation,omitempty"`
}

// URLCitation contains the URL and title of a cited source.
type URLCitation struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}
