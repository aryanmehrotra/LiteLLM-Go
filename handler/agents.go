package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"gofr.dev/pkg/gofr"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/response"

	"aryanmehrotra/litellm-go/models"
	"aryanmehrotra/litellm-go/websearch"
)

const (
	defaultMaxIterations = 10
	agentWebSearchTool   = "web_search"
)

// AgentRun handles POST /v1/agents/run.
// It implements a multi-turn agentic loop:
//  1. Send messages + tools to LLM
//  2. If the model returns tool_calls, execute built-in tools (web_search) or call webhooks
//  3. Append tool results to messages and repeat
//  4. Stop when finish_reason is "stop" or max_iterations is reached
func (h *APIHandler) AgentRun() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		var req models.AgentRunRequest
		if err := ctx.Bind(&req); err != nil {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"request body"}}
		}

		if req.Model == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"model"}}
		}

		if len(req.Messages) == 0 {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"messages"}}
		}

		maxIter := req.MaxIterations
		if maxIter <= 0 {
			maxIter = defaultMaxIterations
		}

		if maxIter > 50 {
			maxIter = 50 // hard cap
		}

		// Resolve provider once (model string stays clean after first resolve)
		p, modelName, err := h.Registry.ResolveProvider(req.Model)
		if err != nil {
			return nil, ErrInvalidParam("model", fmt.Sprintf("model %q not found", req.Model))
		}

		runID := uuid.New().String()
		messages := make([]models.Message, len(req.Messages))
		copy(messages, req.Messages)

		totalUsage := models.Usage{}
		var steps []models.AgentStep
		finalFinishReason := "stop"

		for iteration := 1; iteration <= maxIter; iteration++ {
			chatReq := models.ChatCompletionRequest{
				Model:       modelName,
				Messages:    messages,
				Tools:       req.Tools,
				Temperature: req.Temperature,
				TopP:        req.TopP,
				MaxTokens:   req.MaxTokens,
			}

			resp, err := h.Router.ChatCompletion(ctx, p, modelName, chatReq)
			if err != nil {
				ctx.Errorf("agent run iteration %d error: %v", iteration, err)
				return nil, err
			}

			// Accumulate usage
			totalUsage.PromptTokens += resp.Usage.PromptTokens
			totalUsage.CompletionTokens += resp.Usage.CompletionTokens
			totalUsage.TotalTokens += resp.Usage.TotalTokens

			if len(resp.Choices) == 0 {
				break
			}

			choice := resp.Choices[0]
			finalFinishReason = choice.FinishReason

			step := models.AgentStep{
				Iteration:    iteration,
				Messages:     messages,
				FinishReason: choice.FinishReason,
			}

			// If no tool calls or finish_reason is "stop", we are done
			if choice.FinishReason != "tool_calls" || len(choice.Message.ToolCalls) == 0 {
				steps = append(steps, step)
				messages = append(messages, choice.Message)
				break
			}

			// Add assistant message with tool calls to history
			assistantMsg := models.Message{
				Role:      "assistant",
				Content:   choice.Message.Content,
				ToolCalls: choice.Message.ToolCalls,
			}
			messages = append(messages, assistantMsg)
			step.ToolCalls = choice.Message.ToolCalls

			// Execute each tool call
			var toolResults []models.Message
			for _, tc := range choice.Message.ToolCalls {
				result := h.executeToolCall(ctx, tc, req.ToolWebhooks)
				toolMsg := models.Message{
					Role:       "tool",
					Content:    result,
					ToolCallID: tc.ID,
				}
				messages = append(messages, toolMsg)
				toolResults = append(toolResults, toolMsg)
			}

			step.ToolResults = toolResults
			steps = append(steps, step)

			// Safety: if we've hit the max, mark finish reason accordingly
			if iteration == maxIter {
				finalFinishReason = "max_iterations"
			}
		}

		// Get final message
		var finalMessage models.Message
		if len(messages) > 0 {
			finalMessage = messages[len(messages)-1]
		}

		return response.Raw{Data: models.AgentRunResponse{
			ID:           "agent-" + runID,
			Object:       "agent.run",
			Model:        req.Model,
			Steps:        steps,
			FinalMessage: finalMessage,
			Usage:        totalUsage,
			FinishReason: finalFinishReason,
			Iterations:   len(steps),
		}}, nil
	}
}

// runAgentLoop is an internal helper that executes the agent loop and returns
// the structured result without wrapping in a GoFr response. Used by Assistants runs.
func (h *APIHandler) runAgentLoop(ctx *gofr.Context, req models.AgentRunRequest) (*models.AgentRunResponse, error) {
	maxIter := req.MaxIterations
	if maxIter <= 0 {
		maxIter = defaultMaxIterations
	}

	if maxIter > 50 {
		maxIter = 50
	}

	p, modelName, err := h.Registry.ResolveProvider(req.Model)
	if err != nil {
		return nil, ErrInvalidParam("model", fmt.Sprintf("model %q not found", req.Model))
	}

	runID := uuid.New().String()
	messages := make([]models.Message, len(req.Messages))
	copy(messages, req.Messages)

	totalUsage := models.Usage{}
	var steps []models.AgentStep
	finalFinishReason := "stop"

	for iteration := 1; iteration <= maxIter; iteration++ {
		chatReq := models.ChatCompletionRequest{
			Model:       modelName,
			Messages:    messages,
			Tools:       req.Tools,
			Temperature: req.Temperature,
			TopP:        req.TopP,
			MaxTokens:   req.MaxTokens,
		}

		resp, err := h.Router.ChatCompletion(ctx, p, modelName, chatReq)
		if err != nil {
			return nil, err
		}

		totalUsage.PromptTokens += resp.Usage.PromptTokens
		totalUsage.CompletionTokens += resp.Usage.CompletionTokens
		totalUsage.TotalTokens += resp.Usage.TotalTokens

		if len(resp.Choices) == 0 {
			break
		}

		choice := resp.Choices[0]
		finalFinishReason = choice.FinishReason

		step := models.AgentStep{
			Iteration:    iteration,
			Messages:     messages,
			FinishReason: choice.FinishReason,
		}

		if choice.FinishReason != "tool_calls" || len(choice.Message.ToolCalls) == 0 {
			steps = append(steps, step)
			messages = append(messages, choice.Message)
			break
		}

		assistantMsg := models.Message{
			Role:      "assistant",
			Content:   choice.Message.Content,
			ToolCalls: choice.Message.ToolCalls,
		}
		messages = append(messages, assistantMsg)
		step.ToolCalls = choice.Message.ToolCalls

		var toolResults []models.Message
		for _, tc := range choice.Message.ToolCalls {
			result := h.executeToolCall(ctx, tc, req.ToolWebhooks)
			toolMsg := models.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			}
			messages = append(messages, toolMsg)
			toolResults = append(toolResults, toolMsg)
		}

		step.ToolResults = toolResults
		steps = append(steps, step)

		if iteration == maxIter {
			finalFinishReason = "max_iterations"
		}
	}

	var finalMessage models.Message
	if len(messages) > 0 {
		finalMessage = messages[len(messages)-1]
	}

	return &models.AgentRunResponse{
		ID:           "agent-" + runID,
		Object:       "agent.run",
		Model:        req.Model,
		Steps:        steps,
		FinalMessage: finalMessage,
		Usage:        totalUsage,
		FinishReason: finalFinishReason,
		Iterations:   len(steps),
	}, nil
}

// It supports built-in tools (web_search) and webhook-based tool execution.
func (h *APIHandler) executeToolCall(ctx *gofr.Context, tc models.ToolCall, webhooks map[string]string) string {
	toolName := tc.Function.Name

	// Built-in tool: web_search
	if toolName == agentWebSearchTool {
		return h.executeWebSearch(ctx, tc.Function.Arguments)
	}

	// Webhook-based tool execution
	if webhooks != nil {
		if url, ok := webhooks[toolName]; ok {
			return callWebhook(url, tc.Function.Arguments)
		}
	}

	// Unknown tool — return a helpful error message
	return fmt.Sprintf(`{"error": "tool %q is not registered; provide a webhook via tool_webhooks to execute it"}`, toolName)
}

// executeWebSearch performs a web search using the configured search service.
func (h *APIHandler) executeWebSearch(ctx *gofr.Context, argsJSON string) string {
	if h.Search == nil {
		return `{"error": "web search is not configured on this gateway"}`
	}

	var args struct {
		Query string `json:"query"`
	}

	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil || args.Query == "" {
		return `{"error": "web_search requires a query argument"}`
	}

	results := h.Search.SearchDirect(ctx, args.Query, 5)
	if len(results) == 0 {
		return `{"results": [], "message": "no results found"}`
	}

	type searchResult struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Snippet string `json:"snippet"`
	}

	var sr []searchResult
	for _, r := range results {
		sr = append(sr, searchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Snippet,
		})
	}

	b, _ := json.Marshal(map[string]any{
		"query":   args.Query,
		"results": sr,
		"context": websearch.FormatResults(results),
	})

	return string(b)
}

// callWebhook POSTs the tool arguments to the given URL and returns the result.
func callWebhook(url string, argsJSON string) string {
	payload := map[string]string{"arguments": argsJSON}
	body, _ := json.Marshal(payload)

	client := &http.Client{Timeout: 30 * time.Second}

	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Sprintf(`{"error": "webhook call failed: %s"}`, err.Error())
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB limit
	if err != nil {
		return fmt.Sprintf(`{"error": "failed to read webhook response: %s"}`, err.Error())
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Sprintf(`{"error": "webhook returned status %d: %s"}`, resp.StatusCode, string(respBody))
	}

	return string(respBody)
}
