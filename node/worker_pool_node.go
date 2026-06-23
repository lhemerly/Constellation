package node

import (
	"context"
	"errors"
	"sync"
)

// ErrQueueFull is returned when the worker pool's job queue is full.
var ErrQueueFull = errors.New("worker pool job queue is full")

// WorkerPoolNode uses a fixed pool of goroutines to process incoming data asynchronously.
type WorkerPoolNode struct {
	*BaseNode
	jobQueue chan []byte
	workers  int
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewWorkerPoolNode creates a new WorkerPoolNode with the specified number of workers and queue size.
func NewWorkerPoolNode(id string, workers int, queueSize int) *WorkerPoolNode {
	ctx, cancel := context.WithCancel(context.Background())
	wp := &WorkerPoolNode{
		BaseNode: NewBaseNode(id),
		jobQueue: make(chan []byte, queueSize),
		workers:  workers,
		ctx:      ctx,
		cancel:   cancel,
	}

	return wp
}

// Process overrides BaseNode.Process to handle non-blocking insertion of tasks into the pool.
// Middlewares and actual process logic will be executed by the workers.
func (wp *WorkerPoolNode) Process(input []byte) ([]byte, error) {
	// Clone input to prevent data races, since processing is async
	inputCopy := make([]byte, len(input))
	copy(inputCopy, input)

	select {
	case <-wp.ctx.Done():
		return nil, errors.New("worker pool is shutting down")
	case wp.jobQueue <- inputCopy:
		// Job successfully enqueued
		return nil, nil // Return nil, nil because the real result is processed asynchronously
	default:
		return nil, ErrQueueFull
	}
}

// Create starts the background worker goroutines.
func (wp *WorkerPoolNode) Create() error {
	if err := wp.BaseNode.Create(); err != nil {
		return err
	}

	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}
	return nil
}

// Delete gracefully shuts down the worker pool and cleans up.
func (wp *WorkerPoolNode) Delete() error {
	// Signal workers to stop
	wp.cancel()

	// Wait for workers to finish
	wp.wg.Wait()

	// Safely drain the job channel without closing it
drainLoop:
	for {
		select {
		case <-wp.jobQueue:
			// drain
		default:
			break drainLoop
		}
	}

	return wp.BaseNode.Delete()
}

// worker reads from the job queue and processes the input until context is cancelled.
func (wp *WorkerPoolNode) worker() {
	defer wp.wg.Done()
	for {
		select {
		case <-wp.ctx.Done():
			return
		case job := <-wp.jobQueue:
			// Note: This relies on BaseNode.Process to correctly invoke any middlewares and user logic.
			wp.BaseNode.Process(job)
		}
	}
}
