package handler

import (
	"database/sql"
	"encoding/json"
	"strconv"

	"github.com/google/uuid"
	"gofr.dev/pkg/gofr"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/response"

	"aryanmehrotra/llm-gateway/batch"
	"aryanmehrotra/llm-gateway/middleware"
	"aryanmehrotra/llm-gateway/models"
)

// BatchHandler groups all batch processing endpoint handlers.
type BatchHandler struct {
	Processor *batch.Processor
}

// Submit handles POST /v1/batches — submits a batch of requests for async processing.
func (h *BatchHandler) Submit() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		var req models.BatchSubmitRequest
		if err := ctx.Bind(&req); err != nil {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"request body"}}
		}

		if len(req.Requests) == 0 {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"requests"}}
		}

		batchID := uuid.New().String()

		// Get key hash for tracking
		var keyHash string
		if authKey := middleware.GetAuthKey(ctx); authKey != "" {
			keyHash = sha256hex(authKey)
		}

		// Insert batch record
		_, err := ctx.SQL.ExecContext(ctx,
			`INSERT INTO batches (id, status, total_requests, key_hash) VALUES ($1, 'pending', $2, $3)`,
			batchID, len(req.Requests), keyHash)
		if err != nil {
			ctx.Errorf("create batch: %v", err)
			return nil, ErrInternal("failed to create batch")
		}

		// Insert batch items
		for _, r := range req.Requests {
			bodyStr := string(r.Body)

			method := r.Method
			if method == "" {
				method = "POST"
			}

			_, err := ctx.SQL.ExecContext(ctx,
				`INSERT INTO batch_items (batch_id, custom_id, method, url, body) VALUES ($1, $2, $3, $4, $5)`,
				batchID, r.CustomID, method, r.URL, bodyStr)
			if err != nil {
				ctx.Errorf("create batch item: %v", err)
				return nil, ErrInternal("failed to create batch item")
			}
		}

		// Submit to worker pool for processing
		if err := h.Processor.ProcessBatch(ctx, batchID); err != nil {
			ctx.Errorf("batch processing error: %v", err)
		}

		return response.Raw{Data: models.BatchResponse{
			ID:            batchID,
			Status:        "pending",
			TotalRequests: len(req.Requests),
		}}, nil
	}
}

// Status handles GET /v1/batches/{id} — returns batch status and counts.
func (h *BatchHandler) Status() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		id := ctx.PathParam("id")
		if id == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"id"}}
		}

		var resp models.BatchResponse
		var completedAt sql.NullString

		err := ctx.SQL.QueryRowContext(ctx,
			`SELECT id, status, total_requests, completed_requests, failed_requests, created_at, completed_at
			 FROM batches WHERE id = $1`, id,
		).Scan(&resp.ID, &resp.Status, &resp.TotalRequests, &resp.CompletedRequests,
			&resp.FailedRequests, &resp.CreatedAt, &completedAt)
		if err == sql.ErrNoRows {
			return nil, ErrNotFound("batch")
		}

		if err != nil {
			ctx.Errorf("query batch: %v", err)
			return nil, ErrInternal("failed to retrieve batch")
		}

		if completedAt.Valid {
			resp.CompletedAt = &completedAt.String
		}

		return response.Raw{Data: resp}, nil
	}
}

// Results handles GET /v1/batches/{id}/results — returns completed results.
func (h *BatchHandler) Results() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		id := ctx.PathParam("id")
		if id == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"id"}}
		}

		rows, err := ctx.SQL.QueryContext(ctx,
			`SELECT custom_id, status_code, result, error FROM batch_items
			 WHERE batch_id = $1 AND status IN ('completed', 'failed')
			 ORDER BY id`, id)
		if err != nil {
			ctx.Errorf("query batch results: %v", err)
			return nil, ErrInternal("failed to query batch results")
		}
		defer rows.Close()

		var results []models.BatchResultItem

		for rows.Next() {
			var item models.BatchResultItem
			var resultStr sql.NullString
			var errStr sql.NullString
			var statusCode sql.NullInt64

			if err := rows.Scan(&item.CustomID, &statusCode, &resultStr, &errStr); err != nil {
				continue
			}

			if statusCode.Valid {
				item.StatusCode = int(statusCode.Int64)
			}

			if resultStr.Valid {
				item.Body = json.RawMessage(resultStr.String)
			}

			if errStr.Valid {
				item.Error = errStr.String
			}

			results = append(results, item)
		}

		if err := rows.Err(); err != nil {
			ctx.Errorf("iterate batch results: %v", err)
			return nil, ErrInternal("failed to read batch results")
		}

		return response.Raw{Data: results}, nil
	}
}

// Cancel handles POST /v1/batches/{id}/cancel — cancels a batch.
func (h *BatchHandler) Cancel() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		id := ctx.PathParam("id")
		if id == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"id"}}
		}

		result, err := ctx.SQL.ExecContext(ctx,
			"UPDATE batches SET status = 'cancelled', completed_at = CURRENT_TIMESTAMP WHERE id = $1 AND status IN ('pending', 'processing')",
			id)
		if err != nil {
			ctx.Errorf("cancel batch: %v", err)
			return nil, ErrInternal("failed to cancel batch")
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return nil, ErrNotFound("batch (not found or already completed)")
		}

		// Cancel pending items
		_, _ = ctx.SQL.ExecContext(ctx,
			"UPDATE batch_items SET status = 'cancelled' WHERE batch_id = $1 AND status = 'pending'", id)

		return response.Raw{Data: map[string]string{
			"id":     id,
			"status": "cancelled",
		}}, nil
	}
}

// List handles GET /v1/batches — lists batches with pagination.
func (h *BatchHandler) List() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		limit, _ := strconv.Atoi(ctx.Param("limit"))
		if limit <= 0 || limit > 100 {
			limit = 20
		}

		offset, _ := strconv.Atoi(ctx.Param("offset"))
		if offset < 0 {
			offset = 0
		}

		rows, err := ctx.SQL.QueryContext(ctx,
			`SELECT id, status, total_requests, completed_requests, failed_requests, created_at, completed_at
			 FROM batches ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
		if err != nil {
			ctx.Errorf("list batches: %v", err)
			return nil, ErrInternal("failed to list batches")
		}
		defer rows.Close()

		var batches []models.BatchResponse

		for rows.Next() {
			var b models.BatchResponse
			var completedAt sql.NullString

			if err := rows.Scan(&b.ID, &b.Status, &b.TotalRequests, &b.CompletedRequests,
				&b.FailedRequests, &b.CreatedAt, &completedAt); err != nil {
				continue
			}

			if completedAt.Valid {
				b.CompletedAt = &completedAt.String
			}

			batches = append(batches, b)
		}

		if err := rows.Err(); err != nil {
			ctx.Errorf("iterate batches: %v", err)
			return nil, ErrInternal("failed to list batches")
		}

		if batches == nil {
			batches = []models.BatchResponse{}
		}

		return response.Raw{Data: map[string]any{
			"data":   batches,
			"limit":  limit,
			"offset": offset,
		}}, nil
	}
}
