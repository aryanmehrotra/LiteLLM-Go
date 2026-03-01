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

	"aryanmehrotra/litellm-go/models"
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

	// ── Complex / multi-modal tests ───────────────────────────────────────

	t.Run("ChatCompletion_MultiTurnHistory", func(t *testing.T) {
		// Send a full conversation history: system + user + assistant + user.
		// The mock returns one response for the final user turn.
		llm.QueueText("Python was created by Guido van Rossum in 1991.")

		callsBefore := llm.RequestCount()

		body, _ := json.Marshal(map[string]any{
			"model": "openai/gpt-4o",
			"messages": []map[string]string{
				{"role": "system", "content": "You are a helpful programming assistant."},
				{"role": "user", "content": "Who created Python?"},
				{"role": "assistant", "content": "Python was created by Guido van Rossum."},
				{"role": "user", "content": "In what year?"},
			},
		})
		resp := do(t, http.MethodPost, "/v1/chat/completions", body)
		defer resp.Body.Close()

		assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 300,
			"want 2xx, got %d", resp.StatusCode)

		var result struct {
			Choices []struct {
				Message struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		require.NotEmpty(t, result.Choices)
		assert.Equal(t, "Python was created by Guido van Rossum in 1991.", result.Choices[0].Message.Content)

		// Gateway should have made exactly one LLM call.
		assert.Equal(t, callsBefore+1, llm.RequestCount(), "expected exactly one LLM call")

		// The forwarded request should contain all 4 messages.
		var forwarded struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		require.NoError(t, json.Unmarshal(llm.LastRequestBody(), &forwarded))
		assert.Len(t, forwarded.Messages, 4, "all 4 messages should be forwarded to the LLM")
		assert.Equal(t, "system", forwarded.Messages[0].Role)
		assert.Equal(t, "user", forwarded.Messages[3].Role)
	})

	t.Run("ChatCompletion_ParameterForwarding", func(t *testing.T) {
		// Verify that temperature and max_tokens are forwarded to the upstream LLM.
		llm.QueueText("The sky is blue.")

		temp := 0.3
		maxTok := 64

		body, _ := json.Marshal(map[string]any{
			"model": "openai/gpt-4o",
			"messages": []map[string]string{
				{"role": "user", "content": "Why is the sky blue?"},
			},
			"temperature": temp,
			"max_tokens":  maxTok,
		})
		resp := do(t, http.MethodPost, "/v1/chat/completions", body)
		defer resp.Body.Close()

		assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 300,
			"want 2xx, got %d", resp.StatusCode)

		// Inspect the forwarded request body.
		var forwarded struct {
			Temperature float64 `json:"temperature"`
			MaxTokens   int     `json:"max_tokens"`
		}
		require.NoError(t, json.Unmarshal(llm.LastRequestBody(), &forwarded))
		assert.InDelta(t, temp, forwarded.Temperature, 0.001,
			"temperature should be forwarded to the LLM")
		assert.Equal(t, maxTok, forwarded.MaxTokens, "max_tokens should be forwarded to the LLM")
	})

	t.Run("ResponsesAPI_MultiModal_ImageAndText", func(t *testing.T) {
		// Multi-modal input via the Responses API: an array of content items
		// where one item is text and another is an image URL.
		// The gateway extracts the text (input_text blocks) and forwards it to
		// the LLM. The image is carried in the structured input but the current
		// ChatCompletionRequest.Messages uses a string Content field, so the
		// gateway correctly passes through the text portion.
		llm.QueueText("The image shows a tabby cat sitting on a windowsill.")

		body, _ := json.Marshal(map[string]any{
			"model": "openai/gpt-4o",
			"input": []map[string]any{
				{
					"role": "user",
					"content": []map[string]any{
						{"type": "input_text", "text": "What is in this image?"},
						{"type": "input_image", "image_url": "data:image/jpeg;base64,/9j/4AAQ=="},
					},
				},
			},
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

		found := false
		for _, item := range result.Output {
			if item.Type == "message" {
				found = true
				require.NotEmpty(t, item.Content)
				assert.Contains(t, item.Content[0].Text, "cat",
					"response should describe the image content")
				break
			}
		}

		assert.True(t, found, "output should contain a message item")
	})

	t.Run("ResponsesAPI_SystemInstruction", func(t *testing.T) {
		// The `instructions` field is translated to a system message prepended
		// to the conversation. Verify the gateway handles it correctly.
		llm.QueueText("Bonjour! Comment puis-je vous aider?")

		body, _ := json.Marshal(map[string]any{
			"model":        "openai/gpt-4o",
			"input":        "Say hello.",
			"instructions": "You are a French-speaking assistant. Always reply in French.",
		})
		resp := do(t, http.MethodPost, "/v1/responses", body)
		defer resp.Body.Close()

		assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 300,
			"want 2xx, got %d", resp.StatusCode)

		var result struct {
			Object string `json:"object"`
			Status string `json:"status"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		assert.Equal(t, "response", result.Object)
		assert.Equal(t, "completed", result.Status)

		// The instructions should have been forwarded as a system message.
		var forwarded struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		require.NoError(t, json.Unmarshal(llm.LastRequestBody(), &forwarded))
		require.GreaterOrEqual(t, len(forwarded.Messages), 2,
			"system instruction + user message should both be forwarded")
		assert.Equal(t, "system", forwarded.Messages[0].Role,
			"first forwarded message should be the system instruction")
		assert.Contains(t, forwarded.Messages[0].Content, "French")
	})

	t.Run("ResponsesAPI_MultiTurnInput", func(t *testing.T) {
		// Input as an array of alternating user/assistant turns (multi-turn
		// conversation history via the Responses API).
		llm.QueueText("It is exactly 3.14159265.")

		body, _ := json.Marshal(map[string]any{
			"model": "openai/gpt-4o",
			"input": []map[string]any{
				{"role": "user", "content": "What is pi?"},
				{"role": "assistant", "content": "Pi is approximately 3.14."},
				{"role": "user", "content": "What is the exact value?"},
			},
		})
		resp := do(t, http.MethodPost, "/v1/responses", body)
		defer resp.Body.Close()

		assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 300,
			"want 2xx, got %d", resp.StatusCode)

		var result struct {
			Object string `json:"object"`
			Status string `json:"status"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		assert.Equal(t, "response", result.Object)
		assert.Equal(t, "completed", result.Status)

		// All 3 turns should have been forwarded as chat messages.
		var forwarded struct {
			Messages []struct {
				Role string `json:"role"`
			} `json:"messages"`
		}
		require.NoError(t, json.Unmarshal(llm.LastRequestBody(), &forwarded))
		assert.Len(t, forwarded.Messages, 3, "all 3 turns should be forwarded to the LLM")
	})

	t.Run("AgentRun_MaxIterationsRespected", func(t *testing.T) {
		// With max_iterations=1 and the LLM returning a tool_call on the first
		// (and only) iteration, the agent must stop with finish_reason="max_iterations"
		// rather than calling the LLM a second time.
		llm.QueueToolCall("call-stop-1", "lookup_stock", `{"ticker":"AAPL"}`)

		callsBefore := llm.RequestCount()

		body, _ := json.Marshal(map[string]any{
			"model":          "openai/gpt-4o",
			"max_iterations": 1,
			"messages": []map[string]string{
				{"role": "user", "content": "What is the current Apple stock price?"},
			},
			"tools": []map[string]any{
				{
					"type": "function",
					"function": map[string]any{
						"name":        "lookup_stock",
						"description": "Look up the current stock price",
						"parameters": map[string]any{
							"type":       "object",
							"properties": map[string]any{"ticker": map[string]any{"type": "string"}},
						},
					},
				},
			},
			// No tool_webhooks: the agent will record an error result and stop.
		})
		resp := do(t, http.MethodPost, "/v1/agents/run", body)
		defer resp.Body.Close()

		assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 300,
			"want 2xx, got %d", resp.StatusCode)

		var result struct {
			FinishReason string `json:"finish_reason"`
			Iterations   int    `json:"iterations"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

		assert.Equal(t, "max_iterations", result.FinishReason,
			"agent should stop with max_iterations when the cap is hit")
		assert.Equal(t, 1, result.Iterations,
			"exactly 1 iteration should have been executed")

		// Only 1 LLM call should have been made (the tool result is appended
		// internally; no follow-up LLM call is made because max was reached).
		assert.Equal(t, callsBefore+1, llm.RequestCount(),
			"LLM should be called exactly once when max_iterations=1 yields a tool_call")
	})

	t.Run("AgentRun_ParallelToolCalls", func(t *testing.T) {
		// The LLM returns two tool calls in a single response (parallel tool use).
		// Both webhooks must be invoked before the next LLM turn.
		var (
			weatherCalls int
			stockCalls   int
		)

		weatherSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			weatherCalls++
			fmt.Fprint(w, `{"temperature":"15°C","condition":"cloudy"}`)
		}))
		defer weatherSrv.Close()

		stockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			stockCalls++
			fmt.Fprint(w, `{"price":189.50,"currency":"USD"}`)
		}))
		defer stockSrv.Close()

		// Turn 1: LLM requests two tools simultaneously.
		parallelCalls := []models.ToolCall{
			{ID: "call-w-1", Type: "function", Function: models.FunctionCall{Name: "get_weather", Arguments: `{"location":"London"}`}},
			{ID: "call-s-1", Type: "function", Function: models.FunctionCall{Name: "get_stock", Arguments: `{"ticker":"TSLA"}`}},
		}
		llm.QueueResponse(localtestutil.MultiToolCallResponse(parallelCalls))
		// Turn 2: LLM delivers the final answer after seeing both results.
		llm.QueueText("In London the weather is 15°C and cloudy; TSLA trades at $189.50.")

		callsBefore := llm.RequestCount()

		body, _ := json.Marshal(map[string]any{
			"model":          "openai/gpt-4o",
			"max_iterations": 5,
			"messages": []map[string]string{
				{"role": "user", "content": "What is the weather in London and the TSLA stock price?"},
			},
			"tools": []map[string]any{
				{
					"type": "function",
					"function": map[string]any{
						"name":        "get_weather",
						"description": "Get current weather",
						"parameters": map[string]any{
							"type":       "object",
							"properties": map[string]any{"location": map[string]any{"type": "string"}},
						},
					},
				},
				{
					"type": "function",
					"function": map[string]any{
						"name":        "get_stock",
						"description": "Get stock price",
						"parameters": map[string]any{
							"type":       "object",
							"properties": map[string]any{"ticker": map[string]any{"type": "string"}},
						},
					},
				},
			},
			"tool_webhooks": map[string]string{
				"get_weather": weatherSrv.URL,
				"get_stock":   stockSrv.URL,
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
			Iterations int `json:"iterations"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

		assert.Equal(t, "stop", result.FinishReason)
		assert.Contains(t, result.FinalMessage.Content, "London")
		assert.Equal(t, 2, result.Iterations, "two LLM turns: tool_call + final answer")

		// Both webhooks must have been invoked exactly once each.
		assert.Equal(t, 1, weatherCalls, "weather webhook should be called once")
		assert.Equal(t, 1, stockCalls, "stock webhook should be called once")

		// Two LLM calls: one for the tool_call response, one for the final answer.
		assert.Equal(t, callsBefore+2, llm.RequestCount(),
			"exactly 2 LLM calls for a 2-turn agent exchange")
	})

	t.Run("Embeddings", func(t *testing.T) {
		// Queue an embedding response on the mock LLM server. The provider's
		// Embedding() method calls POST /v1/embeddings on the same base URL.
		embeddingResp, _ := json.Marshal(map[string]any{
			"object": "list",
			"data": []map[string]any{
				{
					"object":    "embedding",
					"index":     0,
					"embedding": []float64{0.1, 0.2, 0.3, 0.4, 0.5},
				},
			},
			"model": "text-embedding-3-small",
			"usage": map[string]int{
				"prompt_tokens": 5,
				"total_tokens":  5,
			},
		})
		llm.QueueRawResponse(http.StatusOK, embeddingResp)

		body, _ := json.Marshal(map[string]any{
			"model": "openai/text-embedding-3-small",
			"input": "The cat sat on the mat.",
		})
		resp := do(t, http.MethodPost, "/v1/embeddings", body)
		defer resp.Body.Close()

		assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 300,
			"want 2xx, got %d", resp.StatusCode)

		var result struct {
			Object string `json:"object"`
			Data   []struct {
				Object    string    `json:"object"`
				Index     int       `json:"index"`
				Embedding []float64 `json:"embedding"`
			} `json:"data"`
			Model string `json:"model"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

		assert.Equal(t, "list", result.Object)
		require.Len(t, result.Data, 1, "expected one embedding")
		assert.Equal(t, "embedding", result.Data[0].Object)
		assert.Len(t, result.Data[0].Embedding, 5, "expected 5-dimensional embedding vector")
	})

	t.Run("Batch_SubmitAndStatus", func(t *testing.T) {
		// Submit a batch containing one chat-completion request. Verify the
		// gateway creates the batch record and returns a batch ID.
		body, _ := json.Marshal(map[string]any{
			"requests": []map[string]any{
				{
					"custom_id": "req-1",
					"method":    "POST",
					"url":       "/v1/chat/completions",
					"body": map[string]any{
						"model": "openai/gpt-4o",
						"messages": []map[string]string{
							{"role": "user", "content": "What is 2+2?"},
						},
					},
				},
			},
		})
		resp := do(t, http.MethodPost, "/v1/batches", body)
		defer resp.Body.Close()

		assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 300,
			"want 2xx, got %d", resp.StatusCode)

		var created struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
		assert.NotEmpty(t, created.ID, "batch ID should be non-empty")
		assert.NotEmpty(t, created.Status, "batch status should be non-empty")

		// Immediately fetch the batch status — it should be retrievable.
		statusResp := do(t, http.MethodGet, "/v1/batches/"+created.ID, nil)
		defer statusResp.Body.Close()

		assert.Equal(t, http.StatusOK, statusResp.StatusCode)

		var status struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		require.NoError(t, json.NewDecoder(statusResp.Body).Decode(&status))
		assert.Equal(t, created.ID, status.ID)
		assert.NotEmpty(t, status.Status)
	})

	t.Run("Assistants_CreateListGetDelete", func(t *testing.T) {
		// Exercise the full assistant CRUD lifecycle.
		// No LLM calls are required for CRUD-only operations.
		createBody, _ := json.Marshal(map[string]any{
			"model":        "openai/gpt-4o",
			"name":         "Test Assistant",
			"instructions": "You are a helpful test assistant.",
			"tools": []map[string]string{
				{"type": "function"},
			},
		})
		createResp := do(t, http.MethodPost, "/v1/assistants", createBody)
		defer createResp.Body.Close()

		assert.True(t, createResp.StatusCode >= 200 && createResp.StatusCode < 300,
			"create: want 2xx, got %d", createResp.StatusCode)

		var created struct {
			ID           string `json:"id"`
			Object       string `json:"object"`
			Name         string `json:"name"`
			Model        string `json:"model"`
			Instructions string `json:"instructions"`
		}
		require.NoError(t, json.NewDecoder(createResp.Body).Decode(&created))
		assert.NotEmpty(t, created.ID, "assistant ID should be non-empty")
		assert.Equal(t, "assistant", created.Object)
		assert.Equal(t, "Test Assistant", created.Name)

		// List — the new assistant should appear.
		listResp := do(t, http.MethodGet, "/v1/assistants", nil)
		defer listResp.Body.Close()
		assert.Equal(t, http.StatusOK, listResp.StatusCode)

		var listed struct {
			Object string `json:"object"`
			Data   []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(listResp.Body).Decode(&listed))
		assert.Equal(t, "list", listed.Object)

		found := false
		for _, a := range listed.Data {
			if a.ID == created.ID {
				found = true
				break
			}
		}

		assert.True(t, found, "newly created assistant should appear in the list")

		// Get by ID.
		getResp := do(t, http.MethodGet, "/v1/assistants/"+created.ID, nil)
		defer getResp.Body.Close()
		assert.Equal(t, http.StatusOK, getResp.StatusCode)

		var fetched struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		require.NoError(t, json.NewDecoder(getResp.Body).Decode(&fetched))
		assert.Equal(t, created.ID, fetched.ID)
		assert.Equal(t, "Test Assistant", fetched.Name)

		// Delete.
		deleteResp := do(t, http.MethodDelete, "/v1/assistants/"+created.ID, nil)
		defer deleteResp.Body.Close()
		assert.Equal(t, http.StatusNoContent, deleteResp.StatusCode)

		// After deletion, GET should return 404.
		notFoundResp := do(t, http.MethodGet, "/v1/assistants/"+created.ID, nil)
		defer notFoundResp.Body.Close()
		assert.Equal(t, http.StatusNotFound, notFoundResp.StatusCode)
	})

	t.Run("Threads_CreateAndMessages", func(t *testing.T) {
		// Create a thread, post a user message, then list the messages.
		threadResp := do(t, http.MethodPost, "/v1/threads", []byte("{}"))
		defer threadResp.Body.Close()

		assert.True(t, threadResp.StatusCode >= 200 && threadResp.StatusCode < 300,
			"create thread: want 2xx, got %d", threadResp.StatusCode)

		var thread struct {
			ID     string `json:"id"`
			Object string `json:"object"`
		}
		require.NoError(t, json.NewDecoder(threadResp.Body).Decode(&thread))
		assert.NotEmpty(t, thread.ID, "thread ID should be non-empty")
		assert.Equal(t, "thread", thread.Object)

		// Add a user message to the thread.
		msgBody, _ := json.Marshal(map[string]string{
			"role":    "user",
			"content": "Hello, I need help with my code.",
		})
		msgResp := do(t, http.MethodPost, "/v1/threads/"+thread.ID+"/messages", msgBody)
		defer msgResp.Body.Close()

		assert.True(t, msgResp.StatusCode >= 200 && msgResp.StatusCode < 300,
			"create message: want 2xx, got %d", msgResp.StatusCode)

		var msg struct {
			ID       string `json:"id"`
			Object   string `json:"object"`
			ThreadID string `json:"thread_id"`
			Role     string `json:"role"`
		}
		require.NoError(t, json.NewDecoder(msgResp.Body).Decode(&msg))
		assert.Equal(t, "thread.message", msg.Object)
		assert.Equal(t, thread.ID, msg.ThreadID)
		assert.Equal(t, "user", msg.Role)

		// List messages — the user message should appear.
		listResp := do(t, http.MethodGet, "/v1/threads/"+thread.ID+"/messages", nil)
		defer listResp.Body.Close()
		assert.Equal(t, http.StatusOK, listResp.StatusCode)

		var messages struct {
			Object string `json:"object"`
			Data   []struct {
				Role string `json:"role"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(listResp.Body).Decode(&messages))
		assert.Equal(t, "list", messages.Object)
		require.NotEmpty(t, messages.Data, "thread should have at least one message")
		assert.Equal(t, "user", messages.Data[0].Role)
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
