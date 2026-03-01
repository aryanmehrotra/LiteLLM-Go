package workerpool

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWorkerPool_InvalidConfig(t *testing.T) {
	_, err := NewWorkerPool(PoolConfig{Workers: 0, QueueSize: 10})
	assert.Error(t, err)

	_, err = NewWorkerPool(PoolConfig{Workers: 2, QueueSize: 0})
	assert.Error(t, err)
}

func TestNewWorkerPool_Defaults(t *testing.T) {
	wp, err := NewWorkerPool(PoolConfig{Workers: 2, QueueSize: 10})
	require.NoError(t, err)
	assert.Equal(t, "default", wp.name)
	assert.Equal(t, 60*time.Second, wp.taskTimeout)
}

func TestWorkerPool_SubmitAndComplete(t *testing.T) {
	wp, err := NewWorkerPool(PoolConfig{
		Name:        "test",
		Workers:     2,
		QueueSize:   10,
		TaskTimeout: 5 * time.Second,
	})
	require.NoError(t, err)
	require.NoError(t, wp.Start())

	var counter atomic.Int64

	for range 5 {
		err := wp.Submit(NewTask(func(_ context.Context) {
			counter.Add(1)
		}, "increment"))
		require.NoError(t, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, wp.ShutdownGraceful(ctx))
	assert.Equal(t, int64(5), counter.Load())

	stats := wp.Stats()
	assert.Equal(t, int64(5), stats["tasks_completed"])
	assert.Equal(t, int64(0), stats["tasks_timed_out"])
	assert.Equal(t, int64(0), stats["tasks_panicked"])
}

func TestWorkerPool_QueueFull(t *testing.T) {
	wp, err := NewWorkerPool(PoolConfig{
		Name:        "full-test",
		Workers:     1,
		QueueSize:   1,
		TaskTimeout: 5 * time.Second,
	})
	require.NoError(t, err)
	require.NoError(t, wp.Start())

	// Block the single worker
	blocker := make(chan struct{})
	_ = wp.Submit(NewTask(func(_ context.Context) {
		<-blocker
	}))

	// Fill the queue
	time.Sleep(10 * time.Millisecond)
	_ = wp.Submit(NewTask(func(_ context.Context) {}, "fill"))

	// This should fail
	err = wp.Submit(NewTask(func(_ context.Context) {}, "overflow"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "queue full")

	close(blocker)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wp.ShutdownGraceful(ctx)
}

func TestWorkerPool_TaskTimeout(t *testing.T) {
	wp, err := NewWorkerPool(PoolConfig{
		Name:        "timeout-test",
		Workers:     1,
		QueueSize:   5,
		TaskTimeout: 50 * time.Millisecond,
	})
	require.NoError(t, err)
	require.NoError(t, wp.Start())

	_ = wp.Submit(NewTask(func(ctx context.Context) {
		<-ctx.Done() // Block until timeout
	}, "slow-task"))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, wp.ShutdownGraceful(ctx))

	stats := wp.Stats()
	assert.Equal(t, int64(1), stats["tasks_timed_out"])
}

func TestWorkerPool_PanicRecovery(t *testing.T) {
	wp, err := NewWorkerPool(PoolConfig{
		Name:        "panic-test",
		Workers:     1,
		QueueSize:   5,
		TaskTimeout: 5 * time.Second,
	})
	require.NoError(t, err)
	require.NoError(t, wp.Start())

	_ = wp.Submit(NewTask(func(_ context.Context) {
		panic("test panic")
	}, "panicking-task"))

	// Submit a normal task after the panic to verify worker continues
	var executed atomic.Bool
	_ = wp.Submit(NewTask(func(_ context.Context) {
		executed.Store(true)
	}, "after-panic"))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, wp.ShutdownGraceful(ctx))

	stats := wp.Stats()
	assert.Equal(t, int64(1), stats["tasks_panicked"])
	assert.True(t, executed.Load(), "worker should continue after panic")
}

func TestWorkerPool_DoubleStart(t *testing.T) {
	wp, err := NewWorkerPool(PoolConfig{Workers: 1, QueueSize: 5})
	require.NoError(t, err)
	require.NoError(t, wp.Start())

	err = wp.Start()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already started")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	wp.ShutdownGraceful(ctx)
}

func TestWorkerPool_SubmitBeforeStart(t *testing.T) {
	wp, err := NewWorkerPool(PoolConfig{Workers: 1, QueueSize: 5})
	require.NoError(t, err)

	err = wp.Submit(NewTask(func(_ context.Context) {}))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not started")
}

func TestWorkerPool_SubmitAfterShutdown(t *testing.T) {
	wp, err := NewWorkerPool(PoolConfig{Workers: 1, QueueSize: 5})
	require.NoError(t, err)
	require.NoError(t, wp.Start())

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	wp.ShutdownGraceful(ctx)

	err = wp.Submit(NewTask(func(_ context.Context) {}))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "shutting down")
}

func TestWorkerPool_SubmitBlocking(t *testing.T) {
	wp, err := NewWorkerPool(PoolConfig{
		Name:        "blocking-test",
		Workers:     1,
		QueueSize:   1,
		TaskTimeout: 5 * time.Second,
	})
	require.NoError(t, err)
	require.NoError(t, wp.Start())

	// Block the worker
	blocker := make(chan struct{})
	_ = wp.Submit(NewTask(func(_ context.Context) {
		<-blocker
	}))

	time.Sleep(10 * time.Millisecond)

	// Fill the queue
	_ = wp.Submit(NewTask(func(_ context.Context) {}))

	// Blocking submit with short timeout should fail
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err = wp.SubmitBlocking(ctx, NewTask(func(_ context.Context) {}))
	assert.Error(t, err)

	close(blocker)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	wp.ShutdownGraceful(shutdownCtx)
}

func TestWorkerPool_IsHealthy(t *testing.T) {
	wp, err := NewWorkerPool(PoolConfig{Workers: 1, QueueSize: 5})
	require.NoError(t, err)

	assert.False(t, wp.IsHealthy())

	require.NoError(t, wp.Start())
	assert.True(t, wp.IsHealthy())

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	wp.ShutdownGraceful(ctx)
	assert.False(t, wp.IsHealthy())
}

func TestWorkerPool_ImmediateShutdown(t *testing.T) {
	wp, err := NewWorkerPool(PoolConfig{
		Name:        "immediate-test",
		Workers:     2,
		QueueSize:   10,
		TaskTimeout: 30 * time.Second,
	})
	require.NoError(t, err)
	require.NoError(t, wp.Start())

	// Submit a slow task
	_ = wp.Submit(NewTask(func(ctx context.Context) {
		<-ctx.Done()
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = wp.Shutdown(ctx)
	assert.NoError(t, err)
}
