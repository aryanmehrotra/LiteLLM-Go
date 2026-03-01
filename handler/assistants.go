package handler

import (
	"database/sql"
	"encoding/json"
	"strconv"
	"time"

	"github.com/google/uuid"
	"gofr.dev/pkg/gofr"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/response"

	"aryanmehrotra/litellm-go/models"
)

// AssistantHandler groups all Assistants API handlers.
type AssistantHandler struct {
	API *APIHandler
}

// CreateAssistant handles POST /v1/assistants.
func (h *AssistantHandler) CreateAssistant() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		var req models.AssistantRequest
		if err := ctx.Bind(&req); err != nil {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"request body"}}
		}

		if req.Model == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"model"}}
		}

		id := "asst_" + uuid.New().String()
		now := time.Now().Unix()

		toolsJSON := "[]"
		if b, err := json.Marshal(req.Tools); err == nil {
			toolsJSON = string(b)
		}

		metaJSON := "{}"
		if b, err := json.Marshal(req.Metadata); err == nil {
			metaJSON = string(b)
		}

		_, err := ctx.SQL.ExecContext(ctx,
			`INSERT INTO assistants (id, name, description, model, instructions, tools, metadata, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			id, req.Name, req.Description, req.Model, req.Instructions, toolsJSON, metaJSON, now)
		if err != nil {
			ctx.Errorf("create assistant: %v", err)
			return nil, ErrInternal("failed to create assistant")
		}

		return response.Raw{Data: models.Assistant{
			ID:           id,
			Object:       "assistant",
			CreatedAt:    now,
			Name:         req.Name,
			Description:  req.Description,
			Model:        req.Model,
			Instructions: req.Instructions,
			Tools:        req.Tools,
			Metadata:     req.Metadata,
		}}, nil
	}
}

// ListAssistants handles GET /v1/assistants.
func (h *AssistantHandler) ListAssistants() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		limit, _ := strconv.Atoi(ctx.Param("limit"))
		if limit <= 0 || limit > 100 {
			limit = 20
		}

		rows, err := ctx.SQL.QueryContext(ctx,
			`SELECT id, name, description, model, instructions, tools, metadata, created_at
			 FROM assistants ORDER BY created_at DESC LIMIT $1`, limit)
		if err != nil {
			ctx.Errorf("list assistants: %v", err)
			return nil, ErrInternal("failed to list assistants")
		}
		defer rows.Close()

		var assistants []models.Assistant

		for rows.Next() {
			var a models.Assistant
			var toolsJSON, metaJSON string

			if err := rows.Scan(&a.ID, &a.Name, &a.Description, &a.Model, &a.Instructions,
				&toolsJSON, &metaJSON, &a.CreatedAt); err != nil {
				continue
			}

			a.Object = "assistant"
			_ = json.Unmarshal([]byte(toolsJSON), &a.Tools)
			_ = json.Unmarshal([]byte(metaJSON), &a.Metadata)

			assistants = append(assistants, a)
		}

		if assistants == nil {
			assistants = []models.Assistant{}
		}

		return response.Raw{Data: models.AssistantListResponse{
			Object:  "list",
			Data:    assistants,
			HasMore: len(assistants) == limit,
		}}, nil
	}
}

// GetAssistant handles GET /v1/assistants/{id}.
func (h *AssistantHandler) GetAssistant() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		id := ctx.PathParam("id")
		if id == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"id"}}
		}

		var a models.Assistant
		var toolsJSON, metaJSON string

		err := ctx.SQL.QueryRowContext(ctx,
			`SELECT id, name, description, model, instructions, tools, metadata, created_at
			 FROM assistants WHERE id = $1`, id,
		).Scan(&a.ID, &a.Name, &a.Description, &a.Model, &a.Instructions,
			&toolsJSON, &metaJSON, &a.CreatedAt)
		if err == sql.ErrNoRows {
			return nil, ErrNotFound("assistant")
		}

		if err != nil {
			ctx.Errorf("get assistant: %v", err)
			return nil, ErrInternal("failed to retrieve assistant")
		}

		a.Object = "assistant"
		_ = json.Unmarshal([]byte(toolsJSON), &a.Tools)
		_ = json.Unmarshal([]byte(metaJSON), &a.Metadata)

		return response.Raw{Data: a}, nil
	}
}

// DeleteAssistant handles DELETE /v1/assistants/{id}.
func (h *AssistantHandler) DeleteAssistant() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		id := ctx.PathParam("id")
		if id == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"id"}}
		}

		result, err := ctx.SQL.ExecContext(ctx, `DELETE FROM assistants WHERE id = $1`, id)
		if err != nil {
			ctx.Errorf("delete assistant: %v", err)
			return nil, ErrInternal("failed to delete assistant")
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return nil, ErrNotFound("assistant")
		}

		return response.Raw{Data: map[string]any{
			"id":      id,
			"object":  "assistant.deleted",
			"deleted": true,
		}}, nil
	}
}

// CreateThread handles POST /v1/threads.
func (h *AssistantHandler) CreateThread() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		var req models.ThreadRequest
		// Bind is optional — threads can be created with no body
		_ = ctx.Bind(&req)

		id := "thread_" + uuid.New().String()
		now := time.Now().Unix()

		metaJSON := "{}"
		if b, err := json.Marshal(req.Metadata); err == nil {
			metaJSON = string(b)
		}

		_, err := ctx.SQL.ExecContext(ctx,
			`INSERT INTO threads (id, metadata, created_at) VALUES ($1, $2, $3)`,
			id, metaJSON, now)
		if err != nil {
			ctx.Errorf("create thread: %v", err)
			return nil, ErrInternal("failed to create thread")
		}

		// Insert initial messages if provided
		for _, msg := range req.Messages {
			msgID := "msg_" + uuid.New().String()
			contentJSON, _ := json.Marshal([]models.MessageContent{{
				Type: "text",
				Text: &models.TextContent{Value: msg.Content, Annotations: []any{}},
			}})

			_, _ = ctx.SQL.ExecContext(ctx,
				`INSERT INTO thread_messages (id, thread_id, role, content, created_at)
				 VALUES ($1, $2, $3, $4, $5)`,
				msgID, id, msg.Role, string(contentJSON), now)
		}

		thread := models.Thread{
			ID:        id,
			Object:    "thread",
			CreatedAt: now,
			Metadata:  req.Metadata,
		}

		return response.Raw{Data: thread}, nil
	}
}

// GetThread handles GET /v1/threads/{id}.
func (h *AssistantHandler) GetThread() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		id := ctx.PathParam("id")
		if id == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"id"}}
		}

		var t models.Thread
		var metaJSON string

		err := ctx.SQL.QueryRowContext(ctx,
			`SELECT id, metadata, created_at FROM threads WHERE id = $1`, id,
		).Scan(&t.ID, &metaJSON, &t.CreatedAt)
		if err == sql.ErrNoRows {
			return nil, ErrNotFound("thread")
		}

		if err != nil {
			ctx.Errorf("get thread: %v", err)
			return nil, ErrInternal("failed to retrieve thread")
		}

		t.Object = "thread"
		_ = json.Unmarshal([]byte(metaJSON), &t.Metadata)

		return response.Raw{Data: t}, nil
	}
}

// DeleteThread handles DELETE /v1/threads/{id}.
func (h *AssistantHandler) DeleteThread() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		id := ctx.PathParam("id")
		if id == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"id"}}
		}

		result, err := ctx.SQL.ExecContext(ctx, `DELETE FROM threads WHERE id = $1`, id)
		if err != nil {
			ctx.Errorf("delete thread: %v", err)
			return nil, ErrInternal("failed to delete thread")
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return nil, ErrNotFound("thread")
		}

		return response.Raw{Data: map[string]any{
			"id":      id,
			"object":  "thread.deleted",
			"deleted": true,
		}}, nil
	}
}

// CreateMessage handles POST /v1/threads/{id}/messages.
func (h *AssistantHandler) CreateMessage() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		threadID := ctx.PathParam("id")
		if threadID == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"id"}}
		}

		var req models.ThreadMessage
		if err := ctx.Bind(&req); err != nil {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"request body"}}
		}

		if req.Content == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"content"}}
		}

		if req.Role == "" {
			req.Role = "user"
		}

		// Ensure thread exists
		var threadCheck string
		if err := ctx.SQL.QueryRowContext(ctx, `SELECT id FROM threads WHERE id = $1`, threadID).Scan(&threadCheck); err == sql.ErrNoRows {
			return nil, ErrNotFound("thread")
		}

		msgID := "msg_" + uuid.New().String()
		now := time.Now().Unix()

		contentJSON, _ := json.Marshal([]models.MessageContent{{
			Type: "text",
			Text: &models.TextContent{Value: req.Content, Annotations: []any{}},
		}})

		_, err := ctx.SQL.ExecContext(ctx,
			`INSERT INTO thread_messages (id, thread_id, role, content, created_at) VALUES ($1, $2, $3, $4, $5)`,
			msgID, threadID, req.Role, string(contentJSON), now)
		if err != nil {
			ctx.Errorf("create message: %v", err)
			return nil, ErrInternal("failed to create message")
		}

		return response.Raw{Data: models.ThreadMessageObject{
			ID:        msgID,
			Object:    "thread.message",
			CreatedAt: now,
			ThreadID:  threadID,
			Role:      req.Role,
			Content: []models.MessageContent{{
				Type: "text",
				Text: &models.TextContent{Value: req.Content, Annotations: []any{}},
			}},
		}}, nil
	}
}

// ListMessages handles GET /v1/threads/{id}/messages.
func (h *AssistantHandler) ListMessages() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		threadID := ctx.PathParam("id")
		if threadID == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"id"}}
		}

		limit, _ := strconv.Atoi(ctx.Param("limit"))
		if limit <= 0 || limit > 100 {
			limit = 20
		}

		rows, err := ctx.SQL.QueryContext(ctx,
			`SELECT id, thread_id, role, content, assistant_id, run_id, created_at
			 FROM thread_messages WHERE thread_id = $1 ORDER BY created_at ASC LIMIT $2`,
			threadID, limit)
		if err != nil {
			ctx.Errorf("list messages: %v", err)
			return nil, ErrInternal("failed to list messages")
		}
		defer rows.Close()

		var msgs []models.ThreadMessageObject

		for rows.Next() {
			var m models.ThreadMessageObject
			var contentJSON string
			var assistantID, runID sql.NullString

			if err := rows.Scan(&m.ID, &m.ThreadID, &m.Role, &contentJSON,
				&assistantID, &runID, &m.CreatedAt); err != nil {
				continue
			}

			m.Object = "thread.message"

			if assistantID.Valid {
				m.AssistantID = assistantID.String
			}

			if runID.Valid {
				m.RunID = runID.String
			}

			_ = json.Unmarshal([]byte(contentJSON), &m.Content)
			msgs = append(msgs, m)
		}

		if msgs == nil {
			msgs = []models.ThreadMessageObject{}
		}

		return response.Raw{Data: models.ThreadMessageListResponse{
			Object:  "list",
			Data:    msgs,
			HasMore: len(msgs) == limit,
		}}, nil
	}
}

// CreateRun handles POST /v1/threads/{id}/runs.
// It executes the assistant on the thread — loading the conversation history,
// running the agent loop (with the assistant's tools), and saving the response.
func (h *AssistantHandler) CreateRun() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		threadID := ctx.PathParam("id")
		if threadID == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"id"}}
		}

		var req models.RunRequest
		if err := ctx.Bind(&req); err != nil {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"request body"}}
		}

		if req.AssistantID == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"assistant_id"}}
		}

		// Load assistant
		var assistant models.Assistant
		var toolsJSON, metaJSON string

		err := ctx.SQL.QueryRowContext(ctx,
			`SELECT id, name, model, instructions, tools, metadata, created_at FROM assistants WHERE id = $1`,
			req.AssistantID,
		).Scan(&assistant.ID, &assistant.Name, &assistant.Model, &assistant.Instructions,
			&toolsJSON, &metaJSON, &assistant.CreatedAt)
		if err == sql.ErrNoRows {
			return nil, ErrNotFound("assistant")
		}

		if err != nil {
			ctx.Errorf("load assistant for run: %v", err)
			return nil, ErrInternal("failed to load assistant")
		}

		_ = json.Unmarshal([]byte(toolsJSON), &assistant.Tools)

		// Override with run-level settings
		model := assistant.Model
		if req.Model != "" {
			model = req.Model
		}

		instructions := assistant.Instructions
		if req.Instructions != "" {
			instructions = req.Instructions
		}

		tools := assistant.Tools
		if len(req.Tools) > 0 {
			tools = req.Tools
		}

		// Create run record
		runID := "run_" + uuid.New().String()
		now := time.Now().Unix()
		toolsForRunJSON, _ := json.Marshal(tools)

		_, err = ctx.SQL.ExecContext(ctx,
			`INSERT INTO runs (id, thread_id, assistant_id, model, instructions, tools, status, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, 'queued', $7)`,
			runID, threadID, req.AssistantID, model, instructions, string(toolsForRunJSON), now)
		if err != nil {
			ctx.Errorf("create run: %v", err)
			return nil, ErrInternal("failed to create run")
		}

		// Load thread messages
		msgRows, err := ctx.SQL.QueryContext(ctx,
			`SELECT role, content FROM thread_messages WHERE thread_id = $1 ORDER BY created_at ASC`,
			threadID)
		if err != nil {
			ctx.Errorf("load thread messages for run: %v", err)
			return nil, ErrInternal("failed to load thread messages")
		}
		defer msgRows.Close()

		var messages []models.Message

		if instructions != "" {
			messages = append(messages, models.Message{Role: "system", Content: instructions})
		}

		for msgRows.Next() {
			var role, contentJSON string
			if err := msgRows.Scan(&role, &contentJSON); err != nil {
				continue
			}

			var content []models.MessageContent
			_ = json.Unmarshal([]byte(contentJSON), &content)

			text := ""
			for _, c := range content {
				if c.Type == "text" && c.Text != nil {
					text += c.Text.Value
				}
			}

			messages = append(messages, models.Message{Role: role, Content: text})
		}

		msgRows.Close()

		if len(messages) == 0 || (len(messages) == 1 && messages[0].Role == "system") {
			_, _ = ctx.SQL.ExecContext(ctx,
				`UPDATE runs SET status = 'failed', failed_at = $1 WHERE id = $2`, time.Now().Unix(), runID)
			return nil, ErrBadRequest("thread has no messages")
		}

		// Convert AssistantTool to models.Tool for the agent loop
		var chatTools []models.Tool
		for _, at := range tools {
			if at.Type == "function" && at.Function != nil {
				chatTools = append(chatTools, models.Tool{
					Type:     "function",
					Function: *at.Function,
				})
			}
		}

		// Update run status to in_progress
		_, _ = ctx.SQL.ExecContext(ctx,
			`UPDATE runs SET status = 'in_progress', started_at = $1 WHERE id = $2`, time.Now().Unix(), runID)

		// Execute the agent loop
		agentReq := models.AgentRunRequest{
			Model:         model,
			Messages:      messages,
			Tools:         chatTools,
			MaxIterations: defaultMaxIterations,
		}

		agentResp, agentErr := h.API.runAgentLoop(ctx, agentReq)
		if agentErr != nil {
			ctx.Errorf("run agent loop: %v", agentErr)
			_, _ = ctx.SQL.ExecContext(ctx,
				`UPDATE runs SET status = 'failed', failed_at = $1 WHERE id = $2`, time.Now().Unix(), runID)
			return nil, agentErr
		}

		// Save assistant reply as thread message
		replyContent := agentResp.FinalMessage.Content
		if replyContent != "" {
			replyID := "msg_" + uuid.New().String()
			replyContentJSON, _ := json.Marshal([]models.MessageContent{{
				Type: "text",
				Text: &models.TextContent{Value: replyContent, Annotations: []any{}},
			}})

			_, _ = ctx.SQL.ExecContext(ctx,
				`INSERT INTO thread_messages (id, thread_id, role, content, assistant_id, run_id, created_at)
				 VALUES ($1, $2, 'assistant', $3, $4, $5, $6)`,
				replyID, threadID, string(replyContentJSON), req.AssistantID, runID, time.Now().Unix())
		}

		// Update run to completed
		completedAt := time.Now().Unix()
		_, _ = ctx.SQL.ExecContext(ctx,
			`UPDATE runs SET status = 'completed', completed_at = $1 WHERE id = $2`, completedAt, runID)

		run := models.Run{
			ID:           runID,
			Object:       "thread.run",
			CreatedAt:    now,
			ThreadID:     threadID,
			AssistantID:  req.AssistantID,
			Status:       "completed",
			Model:        model,
			Instructions: instructions,
			Tools:        tools,
			StartedAt:    &now,
			CompletedAt:  &completedAt,
			Usage:        &agentResp.Usage,
		}

		return response.Raw{Data: run}, nil
	}
}

// GetRun handles GET /v1/threads/{thread_id}/runs/{run_id}.
func (h *AssistantHandler) GetRun() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		threadID := ctx.PathParam("id")
		runID := ctx.PathParam("run_id")

		if threadID == "" || runID == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"id", "run_id"}}
		}

		var run models.Run
		var toolsJSON string
		var startedAt, completedAt, failedAt sql.NullInt64

		err := ctx.SQL.QueryRowContext(ctx,
			`SELECT id, thread_id, assistant_id, model, instructions, tools, status,
			        created_at, started_at, completed_at, failed_at
			 FROM runs WHERE id = $1 AND thread_id = $2`,
			runID, threadID,
		).Scan(&run.ID, &run.ThreadID, &run.AssistantID, &run.Model, &run.Instructions,
			&toolsJSON, &run.Status, &run.CreatedAt, &startedAt, &completedAt, &failedAt)
		if err == sql.ErrNoRows {
			return nil, ErrNotFound("run")
		}

		if err != nil {
			ctx.Errorf("get run: %v", err)
			return nil, ErrInternal("failed to retrieve run")
		}

		run.Object = "thread.run"
		_ = json.Unmarshal([]byte(toolsJSON), &run.Tools)

		if startedAt.Valid {
			run.StartedAt = &startedAt.Int64
		}

		if completedAt.Valid {
			run.CompletedAt = &completedAt.Int64
		}

		if failedAt.Valid {
			run.FailedAt = &failedAt.Int64
		}

		return response.Raw{Data: run}, nil
	}
}

// ListRuns handles GET /v1/threads/{id}/runs.
func (h *AssistantHandler) ListRuns() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		threadID := ctx.PathParam("id")
		if threadID == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"id"}}
		}

		limit, _ := strconv.Atoi(ctx.Param("limit"))
		if limit <= 0 || limit > 100 {
			limit = 20
		}

		rows, err := ctx.SQL.QueryContext(ctx,
			`SELECT id, thread_id, assistant_id, model, instructions, tools, status,
			        created_at, started_at, completed_at, failed_at
			 FROM runs WHERE thread_id = $1 ORDER BY created_at DESC LIMIT $2`,
			threadID, limit)
		if err != nil {
			ctx.Errorf("list runs: %v", err)
			return nil, ErrInternal("failed to list runs")
		}
		defer rows.Close()

		var runs []models.Run

		for rows.Next() {
			var run models.Run
			var toolsJSON string
			var startedAt, completedAt, failedAt sql.NullInt64

			if err := rows.Scan(&run.ID, &run.ThreadID, &run.AssistantID, &run.Model,
				&run.Instructions, &toolsJSON, &run.Status, &run.CreatedAt,
				&startedAt, &completedAt, &failedAt); err != nil {
				continue
			}

			run.Object = "thread.run"
			_ = json.Unmarshal([]byte(toolsJSON), &run.Tools)

			if startedAt.Valid {
				run.StartedAt = &startedAt.Int64
			}

			if completedAt.Valid {
				run.CompletedAt = &completedAt.Int64
			}

			if failedAt.Valid {
				run.FailedAt = &failedAt.Int64
			}

			runs = append(runs, run)
		}

		if runs == nil {
			runs = []models.Run{}
		}

		return response.Raw{Data: models.RunListResponse{
			Object:  "list",
			Data:    runs,
			HasMore: len(runs) == limit,
		}}, nil
	}
}

// CancelRun handles POST /v1/threads/{id}/runs/{run_id}/cancel.
func (h *AssistantHandler) CancelRun() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		threadID := ctx.PathParam("id")
		runID := ctx.PathParam("run_id")

		if threadID == "" || runID == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"id", "run_id"}}
		}

		_, err := ctx.SQL.ExecContext(ctx,
			`UPDATE runs SET status = 'cancelled', failed_at = $1
			 WHERE id = $2 AND thread_id = $3 AND status IN ('queued', 'in_progress')`,
			time.Now().Unix(), runID, threadID)
		if err != nil {
			ctx.Errorf("cancel run: %v", err)
			return nil, ErrInternal("failed to cancel run")
		}

		return h.GetRun()(ctx)
	}
}
