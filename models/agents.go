package models

// AgentRunRequest is the request body for POST /v1/agents/run.
// It runs a multi-turn LLM completion loop, automatically executing
// built-in tools (web_search) until the model stops calling tools.
type AgentRunRequest struct {
	Model         string    `json:"model"`
	Messages      []Message `json:"messages"`
	Tools         []Tool    `json:"tools,omitempty"`
	MaxIterations int       `json:"max_iterations,omitempty"` // default 10
	Temperature   *float64  `json:"temperature,omitempty"`
	TopP          *float64  `json:"top_p,omitempty"`
	MaxTokens     *int      `json:"max_tokens,omitempty"`

	// ToolWebhooks maps tool names to HTTP URLs that the gateway will POST
	// to when the model calls that tool, passing {"arguments": "..."} and
	// expecting {"result": "..."} in response.
	ToolWebhooks map[string]string `json:"tool_webhooks,omitempty"`
}

// AgentStep records one step in the agent execution loop.
type AgentStep struct {
	Iteration    int       `json:"iteration"`
	Messages     []Message `json:"messages"`               // messages sent in this step
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`  // tool calls the model made
	ToolResults  []Message  `json:"tool_results,omitempty"` // tool result messages added
	FinishReason string    `json:"finish_reason"`
}

// AgentRunResponse is the response for POST /v1/agents/run.
type AgentRunResponse struct {
	ID           string                  `json:"id"`
	Object       string                  `json:"object"` // "agent.run"
	Model        string                  `json:"model"`
	Steps        []AgentStep             `json:"steps"`
	FinalMessage Message                 `json:"final_message"`
	Usage        Usage                   `json:"usage"`
	FinishReason string                  `json:"finish_reason"`
	Iterations   int                     `json:"iterations"`
}
