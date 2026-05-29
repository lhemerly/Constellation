package node

import (
	"context"
	"errors"
	"sync"
)

var ErrWorkerPoolDeleted = errors.New("worker pool deleted")

type job struct {
	input  []byte
	result chan<- jobResult
}

type jobResult struct {
	output []byte
	err    error
}

// WorkerPoolNode uses a fixed number of goroutines to process requests concurrently.
type WorkerPoolNode struct {
	*BaseNode
	workers int
	jobs    chan job
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewWorkerPoolNode creates a new WorkerPoolNode with the specified number of workers.
func NewWorkerPoolNode(id string, workers int) *WorkerPoolNode {
	ctx, cancel := context.WithCancel(context.Background())
	wp := &WorkerPoolNode{
		BaseNode: NewBaseNode(id),
		workers:  workers,
		jobs:     make(chan job),
		ctx:      ctx,
		cancel:   cancel,
	}

	return wp
}

// Create spins up the worker goroutines.
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

// worker constantly listens for jobs and processes them.
func (wp *WorkerPoolNode) worker() {
	defer wp.wg.Done()
	for {
		select {
		case <-wp.ctx.Done():
			return
		case j, ok := <-wp.jobs:
			if !ok {
				return
			}

			// Use the base node process to run middlewares and event tracking
			output, err := wp.BaseNode.Process(j.input)

			// Return result
			j.result <- jobResult{output: output, err: err}
		}
	}
}

// Process enqueues the job and waits for the worker to finish.
func (wp *WorkerPoolNode) Process(input []byte) ([]byte, error) {
	resultCh := make(chan jobResult, 1)

	// Create job struct
	j := job{
		input:  input,
		result: resultCh,
	}

	// Try sending job or return if ctx is canceled (node deleted)
	select {
	case <-wp.ctx.Done():
		return nil, ErrWorkerPoolDeleted
	default:
		select {
		case <-wp.ctx.Done():
			return nil, ErrWorkerPoolDeleted
		case wp.jobs <- j:
		}
	}

	// Wait for result
	select {
	case <-wp.ctx.Done():
		return nil, ErrWorkerPoolDeleted
	default:
		select {
		case <-wp.ctx.Done():
			return nil, ErrWorkerPoolDeleted
		case res := <-resultCh:
			return res.output, res.err
		}
	}
}

// Delete gracefully shuts down all workers.
func (wp *WorkerPoolNode) Delete() error {
	// Cancel context to stop workers and waiting requests
	wp.cancel()

	// Wait for all workers to finish
	wp.wg.Wait()

	return wp.BaseNode.Delete()
}
