package models

// FineTuningJob represents a fine-tuning job (OpenAI-compatible).
type FineTuningJob struct {
	ID              string           `json:"id"`
	Object          string           `json:"object"` // "fine_tuning.job"
	CreatedAt       int64            `json:"created_at"`
	FinishedAt      *int64           `json:"finished_at,omitempty"`
	Model           string           `json:"model"`
	FineTunedModel  *string          `json:"fine_tuned_model,omitempty"`
	OrganizationID  string           `json:"organization_id,omitempty"`
	Status          string           `json:"status"` // "validating_files", "queued", "running", "succeeded", "failed", "cancelled"
	TrainingFile    string           `json:"training_file"`
	ValidationFile  *string          `json:"validation_file,omitempty"`
	Hyperparameters *Hyperparameters `json:"hyperparameters,omitempty"`
	TrainedTokens   *int             `json:"trained_tokens,omitempty"`
	Error           *FineTuningError `json:"error,omitempty"`
	Suffix          *string          `json:"suffix,omitempty"`
}

// Hyperparameters holds training configuration for fine-tuning.
type Hyperparameters struct {
	NEpochs              any `json:"n_epochs,omitempty"`         // int or "auto"
	BatchSize            any `json:"batch_size,omitempty"`       // int or "auto"
	LearningRateMultiple any `json:"learning_rate_multiplier,omitempty"` // float or "auto"
}

// FineTuningError holds error details for failed fine-tuning jobs.
type FineTuningError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Param   string `json:"param,omitempty"`
}

// FineTuningJobRequest is the request body for POST /v1/fine_tuning/jobs.
type FineTuningJobRequest struct {
	Model          string           `json:"model"`
	TrainingFile   string           `json:"training_file"`
	ValidationFile string           `json:"validation_file,omitempty"`
	Hyperparameters *Hyperparameters `json:"hyperparameters,omitempty"`
	Suffix         string           `json:"suffix,omitempty"`
}

// FineTuningJobListResponse is the response for GET /v1/fine_tuning/jobs.
type FineTuningJobListResponse struct {
	Object  string          `json:"object"` // "list"
	Data    []FineTuningJob `json:"data"`
	HasMore bool            `json:"has_more"`
}

// FineTuningEvent represents a single event from a fine-tuning job.
type FineTuningEvent struct {
	ID        string `json:"id"`
	Object    string `json:"object"` // "fine_tuning.job.event"
	CreatedAt int64  `json:"created_at"`
	Level     string `json:"level"` // "info", "warn", "error"
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
	Type      string `json:"type"` // "message", "metrics"
}

// FineTuningEventListResponse is the response for GET /v1/fine_tuning/jobs/{id}/events.
type FineTuningEventListResponse struct {
	Object  string            `json:"object"` // "list"
	Data    []FineTuningEvent `json:"data"`
	HasMore bool              `json:"has_more"`
}
