package node

import (
	"context"
	"errors"
	"sync"
)

// ErrQueueFull is returned when the worker pool's queue is full.
var ErrQueueFull = errors.New("worker pool queue is full")

// WorkerPoolNode processes tasks using a fixed pool of background worker goroutines.
type WorkerPoolNode struct {
	*BaseNode
	workers int
	queue   chan []byte
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewWorkerPoolNode creates a new WorkerPoolNode.
func NewWorkerPoolNode(id string, workers int, queueSize int) *WorkerPoolNode {
	ctx, cancel := context.WithCancel(context.Background())
	wp := &WorkerPoolNode{
		BaseNode: NewBaseNode(id),
		workers:  workers,
		queue:    make(chan []byte, queueSize),
		ctx:      ctx,
		cancel:   cancel,
	}

	return wp
}

// Create initializes the node and starts the background workers.
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

// worker constantly pulls tasks from the queue and processes them using the BaseNode's Process.
func (wp *WorkerPoolNode) worker() {
	defer wp.wg.Done()

	for {
		select {
		case <-wp.ctx.Done():
			return
		case task, ok := <-wp.queue:
			if !ok {
				return
			}
			// Process using the base process function to ensure middlewares are applied
			_, _ = wp.BaseNode.Process(task)
		}
	}
}

// Process queues the task for a background worker. It returns ErrQueueFull if the queue is full.
func (wp *WorkerPoolNode) Process(input []byte) ([]byte, error) {
	// Clone input to prevent data races since it will be processed asynchronously
	inputCopy := make([]byte, len(input))
	copy(inputCopy, input)

	// Fast fail if shutting down
	select {
	case <-wp.ctx.Done():
		return nil, errors.New("worker pool is shutting down")
	default:
	}

	select {
	case <-wp.ctx.Done():
		return nil, errors.New("worker pool is shutting down")
	case wp.queue <- inputCopy:
		// Task successfully queued. Background workers don't return responses directly to the caller.
		return nil, nil
	default:
		return nil, ErrQueueFull
	}
}

// Delete gracefully shuts down the worker pool by canceling context and waiting for workers to exit.
func (wp *WorkerPoolNode) Delete() error {
	// Signal workers to stop
	wp.cancel()
	// Wait for workers to finish
	wp.wg.Wait()
	// Safe to close queue now that all workers and potential Process senders are stopped or exiting
	// In some concurrent scenarios we might not close to avoid panics from late senders,
	// but context cancellation prevents late senders from writing to the channel.

	// Drain the queue
drainLoop:
	for {
		select {
		case <-wp.queue:
		default:
			break drainLoop
		}
	}

	// Intentionally not closing the queue to prevent panics from slow senders
	// that evaluated the select statement before ctx was fully cancelled.
	// The channel will be garbage collected.

	return wp.BaseNode.Delete()
}
