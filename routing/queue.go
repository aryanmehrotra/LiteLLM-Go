package routing

import (
	"fmt"
	"time"

	"examples/llm-gateway/models"
)

// QueuedRequest represents a request waiting in the queue.
type QueuedRequest struct {
	Request    models.ChatCompletionRequest
	Provider   string
	KeyID      string
	Priority   int
	EnqueuedAt time.Time
	ResultCh   chan QueueResult
}

// QueueResult carries the result back to the caller.
type QueueResult struct {
	Response *models.ChatCompletionResponse
	Err      error
}

// RequestQueue manages request queuing with an in-memory channel.
type RequestQueue struct {
	ch      chan *QueuedRequest
	Enabled bool
}

// NewRequestQueue creates a new request queue with the given buffer size.
func NewRequestQueue(bufferSize int, enabled bool) *RequestQueue {
	if bufferSize <= 0 {
		bufferSize = 100
	}

	return &RequestQueue{
		ch:      make(chan *QueuedRequest, bufferSize),
		Enabled: enabled,
	}
}

// Enqueue adds a request to the queue and returns a channel for the result.
func (q *RequestQueue) Enqueue(req *QueuedRequest) <-chan QueueResult {
	req.EnqueuedAt = time.Now()
	req.ResultCh = make(chan QueueResult, 1)

	select {
	case q.ch <- req:
	default:
		req.ResultCh <- QueueResult{Err: fmt.Errorf("request queue full")}
	}

	return req.ResultCh
}

// Dequeue returns the channel for consuming queued requests.
func (q *RequestQueue) Dequeue() <-chan *QueuedRequest {
	return q.ch
}
