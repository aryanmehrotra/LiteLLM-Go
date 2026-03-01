package testutil

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"aryanmehrotra/litellm-go/models"
)

// MockLLMServer is an httptest.Server that serves OpenAI-format chat completion
// responses from a pre-configured queue. It records all incoming requests so
// tests can assert on the requests sent to the "LLM".
//
// Usage:
//
//	srv := testutil.NewMockLLMServer(t)
//	srv.QueueText("Hello from mock LLM!")
//	// Point an OpenAI-compatible provider at srv.URL() and call it.
type MockLLMServer struct {
	t        *testing.T
	server   *httptest.Server
	mu       sync.Mutex
	queue    []mockQueueItem
	idx      int
	reqBodies []json.RawMessage // recorded raw request bodies
}

type mockQueueItem struct {
	statusCode int
	body       []byte
}

// NewMockLLMServer creates and starts a mock HTTP server that serves
// OpenAI-format responses. The server is automatically closed when the
// test finishes (via t.Cleanup).
func NewMockLLMServer(t *testing.T) *MockLLMServer {
	t.Helper()

	m := &MockLLMServer{t: t}
	m.server = httptest.NewServer(http.HandlerFunc(m.handle))
	t.Cleanup(m.server.Close)

	return m
}

func (m *MockLLMServer) handle(w http.ResponseWriter, r *http.Request) {
	// Non-POST requests (e.g. GET /api/tags from ollamaProvider.RefreshModels,
	// or circuit-breaker health probes) are acknowledged without consuming a
	// queued response so they don't interfere with the test's response sequence.
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Record the request body.
	var rawBody json.RawMessage

	if r.Body != nil {
		b, _ := io.ReadAll(r.Body)
		rawBody = b
	}

	m.mu.Lock()

	m.reqBodies = append(m.reqBodies, rawBody)

	if m.idx >= len(m.queue) {
		m.mu.Unlock()
		m.t.Logf("MockLLMServer: received unexpected request #%d (no more queued responses)", m.idx+1)
		http.Error(w, `{"error":{"message":"mock: no more queued responses","type":"server_error"}}`, http.StatusInternalServerError)

		return
	}

	item := m.queue[m.idx]
	m.idx++
	m.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(item.statusCode)
	_, _ = w.Write(item.body)
}

// URL returns the base URL of the mock server (e.g. "http://127.0.0.1:PORT").
func (m *MockLLMServer) URL() string { return m.server.URL }

// QueueResponse enqueues a ChatCompletionResponse to be returned for the next request.
func (m *MockLLMServer) QueueResponse(resp *models.ChatCompletionResponse) {
	b, err := json.Marshal(resp)
	if err != nil {
		m.t.Fatalf("MockLLMServer.QueueResponse: marshal error: %v", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.queue = append(m.queue, mockQueueItem{statusCode: http.StatusOK, body: b})
}

// QueueText enqueues a simple text ("stop") response.
func (m *MockLLMServer) QueueText(content string) {
	m.QueueResponse(TextResponse(content))
}

// QueueToolCall enqueues a tool_calls response requesting the given function.
func (m *MockLLMServer) QueueToolCall(callID, toolName, argsJSON string) {
	m.QueueResponse(ToolCallResponse(callID, toolName, argsJSON))
}

// QueueError enqueues an HTTP error response with the given status code and message.
func (m *MockLLMServer) QueueError(statusCode int, message string) {
	body := fmt.Sprintf(`{"error":{"message":%q,"type":"server_error"}}`, message)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.queue = append(m.queue, mockQueueItem{statusCode: statusCode, body: []byte(body)})
}

// RequestCount returns the total number of HTTP requests received so far.
func (m *MockLLMServer) RequestCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.reqBodies)
}

// RequestBodyAt returns the raw JSON body of the request at position i (0-indexed).
func (m *MockLLMServer) RequestBodyAt(i int) json.RawMessage {
	m.mu.Lock()
	defer m.mu.Unlock()

	if i >= len(m.reqBodies) {
		return nil
	}

	return m.reqBodies[i]
}

// LastRequestBody returns the body of the most recent request.
func (m *MockLLMServer) LastRequestBody() json.RawMessage {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.reqBodies) == 0 {
		return nil
	}

	return m.reqBodies[len(m.reqBodies)-1]
}

// LastRequest parses the most recent request body into a ChatCompletionRequest.
func (m *MockLLMServer) LastRequest() *models.ChatCompletionRequest {
	body := m.LastRequestBody()
	if body == nil {
		return nil
	}

	var req models.ChatCompletionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		m.t.Fatalf("MockLLMServer.LastRequest: unmarshal error: %v", err)
	}

	return &req
}
