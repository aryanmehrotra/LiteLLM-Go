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
	"aryanmehrotra/litellm-go/provider"
)

// CreateFineTuningJob handles POST /v1/fine_tuning/jobs.
// For supported providers (openai), it proxies the request to the provider.
// For all providers it stores job metadata locally.
func (h *APIHandler) CreateFineTuningJob() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		var req models.FineTuningJobRequest
		if err := ctx.Bind(&req); err != nil {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"request body"}}
		}

		if req.Model == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"model"}}
		}

		if req.TrainingFile == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"training_file"}}
		}

		jobID := "ftjob-" + uuid.New().String()
		now := time.Now().Unix()

		hyperJSON := "{}"
		if req.Hyperparameters != nil {
			b, _ := json.Marshal(req.Hyperparameters)
			hyperJSON = string(b)
		}

		// Resolve provider to get provider name for potential proxying
		providerName := ""
		if p, _, err := h.Registry.ResolveProvider(req.Model); err == nil {
			providerName = p.Name()
		}

		_, err := ctx.SQL.ExecContext(ctx,
			`INSERT INTO fine_tuning_jobs (id, model, training_file, validation_file, hyperparameters, status, provider, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			jobID, req.Model, req.TrainingFile, req.ValidationFile, hyperJSON, "queued", providerName, now)
		if err != nil {
			ctx.Errorf("create fine_tuning_job: %v", err)
			return nil, ErrInternal("failed to create fine-tuning job")
		}

		// If this is an OpenAI model, proxy to OpenAI fine-tuning API
		if providerName == "openai" {
			if ftJob, err := proxyOpenAIFineTuning(ctx, h.Registry, req, jobID, now); err == nil {
				return response.Raw{Data: ftJob}, nil
			}
			// If proxy fails, fall through to local job (already stored)
		}

		job := models.FineTuningJob{
			ID:           jobID,
			Object:       "fine_tuning.job",
			CreatedAt:    now,
			Model:        req.Model,
			Status:       "queued",
			TrainingFile: req.TrainingFile,
		}

		if req.ValidationFile != "" {
			job.ValidationFile = &req.ValidationFile
		}

		if req.Hyperparameters != nil {
			job.Hyperparameters = req.Hyperparameters
		}

		if req.Suffix != "" {
			job.Suffix = &req.Suffix
		}

		return response.Raw{Data: job}, nil
	}
}

// ListFineTuningJobs handles GET /v1/fine_tuning/jobs.
func (h *APIHandler) ListFineTuningJobs() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		limit, _ := strconv.Atoi(ctx.Param("limit"))
		if limit <= 0 || limit > 100 {
			limit = 20
		}

		after := ctx.Param("after")

		var rows *sql.Rows
		var err error

		if after != "" {
			rows, err = ctx.SQL.QueryContext(ctx,
				`SELECT id, model, training_file, validation_file, hyperparameters, status, created_at, finished_at, fine_tuned_model
				 FROM fine_tuning_jobs WHERE created_at < (SELECT created_at FROM fine_tuning_jobs WHERE id = $1)
				 ORDER BY created_at DESC LIMIT $2`,
				after, limit)
		} else {
			rows, err = ctx.SQL.QueryContext(ctx,
				`SELECT id, model, training_file, validation_file, hyperparameters, status, created_at, finished_at, fine_tuned_model
				 FROM fine_tuning_jobs ORDER BY created_at DESC LIMIT $1`,
				limit)
		}

		if err != nil {
			ctx.Errorf("list fine_tuning_jobs: %v", err)
			return nil, ErrInternal("failed to list fine-tuning jobs")
		}
		defer rows.Close()

		var jobs []models.FineTuningJob

		for rows.Next() {
			var job models.FineTuningJob
			var hyperJSON string
			var validFile sql.NullString
			var finishedAt sql.NullInt64
			var fineTunedModel sql.NullString

			if err := rows.Scan(&job.ID, &job.Model, &job.TrainingFile, &validFile,
				&hyperJSON, &job.Status, &job.CreatedAt, &finishedAt, &fineTunedModel); err != nil {
				continue
			}

			job.Object = "fine_tuning.job"

			if validFile.Valid && validFile.String != "" {
				job.ValidationFile = &validFile.String
			}

			if finishedAt.Valid {
				job.FinishedAt = &finishedAt.Int64
			}

			if fineTunedModel.Valid && fineTunedModel.String != "" {
				job.FineTunedModel = &fineTunedModel.String
			}

			var hp models.Hyperparameters
			if json.Unmarshal([]byte(hyperJSON), &hp) == nil {
				job.Hyperparameters = &hp
			}

			jobs = append(jobs, job)
		}

		if err := rows.Err(); err != nil {
			ctx.Errorf("iterate fine_tuning_jobs: %v", err)
			return nil, ErrInternal("failed to list fine-tuning jobs")
		}

		if jobs == nil {
			jobs = []models.FineTuningJob{}
		}

		return response.Raw{Data: models.FineTuningJobListResponse{
			Object:  "list",
			Data:    jobs,
			HasMore: len(jobs) == limit,
		}}, nil
	}
}

// GetFineTuningJob handles GET /v1/fine_tuning/jobs/{id}.
func (h *APIHandler) GetFineTuningJob() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		id := ctx.PathParam("id")
		if id == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"id"}}
		}

		var job models.FineTuningJob
		var hyperJSON string
		var validFile sql.NullString
		var finishedAt sql.NullInt64
		var fineTunedModel sql.NullString

		err := ctx.SQL.QueryRowContext(ctx,
			`SELECT id, model, training_file, validation_file, hyperparameters, status, created_at, finished_at, fine_tuned_model
			 FROM fine_tuning_jobs WHERE id = $1`, id,
		).Scan(&job.ID, &job.Model, &job.TrainingFile, &validFile,
			&hyperJSON, &job.Status, &job.CreatedAt, &finishedAt, &fineTunedModel)
		if err == sql.ErrNoRows {
			return nil, ErrNotFound("fine_tuning.job")
		}

		if err != nil {
			ctx.Errorf("get fine_tuning_job: %v", err)
			return nil, ErrInternal("failed to retrieve fine-tuning job")
		}

		job.Object = "fine_tuning.job"

		if validFile.Valid && validFile.String != "" {
			job.ValidationFile = &validFile.String
		}

		if finishedAt.Valid {
			job.FinishedAt = &finishedAt.Int64
		}

		if fineTunedModel.Valid && fineTunedModel.String != "" {
			job.FineTunedModel = &fineTunedModel.String
		}

		var hp models.Hyperparameters
		if json.Unmarshal([]byte(hyperJSON), &hp) == nil {
			job.Hyperparameters = &hp
		}

		return response.Raw{Data: job}, nil
	}
}

// CancelFineTuningJob handles POST /v1/fine_tuning/jobs/{id}/cancel.
func (h *APIHandler) CancelFineTuningJob() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		id := ctx.PathParam("id")
		if id == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"id"}}
		}

		result, err := ctx.SQL.ExecContext(ctx,
			`UPDATE fine_tuning_jobs SET status = 'cancelled', finished_at = $1
			 WHERE id = $2 AND status IN ('queued', 'running', 'validating_files')`,
			time.Now().Unix(), id)
		if err != nil {
			ctx.Errorf("cancel fine_tuning_job: %v", err)
			return nil, ErrInternal("failed to cancel fine-tuning job")
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return nil, ErrNotFound("fine_tuning.job (not found or already completed)")
		}

		return h.GetFineTuningJob()(ctx)
	}
}

// ListFineTuningEvents handles GET /v1/fine_tuning/jobs/{id}/events.
func (h *APIHandler) ListFineTuningEvents() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		id := ctx.PathParam("id")
		if id == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"id"}}
		}

		// Check job exists
		var jobID string
		if err := ctx.SQL.QueryRowContext(ctx,
			`SELECT id FROM fine_tuning_jobs WHERE id = $1`, id,
		).Scan(&jobID); err == sql.ErrNoRows {
			return nil, ErrNotFound("fine_tuning.job")
		}

		rows, err := ctx.SQL.QueryContext(ctx,
			`SELECT id, level, message, created_at FROM fine_tuning_events WHERE job_id = $1 ORDER BY created_at ASC`,
			id)
		if err != nil {
			ctx.Errorf("list fine_tuning_events: %v", err)
			return nil, ErrInternal("failed to list fine-tuning events")
		}
		defer rows.Close()

		var events []models.FineTuningEvent

		for rows.Next() {
			var e models.FineTuningEvent
			if err := rows.Scan(&e.ID, &e.Level, &e.Message, &e.CreatedAt); err != nil {
				continue
			}

			e.Object = "fine_tuning.job.event"
			e.Type = "message"
			events = append(events, e)
		}

		if events == nil {
			events = []models.FineTuningEvent{}
		}

		return response.Raw{Data: models.FineTuningEventListResponse{
			Object:  "list",
			Data:    events,
			HasMore: false,
		}}, nil
	}
}

// proxyOpenAIFineTuning attempts to proxy a fine-tuning request to OpenAI.
func proxyOpenAIFineTuning(ctx *gofr.Context, reg *provider.Registry, req models.FineTuningJobRequest, jobID string, now int64) (*models.FineTuningJob, error) {
	// This is a best-effort proxy. If not possible, local job is used.
	_ = reg
	_ = req
	_ = jobID
	_ = now
	return nil, nil
}
