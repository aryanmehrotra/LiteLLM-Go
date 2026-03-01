package testutil

import (
	"fmt"
	"sync"

	"gofr.dev/pkg/gofr"

	"aryanmehrotra/litellm-go/models"
)

// MockProvider is a test double for provider.Provider / routing.ChatProvider.
// It returns pre-configured responses in order without making any HTTP calls.
// Useful for testing handler logic independently of a real LLM.
type MockProvider struct {
	name      string
	mu        sync.Mutex
	responses []*models.ChatCompletionResponse
	errs      []error // parallel to responses; non-nil means return that error
	idx       int
	calls     []models.ChatCompletionRequest // recorded call history
}

// NewMockProvider creates a MockProvider that returns the given responses in
// sequence. Pass nil as a response to make that call return an error instead.
func NewMockProvider(name string, responses ...*models.ChatCompletionResponse) *MockProvider {
	return &MockProvider{name: name, responses: responses, errs: make([]error, len(responses))}
}

// NewMockProviderWithErrors creates a MockProvider where responses and errors
// are provided as paired slices. Either element may be nil.
func NewMockProviderWithErrors(name string, responses []*models.ChatCompletionResponse, errs []error) *MockProvider {
	return &MockProvider{name: name, responses: responses, errs: errs}
}

// Name implements provider.Provider.
func (m *MockProvider) Name() string { return m.name }

// Models implements provider.Provider.
func (m *MockProvider) Models() []string { return []string{m.name + "-model"} }

// ChatCompletion implements provider.Provider / routing.ChatProvider.
// It ignores ctx and returns the next queued response.
func (m *MockProvider) ChatCompletion(_ *gofr.Context, req models.ChatCompletionRequest) (*models.ChatCompletionResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.calls = append(m.calls, req)

	if m.idx >= len(m.responses) {
		return nil, fmt.Errorf("mock: no more queued responses (call #%d, only %d queued)", m.idx+1, len(m.responses))
	}

	resp := m.responses[m.idx]
	err := m.errs[m.idx]
	m.idx++

	return resp, err
}

// CallCount returns the number of times ChatCompletion has been called.
func (m *MockProvider) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.calls)
}

// CallAt returns the ChatCompletionRequest from call number i (0-indexed).
func (m *MockProvider) CallAt(i int) models.ChatCompletionRequest {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.calls[i]
}

// Calls returns a copy of all recorded requests.
func (m *MockProvider) Calls() []models.ChatCompletionRequest {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make([]models.ChatCompletionRequest, len(m.calls))
	copy(out, m.calls)

	return out
}

// ---------------------------------------------------------------------------
// Response builder helpers
// ---------------------------------------------------------------------------

// TextResponse builds a stop-finish ChatCompletionResponse with the given content.
func TextResponse(content string) *models.ChatCompletionResponse {
	return &models.ChatCompletionResponse{
		ID:     "chatcmpl-mock-text",
		Object: "chat.completion",
		Model:  "mock-model",
		Choices: []models.Choice{{
			Index:        0,
			FinishReason: "stop",
			Message:      models.Message{Role: "assistant", Content: content},
		}},
		Usage: models.Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
	}
}

// ToolCallResponse builds a tool_calls-finish ChatCompletionResponse that
// requests the LLM to call toolName with argsJSON.
func ToolCallResponse(callID, toolName, argsJSON string) *models.ChatCompletionResponse {
	return &models.ChatCompletionResponse{
		ID:     "chatcmpl-mock-tool",
		Object: "chat.completion",
		Model:  "mock-model",
		Choices: []models.Choice{{
			Index:        0,
			FinishReason: "tool_calls",
			Message: models.Message{
				Role: "assistant",
				ToolCalls: []models.ToolCall{{
					ID:   callID,
					Type: "function",
					Function: models.FunctionCall{
						Name:      toolName,
						Arguments: argsJSON,
					},
				}},
			},
		}},
		Usage: models.Usage{PromptTokens: 15, CompletionTokens: 10, TotalTokens: 25},
	}
}

// MultiToolCallResponse builds a response that requests multiple tool calls.
func MultiToolCallResponse(calls []models.ToolCall) *models.ChatCompletionResponse {
	return &models.ChatCompletionResponse{
		ID:     "chatcmpl-mock-multi-tool",
		Object: "chat.completion",
		Model:  "mock-model",
		Choices: []models.Choice{{
			Index:        0,
			FinishReason: "tool_calls",
			Message: models.Message{
				Role:      "assistant",
				ToolCalls: calls,
			},
		}},
		Usage: models.Usage{PromptTokens: 20, CompletionTokens: 15, TotalTokens: 35},
	}
}
