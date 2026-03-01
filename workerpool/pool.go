package workerpool

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"gofr.dev/pkg/gofr"
)

// Task represents a unit of work to be executed by the worker pool.
type Task struct {
	Fn   func(ctx context.Context)
	Info string // optional metadata for logging
}

// NewTask creates a task with the given function and optional info string.
func NewTask(fn func(ctx context.Context), info ...string) Task {
	t := Task{Fn: fn}
	if len(info) > 0 {
		t.Info = info[0]
	}

	return t
}

// PoolConfig holds configuration for creating a WorkerPool.
type PoolConfig struct {
	Name        string
	Workers     int
	QueueSize   int
	TaskTimeout time.Duration
	App         *gofr.App // optional: enables GoFr metrics
}

// WorkerPool manages a fixed set of worker goroutines processing tasks from a buffered channel.
type WorkerPool struct {
	name        string
	workers     int
	queueSize   int
	taskTimeout time.Duration
	app         *gofr.App

	tasks   chan Task
	wg      sync.WaitGroup
	cancel  context.CancelFunc
	ctx     context.Context
	started atomic.Bool
	closing atomic.Bool

	// Metrics counters
	completed atomic.Int64
	timedOut  atomic.Int64
	panicked  atomic.Int64
}

// NewWorkerPool creates a new pool with the given configuration. Call Start() to begin processing.
func NewWorkerPool(cfg PoolConfig) (*WorkerPool, error) {
	if cfg.Workers <= 0 {
		return nil, errors.New("workerpool: workers must be > 0")
	}

	if cfg.QueueSize <= 0 {
		return nil, errors.New("workerpool: queue size must be > 0")
	}

	if cfg.Name == "" {
		cfg.Name = "default"
	}

	if cfg.TaskTimeout <= 0 {
		cfg.TaskTimeout = 60 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &WorkerPool{
		name:        cfg.Name,
		workers:     cfg.Workers,
		queueSize:   cfg.QueueSize,
		taskTimeout: cfg.TaskTimeout,
		app:         cfg.App,
		tasks:       make(chan Task, cfg.QueueSize),
		ctx:         ctx,
		cancel:      cancel,
	}, nil
}

// Start launches the worker goroutines. Returns an error if already started.
func (wp *WorkerPool) Start() error {
	if wp.started.Load() {
		return errors.New("workerpool: already started")
	}

	wp.started.Store(true)

	for range wp.workers {
		wp.wg.Add(1)

		go wp.worker()
	}

	return nil
}

// worker is the main loop for each worker goroutine.
func (wp *WorkerPool) worker() {
	defer wp.wg.Done()

	for task := range wp.tasks {
		wp.executeTask(task)
	}
}

// executeTask runs a single task with timeout and panic recovery.
func (wp *WorkerPool) executeTask(t Task) {
	taskCtx, taskCancel := context.WithTimeout(wp.ctx, wp.taskTimeout)
	defer taskCancel()

	done := make(chan struct{})

	go func() {
		defer func() {
			if r := recover(); r != nil {
				wp.panicked.Add(1)

				if wp.app != nil {
					wp.app.Logger().Errorf("workerpool %s: task panicked: %v (info: %s)", wp.name, r, t.Info)
				}
			}

			close(done)
		}()

		t.Fn(taskCtx)
	}()

	select {
	case <-done:
		// Check if the context timed out (task may have finished after timeout)
		if taskCtx.Err() == context.DeadlineExceeded {
			wp.timedOut.Add(1)
		} else {
			wp.completed.Add(1)
		}
	case <-taskCtx.Done():
		wp.timedOut.Add(1)
		// Wait for the goroutine to finish (panic recovery needs to run)
		<-done
	}
}

// Submit adds a task to the queue. Returns an error if the queue is full or the pool is shutting down.
func (wp *WorkerPool) Submit(t Task) error {
	if wp.closing.Load() {
		return errors.New("workerpool: pool is shutting down")
	}

	if !wp.started.Load() {
		return errors.New("workerpool: pool not started")
	}

	select {
	case wp.tasks <- t:
		return nil
	default:
		return fmt.Errorf("workerpool: queue full (capacity %d)", wp.queueSize)
	}
}

// SubmitBlocking adds a task to the queue, blocking until space is available or ctx is cancelled.
func (wp *WorkerPool) SubmitBlocking(ctx context.Context, t Task) error {
	if wp.closing.Load() {
		return errors.New("workerpool: pool is shutting down")
	}

	if !wp.started.Load() {
		return errors.New("workerpool: pool not started")
	}

	select {
	case wp.tasks <- t:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ShutdownGraceful closes the task channel and waits for all in-flight tasks to complete.
// If ctx is cancelled before workers finish, it force-cancels remaining tasks.
func (wp *WorkerPool) ShutdownGraceful(ctx context.Context) error {
	if !wp.started.Load() {
		return nil
	}

	wp.closing.Store(true)
	close(wp.tasks)

	done := make(chan struct{})

	go func() {
		wp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		wp.cancel()
		wp.wg.Wait()

		return ctx.Err()
	}
}

// Shutdown immediately cancels all in-flight tasks and waits for workers to exit.
func (wp *WorkerPool) Shutdown(ctx context.Context) error {
	if !wp.started.Load() {
		return nil
	}

	wp.closing.Store(true)
	wp.cancel()
	close(wp.tasks)

	done := make(chan struct{})

	go func() {
		wp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stats returns a snapshot of pool metrics.
func (wp *WorkerPool) Stats() map[string]any {
	queueDepth := len(wp.tasks)

	return map[string]any{
		"name":              wp.name,
		"workers":           wp.workers,
		"queue_depth":       queueDepth,
		"queue_capacity":    wp.queueSize,
		"queue_utilization": float64(queueDepth) / float64(wp.queueSize),
		"tasks_completed":   wp.completed.Load(),
		"tasks_timed_out":   wp.timedOut.Load(),
		"tasks_panicked":    wp.panicked.Load(),
	}
}

// IsHealthy returns true if the pool is started and not shutting down.
func (wp *WorkerPool) IsHealthy() bool {
	return wp.started.Load() && !wp.closing.Load()
}
