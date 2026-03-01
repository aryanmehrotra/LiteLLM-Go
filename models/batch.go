package models

import "encoding/json"

// BatchRequest represents a single request within a batch submission.
type BatchRequest struct {
	CustomID string          `json:"custom_id"`
	Method   string          `json:"method"`
	URL      string          `json:"url"`
	Body     json.RawMessage `json:"body"`
}

// BatchSubmitRequest is the request body for POST /v1/batches.
type BatchSubmitRequest struct {
	Requests []BatchRequest `json:"requests"`
}

// BatchResponse is the response for batch status queries.
type BatchResponse struct {
	ID                string  `json:"id"`
	Status            string  `json:"status"`
	TotalRequests     int     `json:"total_requests"`
	CompletedRequests int     `json:"completed_requests"`
	FailedRequests    int     `json:"failed_requests"`
	CreatedAt         string  `json:"created_at"`
	CompletedAt       *string `json:"completed_at,omitempty"`
}

// BatchResultItem holds the result of a single batch item.
type BatchResultItem struct {
	CustomID   string          `json:"custom_id"`
	StatusCode int             `json:"status_code"`
	Body       json.RawMessage `json:"body,omitempty"`
	Error      string          `json:"error,omitempty"`
}
