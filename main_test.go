// Integration tests for the LLM Gateway, following the GoFr integration test
// pattern: tests live in package main, call go main() directly in a goroutine,
// configure the server via t.Setenv, then exercise real HTTP endpoints.
//
// Pattern reference: gofr.dev/examples/http-server/main_test.go
//
// Infrastructure: PostgreSQL and Redis are started via docker compose in
// TestMain before any test runs. Each test function starts its own gateway
// instance on a free port by calling go main() with the appropriate env vars.
//
// Run:
//
//	go test -run TestIntegration -v -timeout 120s .
package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "github.com/lib/pq" // postgres driver for readiness check
	"gofr.dev/pkg/gofr/testutil"

	localtestutil "aryanmehrotra/litellm-go/testutil"
)

const integrationMasterKey = "sk-master-intg-2026"

// TestMain starts the required infrastructure (PostgreSQL + Redis) via
// docker compose before running any integration tests, and tears it down
// afterwards. This mirrors how GoFr's own examples are tested in CI.
func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")

	if _, err := exec.LookPath("docker"); err != nil {
		// No docker available — tests that need infra will be skipped inside
		// each test function via t.Skip. Run the suite so unit tests still pass.
		m.Run()
		return
	}

	composeFile := filepath.Join("docker", "docker-compose.yaml")

	// Start only the infrastructure containers (not the llm-gateway container).
	up := exec.Command("docker", "compose", "-f", composeFile, "up", "-d", "postgres", "redis")
	if out, err := up.CombinedOutput(); err != nil {
		fmt.Printf("docker compose up: %v\n%s\n", err, out)
	}

	// Wait until both services are fully ready before running tests.
	// For Redis a TCP check is sufficient; for PostgreSQL we also verify that
	// the server accepts database connections (it opens the TCP port before it
	// finishes initialization, which causes GoFr to FATAL-exit if we are too early).
	waitForTCPPort("localhost", 6379, 30*time.Second)
	waitForPostgres("localhost", 5432, 60*time.Second)

	code := m.Run()

	// Tear down containers and volumes after all tests complete.
	down := exec.Command("docker", "compose", "-f", composeFile, "down", "--volumes", "--remove-orphans")
	if out, err := down.CombinedOutput(); err != nil {
		fmt.Printf("docker compose down: %v\n%s\n", err, out)
	}

	os.Exit(code)
}

// TestIntegration_AgentSystem starts the real gateway server (via go main())
// pointed at a mock LLM HTTP server, then exercises every major agent and API
// flow with real HTTP requests — exactly as a human user would.
//
// Following the GoFr integration test pattern:
//   - testutil.GetFreePort allocates a free port
//   - t.Setenv configures the gateway before main() reads its config
//   - go main() starts the server in a goroutine
//   - waitForGateway polls /health until the server is accepting requests
func TestIntegration_AgentSystem(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not in PATH — skipping integration tests that need postgres+redis")
	}

	// ── mock LLM server ───────────────────────────────────────────────────
	// MockLLMServer is a real httptest.Server. Each subtest queues the
	// responses it expects, and the server returns them in order.
	llm := localtestutil.NewMockLLMServer(t)

	// ── port allocation (GoFr pattern) ───────────────────────────────────
	httpPort := testutil.GetFreePort(t)
	metricsPort := testutil.GetFreePort(t)

	// ── environment configuration via t.Setenv ───────────────────────────
	// t.Setenv overrides the value read from configs/.env at server start.
	t.Setenv("HTTP_PORT", strconv.Itoa(httpPort))
	t.Setenv("METRICS_PORT", strconv.Itoa(metricsPort))

	// Provider: only OpenAI, pointed at the mock LLM server.
	t.Setenv("OPENAI_API_KEY", "sk-mock-integration-test")
	t.Setenv("OPENAI_BASE_URL", llm.URL())
	// Ollama is also registered by main(); redirect it to the mock so that
	// ollamaProvider.RefreshModels() returns quickly instead of timing out.
	t.Setenv("OLLAMA_BASE_URL", llm.URL())
	t.Setenv("DEFAULT_PROVIDER", "openai")

	// Auth
	t.Setenv("GATEWAY_MASTER_KEY", integrationMasterKey)
	t.Setenv("GATEWAY_API_KEYS", "sk-intg-test-user")

	// Keep tests fast: no retries, short cache TTL, quiet logs.
	t.Setenv("RETRY_MAX", "0")
	t.Setenv("COOLDOWN_THRESHOLD", "9999")
	t.Setenv("CB_THRESHOLD", "9999")
	t.Setenv("CACHE_TTL_SECONDS", "1")
	t.Setenv("LOG_LEVEL", "WARN")
	t.Setenv("WEBSEARCH_ENABLED", "false")

	// ── start the real gateway in a goroutine (GoFr pattern) ─────────────
	go main()

	// Wait until the gateway's /health endpoint responds. Running 21 DB
	// migrations on a fresh postgres takes longer than GoFr's standard
	// 100 ms sleep, so we poll instead.
	healthURL := fmt.Sprintf("http://localhost:%d/health", httpPort)
	waitForGateway(t, healthURL, 45*time.Second)

	host := fmt.Sprintf("http://localhost:%d", httpPort)

	// do sends an authenticated request to the gateway and returns the response.
	do := func(t *testing.T, method, path string, body []byte) *http.Response {
		t.Helper()

		var bodyReader io.Reader
		if body != nil {
			bodyReader = bytes.NewReader(body)
		}

		req, err := http.NewRequest(method, host+path, bodyReader)
		require.NoError(t, err)

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+integrationMasterKey)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)

		return resp
	}

	// ── sub-tests ─────────────────────────────────────────────────────────

	t.Run("Health", func(t *testing.T) {
		resp := do(t, http.MethodGet, "/health", nil)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("ListModels", func(t *testing.T) {
		resp := do(t, http.MethodGet, "/v1/models", nil)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		assert.NotEmpty(t, result.Data, "expected at least one model")

		hasOpenAI := false
		for _, m := range result.Data {
			if strings.HasPrefix(m.ID, "openai/") {
				hasOpenAI = true
				break
			}
		}

		assert.True(t, hasOpenAI, "expected at least one openai/ model")
	})

	t.Run("ChatCompletion_Success", func(t *testing.T) {
		llm.QueueText("The capital of France is Paris.")

		body, _ := json.Marshal(map[string]any{
			"model": "openai/gpt-4o",
			"messages": []map[string]string{
				{"role": "user", "content": "What is the capital of France?"},
			},
		})
		resp := do(t, http.MethodPost, "/v1/chat/completions", body)
		defer resp.Body.Close()

		assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 300,
			"want 2xx, got %d", resp.StatusCode)

		var result struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		require.NotEmpty(t, result.Choices)
		assert.Equal(t, "The capital of France is Paris.", result.Choices[0].Message.Content)
	})

	t.Run("ChatCompletion_MissingModel", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"messages": []map[string]string{
				{"role": "user", "content": "hello"},
			},
		})
		resp := do(t, http.MethodPost, "/v1/chat/completions", body)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("AgentRun_SingleTurn", func(t *testing.T) {
		llm.QueueText("The answer is 42.")

		body, _ := json.Marshal(map[string]any{
			"model":          "openai/gpt-4o",
			"max_iterations": 3,
			"messages": []map[string]string{
				{"role": "user", "content": "What is the meaning of life?"},
			},
		})
		resp := do(t, http.MethodPost, "/v1/agents/run", body)
		defer resp.Body.Close()

		assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 300,
			"want 2xx, got %d", resp.StatusCode)

		var result struct {
			FinishReason string `json:"finish_reason"`
			FinalMessage struct {
				Content string `json:"content"`
			} `json:"final_message"`
			Steps []any `json:"steps"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

		assert.Equal(t, "stop", result.FinishReason)
		assert.Equal(t, "The answer is 42.", result.FinalMessage.Content)
		assert.Len(t, result.Steps, 1)
	})

	t.Run("AgentRun_ToolCall_ThenFinalAnswer", func(t *testing.T) {
		// The mock LLM webhook server simulates an external tool handler.
		var webhookCalled int
		webhookSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			webhookCalled++
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"temperature":"22°C","condition":"sunny","city":"Tokyo"}`)
		}))
		defer webhookSrv.Close()

		// Turn 1: LLM requests a tool call.
		llm.QueueToolCall("call-weather-1", "get_weather", `{"location":"Tokyo"}`)
		// Turn 2: LLM delivers the final answer after seeing the tool result.
		llm.QueueText("The weather in Tokyo is 22°C and sunny.")

		callsBefore := llm.RequestCount()

		body, _ := json.Marshal(map[string]any{
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
		resp := do(t, http.MethodPost, "/v1/agents/run", body)
		defer resp.Body.Close()

		assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 300,
			"want 2xx, got %d", resp.StatusCode)

		var result struct {
			FinishReason string `json:"finish_reason"`
			FinalMessage struct {
				Content string `json:"content"`
			} `json:"final_message"`
			Steps []struct {
				ToolCalls []any `json:"tool_calls"`
			} `json:"steps"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

		assert.Equal(t, "stop", result.FinishReason)
		assert.Equal(t, "The weather in Tokyo is 22°C and sunny.", result.FinalMessage.Content)

		// Verify the LLM was called twice (tool_call turn + final answer turn).
		assert.GreaterOrEqual(t, llm.RequestCount()-callsBefore, 2,
			"LLM should have been called at least twice (tool + final)")

		// Verify the webhook was invoked once.
		assert.Equal(t, 1, webhookCalled, "webhook should have been called once")

		// Verify the first step contains tool_calls.
		require.GreaterOrEqual(t, len(result.Steps), 2, "want at least 2 steps")
		assert.NotEmpty(t, result.Steps[0].ToolCalls, "step[0] should have tool_calls")
	})

	t.Run("ResponsesAPI", func(t *testing.T) {
		llm.QueueText("Go is a statically typed, compiled language designed at Google.")

		body, _ := json.Marshal(map[string]any{
			"model": "openai/gpt-4o",
			"input": "Tell me about the Go programming language.",
		})
		resp := do(t, http.MethodPost, "/v1/responses", body)
		defer resp.Body.Close()

		assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 300,
			"want 2xx, got %d", resp.StatusCode)

		var result struct {
			Object string `json:"object"`
			Status string `json:"status"`
			Output []struct {
				Type    string `json:"type"`
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"output"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

		assert.Equal(t, "response", result.Object)
		assert.Equal(t, "completed", result.Status)
		require.NotEmpty(t, result.Output, "output should be non-empty")

		// Find the message item and verify its text content.
		found := false
		for _, item := range result.Output {
			if item.Type == "message" {
				found = true
				require.NotEmpty(t, item.Content, "message content should be non-empty")
				assert.NotEmpty(t, item.Content[0].Text)
				break
			}
		}

		assert.True(t, found, "output should contain a message item")
	})
}

// waitForGateway polls the gateway health URL until it returns 200 or the
// timeout elapses. Running DB migrations on a fresh postgres takes several
// seconds, so a fixed sleep is not reliable.
func waitForGateway(t *testing.T, healthURL string, timeout time.Duration) {
	t.Helper()

	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resp, err := client.Get(healthURL)
		if err == nil {
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				return
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	t.Fatalf("gateway not ready at %s within %s", healthURL, timeout)
}

// waitForTCPPort polls until host:port accepts a TCP connection or timeout elapses.
func waitForTCPPort(host string, port int, timeout time.Duration) {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			conn.Close()
			return
		}

		time.Sleep(500 * time.Millisecond)
	}
}

// waitForPostgres polls until postgres accepts a real database connection.
// This is necessary because postgres opens its TCP port before it finishes
// initialization; a TCP-only check returns too early and causes GoFr to
// FATAL-exit when it tries to run migrations.
func waitForPostgres(host string, port int, timeout time.Duration) {
	dsn := fmt.Sprintf("host=%s port=%d user=postgres password=postgres dbname=llmgw sslmode=disable",
		host, port)
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		db, err := sql.Open("postgres", dsn)
		if err == nil {
			err = db.Ping()
			db.Close()

			if err == nil {
				return
			}
		}

		time.Sleep(500 * time.Millisecond)
	}
}
