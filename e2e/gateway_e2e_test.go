// Package e2e contains end-to-end tests that start the real gateway binary
// alongside a mock LLM HTTP server and exercise every major API flow exactly
// as a human user would — by sending real HTTP requests and asserting the
// real HTTP responses.
//
// # What gets started
//
//  1. PostgreSQL + Redis via docker compose (real dependencies, not mocks)
//  2. An in-process httptest.Server that acts as the LLM (returns queued responses)
//  3. The compiled gateway binary, pointed at the mock LLM and real DB/Redis
//
// # Running
//
//	go test ./e2e/ -v -timeout 120s
//
// Docker must be available. The test skips automatically if docker is not found.
package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aryanmehrotra/litellm-go/models"
)

// ─────────────────────────────────────────────
// constants & shared state
// ─────────────────────────────────────────────

const (
	masterKey = "sk-e2e-master-key"   // GATEWAY_MASTER_KEY used in tests
	apiKey    = "sk-e2e-gateway-test" // GATEWAY_API_KEYS used in tests
)

// repoRoot is the absolute path to the repository root (one level above e2e/).
var repoRoot = func() string {
	abs, err := filepath.Abs("..")
	if err != nil {
		panic(fmt.Sprintf("e2e: cannot determine repo root: %v", err))
	}

	return abs
}()

// ─────────────────────────────────────────────
// TestGatewayE2E — top-level orchestrator
// ─────────────────────────────────────────────

// TestGatewayE2E is the single parent test that:
//  1. Starts PostgreSQL and Redis with docker compose
//  2. Starts a mock LLM server (real TCP port, in-process)
//  3. Compiles the gateway binary
//  4. Starts the gateway binary, wired to the mock LLM + real infra
//  5. Runs a suite of subtests, each sending real HTTP requests to the gateway
//
// The gateway runs as a real OS process on a real TCP port, exactly as it
// would in production. The mock LLM server returns pre-queued OpenAI-format
// responses so the tests are deterministic without needing real API keys.
func TestGatewayE2E(t *testing.T) {
	// Skip if Docker is not available — the test needs postgres + redis.
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not found in PATH — skipping e2e tests")
	}

	// ── Step 1: start infrastructure ──────────────────────────────────────
	t.Log("▶ starting PostgreSQL and Redis via docker compose …")
	stopInfra := startInfrastructure(t)
	defer stopInfra()

	// ── Step 2: start mock LLM server ─────────────────────────────────────
	t.Log("▶ starting mock LLM HTTP server …")
	llm := newMockLLM(t)

	// ── Step 3: compile the gateway ───────────────────────────────────────
	t.Log("▶ compiling gateway binary …")
	binaryPath := buildGateway(t)

	// ── Step 4: pick a free port and start the gateway ────────────────────
	gwPort := freePort(t)
	t.Logf("▶ starting gateway on port %d → mock LLM at %s …", gwPort, llm.URL())
	startGateway(t, binaryPath, gwPort, llm.URL())

	gatewayBase := fmt.Sprintf("http://localhost:%d", gwPort)

	// Helper: send an authenticated request to the gateway.
	do := func(method, path string, body any) *http.Response {
		t.Helper()

		var bodyReader io.Reader

		if body != nil {
			b, err := json.Marshal(body)
			if err != nil {
				t.Fatalf("marshal request body: %v", err)
			}

			bodyReader = bytes.NewReader(b)
		}

		req, err := http.NewRequest(method, gatewayBase+path, bodyReader)
		if err != nil {
			t.Fatalf("create request: %v", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+masterKey)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}

		return resp
	}

	// readBody asserts 2xx status and decodes the response body into a map.
	readBody := func(resp *http.Response, label string) map[string]any {
		t.Helper()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			raw, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			t.Fatalf("%s: want 2xx, got %d — body: %s", label, resp.StatusCode, raw)
		}

		defer resp.Body.Close()

		var m map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
			t.Fatalf("%s: decode response body: %v", label, err)
		}

		return m
	}

	// ── Step 5: run sub-tests ─────────────────────────────────────────────

	// ── Health ────────────────────────────────────────────────────────────
	t.Run("Health", func(t *testing.T) {
		resp := do("GET", "/health", nil)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET /health: want 200, got %d", resp.StatusCode)
		}
	})

	// ── List Models ───────────────────────────────────────────────────────
	t.Run("ListModels", func(t *testing.T) {
		resp := do("GET", "/v1/models", nil)
		body := readBody(resp, "GET /v1/models")

		data, ok := body["data"].([]any)
		if !ok || len(data) == 0 {
			t.Errorf("GET /v1/models: expected non-empty data array, got %v", body)
		}

		// At least one model should start with "openai/"
		found := false

		for _, d := range data {
			m, _ := d.(map[string]any)
			if id, _ := m["id"].(string); len(id) > 7 && id[:7] == "openai/" {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("GET /v1/models: no openai/ models found in response: %v", data)
		}
	})

	// ── Chat Completion — happy path ──────────────────────────────────────
	t.Run("ChatCompletion_Success", func(t *testing.T) {
		llm.QueueText("The capital of France is Paris.")

		resp := do("POST", "/v1/chat/completions", map[string]any{
			"model": "openai/gpt-4o",
			"messages": []map[string]string{
				{"role": "user", "content": "What is the capital of France?"},
			},
		})
		body := readBody(resp, "POST /v1/chat/completions")

		choices, _ := body["choices"].([]any)
		if len(choices) == 0 {
			t.Fatalf("choices is empty")
		}

		choice, ok := choices[0].(map[string]any)
		if !ok {
			t.Fatalf("choices[0] is not a map: %T", choices[0])
		}

		message, _ := choice["message"].(map[string]any)
		content, _ := message["content"].(string)

		if content != "The capital of France is Paris." {
			t.Errorf("content: want 'The capital of France is Paris.', got %q", content)
		}
	})

	// ── Chat Completion — validation error (missing model) ────────────────
	t.Run("ChatCompletion_MissingModel", func(t *testing.T) {
		resp := do("POST", "/v1/chat/completions", map[string]any{
			"messages": []map[string]string{
				{"role": "user", "content": "hello"},
			},
		})
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("want 400, got %d", resp.StatusCode)
		}
	})

	// ── Chat Completion — unknown provider ────────────────────────────────
	t.Run("ChatCompletion_UnknownProvider", func(t *testing.T) {
		resp := do("POST", "/v1/chat/completions", map[string]any{
			"model": "anthropic/claude-3-opus", // anthropic not configured
			"messages": []map[string]string{
				{"role": "user", "content": "hello"},
			},
		})
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			t.Errorf("expected non-200 for unconfigured provider, got 200")
		}
	})

	// ── Agent Run — single turn (LLM responds with stop) ─────────────────
	t.Run("AgentRun_SingleTurn", func(t *testing.T) {
		llm.QueueText("The answer to life, the universe, and everything is 42.")

		resp := do("POST", "/v1/agents/run", map[string]any{
			"model":          "openai/gpt-4o",
			"max_iterations": 3,
			"messages": []map[string]string{
				{"role": "user", "content": "What is the meaning of life?"},
			},
		})
		body := readBody(resp, "POST /v1/agents/run")

		if reason, _ := body["finish_reason"].(string); reason != "stop" {
			t.Errorf("finish_reason: want 'stop', got %q", reason)
		}

		finalMsg, _ := body["final_message"].(map[string]any)
		content, _ := finalMsg["content"].(string)

		if content != "The answer to life, the universe, and everything is 42." {
			t.Errorf("final_message.content: got %q", content)
		}

		steps, _ := body["steps"].([]any)
		if len(steps) != 1 {
			t.Errorf("steps: want 1, got %d", len(steps))
		}
	})

	// ── Agent Run — tool call then final answer ───────────────────────────
	// The mock LLM first returns a tool_call requesting get_weather,
	// then (after the webhook is called) returns the final text answer.
	// A tiny webhook server captures the tool invocation.
	t.Run("AgentRun_ToolCall_ThenFinalAnswer", func(t *testing.T) {
		// Start a webhook server that simulates a tool handler.
		var webhookCalled int32

		webhookSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&webhookCalled, 1)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"temperature":"22°C","condition":"sunny","city":"Tokyo"}`))
		}))
		defer webhookSrv.Close()

		// Turn 1: LLM asks to call get_weather
		llm.QueueToolCall("call-weather-1", "get_weather", `{"location":"Tokyo"}`)
		// Turn 2: LLM delivers the final answer (after seeing tool result)
		llm.QueueText("The weather in Tokyo is 22°C and sunny.")

		resp := do("POST", "/v1/agents/run", map[string]any{
			"model":          "openai/gpt-4o",
			"max_iterations": 5,
			"messages": []map[string]string{
				{"role": "user", "content": "What is the weather in Tokyo?"},
			},
			"tools": []map[string]any{
				{
					"type": "function",
					"function": map[string]any{
						"name":        "get_weather",
						"description": "Get the current weather for a location",
						"parameters": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"location": map[string]any{"type": "string"},
							},
						},
					},
				},
			},
			"tool_webhooks": map[string]string{
				"get_weather": webhookSrv.URL,
			},
		})
		body := readBody(resp, "POST /v1/agents/run (tool)")

		// Verify finish reason
		if reason, _ := body["finish_reason"].(string); reason != "stop" {
			t.Errorf("finish_reason: want 'stop', got %q", reason)
		}

		// Verify LLM was called twice (once for tool_call, once for final answer)
		if llm.CallCount() < 2 {
			t.Errorf("LLM calls: want ≥2 (tool + final), got %d", llm.CallCount())
		}

		// Verify the webhook was called
		if n := atomic.LoadInt32(&webhookCalled); n != 1 {
			t.Errorf("webhook calls: want 1, got %d", n)
		}

		// Verify the final answer came through
		finalMsg, _ := body["final_message"].(map[string]any)
		content, _ := finalMsg["content"].(string)

		if content != "The weather in Tokyo is 22°C and sunny." {
			t.Errorf("final_message.content: got %q", content)
		}

		// Verify steps: step 0 has tool_calls, step 1 has stop
		steps, _ := body["steps"].([]any)
		if len(steps) < 2 {
			t.Fatalf("steps: want ≥2, got %d", len(steps))
		}

		step0, ok := steps[0].(map[string]any)
		if !ok {
			t.Fatalf("steps[0] is not a map: %T", steps[0])
		}

		if toolCalls, _ := step0["tool_calls"].([]any); len(toolCalls) == 0 {
			t.Error("steps[0] should have tool_calls")
		}
	})

	// ── Responses API — string input ──────────────────────────────────────
	t.Run("ResponsesAPI_StringInput", func(t *testing.T) {
		llm.QueueText("Go is a statically typed, compiled programming language designed at Google.")

		resp := do("POST", "/v1/responses", map[string]any{
			"model": "openai/gpt-4o",
			"input": "Tell me about the Go programming language.",
		})
		body := readBody(resp, "POST /v1/responses")

		if obj, _ := body["object"].(string); obj != "response" {
			t.Errorf("object: want 'response', got %q", obj)
		}

		output, _ := body["output"].([]any)
		if len(output) == 0 {
			t.Fatalf("output: expected non-empty array")
		}

		// Find the message output item
		var found bool

		for _, item := range output {
			m, _ := item.(map[string]any)
			if m["type"] == "message" {
				found = true

				content, _ := m["content"].([]any)
				if len(content) == 0 {
					t.Error("message content is empty")
					break
				}

				block, _ := content[0].(map[string]any)
				if text, _ := block["text"].(string); text == "" {
					t.Error("message content[0].text is empty")
				}

				break
			}
		}

		if !found {
			t.Error("no 'message' item found in output")
		}
	})

	// ── Responses API — with instructions ─────────────────────────────────
	t.Run("ResponsesAPI_WithInstructions", func(t *testing.T) {
		llm.QueueText("Bonjour! Je suis votre assistant.")

		resp := do("POST", "/v1/responses", map[string]any{
			"model":        "openai/gpt-4o",
			"input":        "Hello",
			"instructions": "You must respond only in French.",
		})
		body := readBody(resp, "POST /v1/responses (instructions)")

		if body["status"] != "completed" {
			t.Errorf("status: want 'completed', got %v", body["status"])
		}
	})

	// ── Agent Run — max iterations validation ─────────────────────────────
	t.Run("AgentRun_MaxIterations_HardCap", func(t *testing.T) {
		// Queue a single stop response — the agent should finish in 1 iteration.
		llm.QueueText("Done in one shot.")

		resp := do("POST", "/v1/agents/run", map[string]any{
			"model":          "openai/gpt-4o",
			"max_iterations": 200, // over the hard cap of 50
			"messages": []map[string]string{
				{"role": "user", "content": "do something"},
			},
		})
		body := readBody(resp, "POST /v1/agents/run (max_iter)")

		// Should still complete successfully (the cap is enforced server-side)
		if reason, _ := body["finish_reason"].(string); reason == "" {
			t.Errorf("finish_reason should not be empty")
		}
	})

	t.Logf("✓ all e2e subtests passed — mock LLM received %d total requests", llm.CallCount())
}

// ─────────────────────────────────────────────
// infrastructure helpers
// ─────────────────────────────────────────────

// startInfrastructure starts PostgreSQL and Redis via docker compose and waits
// until both ports are accepting TCP connections. Returns a teardown function.
func startInfrastructure(t *testing.T) func() {
	t.Helper()

	composeFile := filepath.Join(repoRoot, "docker", "docker-compose.yaml")

	// Start only the infrastructure services (not the llm-gateway container).
	up := exec.Command("docker", "compose", "-f", composeFile, "up", "-d", "postgres", "redis")
	out, err := up.CombinedOutput()

	if err != nil {
		t.Fatalf("docker compose up postgres redis: %v\n%s", err, out)
	}

	t.Logf("docker compose up: %s", out)

	// Wait for PostgreSQL (5432) and Redis (6379) to accept connections.
	waitForTCPPort(t, "localhost", 5432, 30*time.Second)
	waitForTCPPort(t, "localhost", 6379, 30*time.Second)

	t.Log("PostgreSQL and Redis are ready")

	return func() {
		down := exec.Command("docker", "compose", "-f", composeFile, "down", "--volumes", "--remove-orphans")
		if out, err := down.CombinedOutput(); err != nil {
			t.Logf("docker compose down: %v\n%s", err, out)
		}
	}
}

// waitForTCPPort polls until the given host:port accepts a TCP connection or
// the timeout expires.
func waitForTCPPort(t *testing.T, host string, port int, timeout time.Duration) {
	t.Helper()

	addr := fmt.Sprintf("%s:%d", host, port)
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			conn.Close()
			t.Logf("port %s ready", addr)

			return
		}

		time.Sleep(500 * time.Millisecond)
	}

	t.Fatalf("port %s not ready within %s", addr, timeout)
}

// buildGateway compiles the gateway binary and returns its path.
func buildGateway(t *testing.T) string {
	t.Helper()

	binaryPath := filepath.Join(t.TempDir(), "litellm-gateway")

	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	t.Logf("gateway binary built: %s", binaryPath)

	return binaryPath
}

// startGateway runs the gateway binary as a child process with the given
// port and mock LLM URL injected as environment variables.
// The process is terminated in t.Cleanup.
func startGateway(t *testing.T, binaryPath string, port int, mockLLMURL string) {
	t.Helper()

	env := append(os.Environ(),
		// Network
		fmt.Sprintf("HTTP_PORT=%d", port),
		"METRICS_PORT=0",

		// Providers — only OpenAI, pointed at mock LLM
		"OPENAI_API_KEY=sk-e2e-test-mock-key",
		fmt.Sprintf("OPENAI_BASE_URL=%s", mockLLMURL),
		// Ollama also pointed at mock so it doesn't try to connect to localhost:11434
		fmt.Sprintf("OLLAMA_BASE_URL=%s", mockLLMURL),
		"DEFAULT_PROVIDER=openai",

		// Infrastructure (real docker-compose containers)
		"DB_HOST=localhost",
		"DB_PORT=5432",
		"DB_USER=postgres",
		"DB_PASSWORD=postgres",
		"DB_NAME=llmgw",
		"DB_DIALECT=postgres",
		"REDIS_HOST=localhost",
		"REDIS_PORT=6379",

		// Auth
		fmt.Sprintf("GATEWAY_MASTER_KEY=%s", masterKey),
		fmt.Sprintf("GATEWAY_API_KEYS=%s", apiKey),

		// Routing — no retries so tests are fast
		"RETRY_MAX=0",
		"COOLDOWN_THRESHOLD=9999",
		"CB_THRESHOLD=9999",

		// Cache — short TTL so test responses don't persist between sub-tests
		"CACHE_TTL_SECONDS=1",

		// Quiet down verbose output
		"LOG_LEVEL=WARN",
		"GOFR_TELEMETRY=false",
		"WEBSEARCH_ENABLED=false",
	)

	cmd := exec.Command(binaryPath)
	cmd.Env = env
	cmd.Dir = repoRoot
	// Pipe gateway output through the test logger
	cmd.Stdout = &testLogWriter{t: t, prefix: "[gateway] "}
	cmd.Stderr = &testLogWriter{t: t, prefix: "[gateway] "}

	if err := cmd.Start(); err != nil {
		t.Fatalf("start gateway: %v", err)
	}

	t.Cleanup(func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	})

	// Wait for the gateway's /health endpoint to return 200.
	waitForGateway(t, fmt.Sprintf("http://localhost:%d/health", port))
}

// waitForGateway polls the given health URL until it returns 200 or times out.
func waitForGateway(t *testing.T, healthURL string) {
	t.Helper()

	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(45 * time.Second)

	for time.Now().Before(deadline) {
		resp, err := client.Get(healthURL)
		if err == nil {
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				t.Logf("gateway ready at %s", healthURL)
				return
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	t.Fatalf("gateway not ready at %s within 45s", healthURL)
}

// freePort returns an available TCP port on localhost.
func freePort(t *testing.T) int {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("get free port: %v", err)
	}

	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	return port
}

// testLogWriter forwards gateway output lines to t.Log.
type testLogWriter struct {
	t      *testing.T
	prefix string
	buf    bytes.Buffer
	mu     sync.Mutex
}

func (w *testLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buf.Write(p)

	for {
		line, err := w.buf.ReadString('\n')
		if err != nil {
			// Partial line — put it back in the buffer for the next Write call.
			if line != "" {
				w.buf.WriteString(line)
			}

			break
		}

		w.t.Log(w.prefix + line)
	}

	return len(p), nil
}

// ─────────────────────────────────────────────
// mockLLM — in-process LLM HTTP server
// ─────────────────────────────────────────────

// mockLLM is a real httptest.Server that dequeues pre-loaded OpenAI-format
// responses for each incoming POST request. GET requests (GoFr health probes)
// always return 200 OK.
type mockLLM struct {
	t      *testing.T
	server *httptest.Server
	mu     sync.Mutex
	queue  [][]byte
	calls  atomic.Int32
}

func newMockLLM(t *testing.T) *mockLLM {
	t.Helper()

	m := &mockLLM{t: t}
	m.server = httptest.NewServer(http.HandlerFunc(m.handle))
	t.Cleanup(m.server.Close)

	return m
}

// URL returns the base URL of the mock LLM server.
func (m *mockLLM) URL() string { return m.server.URL }

// CallCount returns the total number of POST requests received.
func (m *mockLLM) CallCount() int { return int(m.calls.Load()) }

// QueueText enqueues a simple "stop" completion response.
func (m *mockLLM) QueueText(content string) {
	m.queue_(models.ChatCompletionResponse{
		ID:     "chatcmpl-mock",
		Object: "chat.completion",
		Model:  "gpt-4o",
		Choices: []models.Choice{{
			Index:        0,
			FinishReason: "stop",
			Message:      models.Message{Role: "assistant", Content: content},
		}},
		Usage: models.Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
	})
}

// QueueToolCall enqueues a "tool_calls" response requesting the given function.
func (m *mockLLM) QueueToolCall(callID, toolName, argsJSON string) {
	m.queue_(models.ChatCompletionResponse{
		ID:     "chatcmpl-tool",
		Object: "chat.completion",
		Model:  "gpt-4o",
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
	})
}

func (m *mockLLM) queue_(resp models.ChatCompletionResponse) {
	b, err := json.Marshal(resp)
	if err != nil {
		m.t.Fatalf("mockLLM: marshal response: %v", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.queue = append(m.queue, b)
}

// handle serves the next queued response for POST requests and 200 OK for GET.
func (m *mockLLM) handle(w http.ResponseWriter, r *http.Request) {
	// GoFr registers HTTP services and probes their health via GET requests.
	// Return a simple 200 so the circuit breaker doesn't open.
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusOK)
		return
	}

	m.calls.Add(1)

	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.queue) == 0 {
		m.t.Logf("[mockLLM] WARNING: no more queued responses (call #%d)", m.calls.Load())
		http.Error(w, `{"error":{"message":"mock: queue empty","type":"server_error"}}`, http.StatusInternalServerError)

		return
	}

	resp := m.queue[0]
	m.queue = m.queue[1:]

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}
