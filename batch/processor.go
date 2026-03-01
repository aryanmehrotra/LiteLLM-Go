package batch

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"gofr.dev/pkg/gofr"

	"aryanmehrotra/litellm-go/cache"
	"aryanmehrotra/litellm-go/models"
	"aryanmehrotra/litellm-go/provider"
	"aryanmehrotra/litellm-go/routing"
	"aryanmehrotra/litellm-go/workerpool"
)

// DB abstracts the database operations needed by the batch processor.
type DB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// Processor handles batch request processing using a worker pool.
type Processor struct {
	registry *provider.Registry
	cache    *cache.Cache
	router   *routing.Router
	pool     *workerpool.WorkerPool
}

// NewProcessor creates a batch processor with the given dependencies.
func NewProcessor(reg *provider.Registry, c *cache.Cache, router *routing.Router, pool *workerpool.WorkerPool) *Processor {
	return &Processor{
		registry: reg,
		cache:    c,
		router:   router,
		pool:     pool,
	}
}

// ProcessBatch loads pending items for a batch and submits them to the worker pool.
// The gofr.Context is captured for provider calls (HTTP services outlive the request).
func (bp *Processor) ProcessBatch(ctx *gofr.Context, batchID string) error {
	// Update batch status to processing
	_, err := ctx.SQL.ExecContext(ctx, "UPDATE batches SET status = 'processing' WHERE id = $1", batchID)
	if err != nil {
		return fmt.Errorf("update batch status: %w", err)
	}

	// Load pending items
	rows, err := ctx.SQL.QueryContext(ctx,
		"SELECT id, custom_id, body FROM batch_items WHERE batch_id = $1 AND status = 'pending'", batchID)
	if err != nil {
		return fmt.Errorf("load batch items: %w", err)
	}
	defer rows.Close()

	type batchItem struct {
		ID       int
		CustomID string
		Body     string
	}

	var items []batchItem

	for rows.Next() {
		var item batchItem
		if err := rows.Scan(&item.ID, &item.CustomID, &item.Body); err != nil {
			continue
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate batch items: %w", err)
	}

	// Capture gofr.Context and SQL handle for use in goroutines.
	// GoFr HTTP services and SQL pool outlive individual requests.
	gofrCtx := ctx
	sqlDB := ctx.SQL

	for _, item := range items {
		task := workerpool.NewTask(func(_ context.Context) {
			bp.processItem(gofrCtx, sqlDB, batchID, item.ID, item.Body)
		}, fmt.Sprintf("batch=%s item=%d", batchID, item.ID))

		if err := bp.pool.Submit(task); err != nil {
			_, _ = sqlDB.ExecContext(ctx,
				`UPDATE batch_items SET status = 'failed', error = $1, completed_at = CURRENT_TIMESTAMP WHERE id = $2`,
				err.Error(), item.ID)
			_, _ = sqlDB.ExecContext(ctx,
				"UPDATE batches SET failed_requests = failed_requests + 1 WHERE id = $1", batchID)
		}
	}

	return nil
}

// processItem handles a single batch item: parses the request, calls the provider, and updates the DB.
func (bp *Processor) processItem(ctx *gofr.Context, db DB, batchID string, itemID int, body string) {
	var req models.ChatCompletionRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		bp.failItem(ctx, db, batchID, itemID, "invalid request body: "+err.Error())
		return
	}

	p, modelName, err := bp.registry.ResolveProvider(req.Model)
	if err != nil {
		bp.failItem(ctx, db, batchID, itemID, "unknown model: "+req.Model)
		return
	}

	req.Model = modelName

	resp, err := p.ChatCompletion(ctx, req)
	if err != nil {
		bp.failItem(ctx, db, batchID, itemID, err.Error())
		return
	}

	respJSON, err := json.Marshal(resp)
	if err != nil {
		bp.failItem(ctx, db, batchID, itemID, "marshal response: "+err.Error())
		return
	}

	_, _ = db.ExecContext(ctx,
		`UPDATE batch_items SET status = 'completed', status_code = 200, result = $1, completed_at = CURRENT_TIMESTAMP WHERE id = $2`,
		string(respJSON), itemID)

	_, _ = db.ExecContext(ctx,
		"UPDATE batches SET completed_requests = completed_requests + 1 WHERE id = $1", batchID)

	bp.checkBatchComplete(ctx, db, batchID)
}

// failItem marks a batch item as failed and increments the batch failure counter.
func (bp *Processor) failItem(ctx context.Context, db DB, batchID string, itemID int, errMsg string) {
	_, _ = db.ExecContext(ctx,
		`UPDATE batch_items SET status = 'failed', status_code = 500, error = $1, completed_at = CURRENT_TIMESTAMP WHERE id = $2`,
		errMsg, itemID)

	_, _ = db.ExecContext(ctx,
		"UPDATE batches SET failed_requests = failed_requests + 1 WHERE id = $1", batchID)

	bp.checkBatchComplete(ctx, db, batchID)
}

// checkBatchComplete marks the batch as completed if all items are done.
func (bp *Processor) checkBatchComplete(ctx context.Context, db DB, batchID string) {
	_, _ = db.ExecContext(ctx,
		`UPDATE batches SET status = 'completed', completed_at = CURRENT_TIMESTAMP
		 WHERE id = $1 AND completed_requests + failed_requests >= total_requests AND status != 'cancelled'`,
		batchID)
}
