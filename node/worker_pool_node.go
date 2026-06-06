package node

import (
	"context"
	"errors"
	"sync"
)

var ErrWorkerPoolClosed = errors.New("worker pool is closed")

// WorkerPoolNode processes requests concurrently using a fixed-size worker pool.
type WorkerPoolNode struct {
	*BaseNode
	workers   int
	queueSize int
	jobQueue  chan *job
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.Mutex // protects closed flag
	closed    bool
}

type job struct {
	input  []byte
	result chan *jobResult
}

type jobResult struct {
	output []byte
	err    error
}

// NewWorkerPoolNode creates a new WorkerPoolNode.
func NewWorkerPoolNode(id string, workers int, queueSize int) *WorkerPoolNode {
	if workers <= 0 {
		workers = 1
	}
	if queueSize <= 0 {
		queueSize = 100
	}

	ctx, cancel := context.WithCancel(context.Background())
	wp := &WorkerPoolNode{
		BaseNode:  NewBaseNode(id),
		workers:   workers,
		queueSize: queueSize,
		jobQueue:  make(chan *job, queueSize),
		ctx:       ctx,
		cancel:    cancel,
	}

	return wp
}

// Create initializes the worker pool and starts the worker goroutines.
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

// worker continuously picks up jobs from the jobQueue and processes them.
func (wp *WorkerPoolNode) worker() {
	defer wp.wg.Done()

	for {
		select {
		case <-wp.ctx.Done():
			return
		case j, ok := <-wp.jobQueue:
			if !ok {
				return
			}

			// Use the BaseNode Process method to process the input
			// This automatically handles middlewares, custom process func, and event counters
			out, err := wp.BaseNode.Process(j.input)
			j.result <- &jobResult{output: out, err: err}
		}
	}
}

// Process overrides the BaseNode Process method to queue the request.
// We must override Process directly because users will overwrite SetProcessFunc
// with their own business logic, and we need our worker logic to run first.
func (wp *WorkerPoolNode) Process(input []byte) ([]byte, error) {
	wp.mu.Lock()
	if wp.closed {
		wp.mu.Unlock()
		return nil, ErrWorkerPoolClosed
	}
	wp.mu.Unlock()

	j := &job{
		input:  input,
		result: make(chan *jobResult, 1),
	}

	select {
	case <-wp.ctx.Done():
		return nil, ErrWorkerPoolClosed
	case wp.jobQueue <- j:
		// Job queued successfully
	}

	select {
	case <-wp.ctx.Done():
		return nil, ErrWorkerPoolClosed
	case res := <-j.result:
		return res.output, res.err
	}
}

// Delete gracefully shuts down the worker pool.
func (wp *WorkerPoolNode) Delete() error {
	wp.mu.Lock()
	if wp.closed {
		wp.mu.Unlock()
		return nil
	}
	wp.closed = true
	wp.mu.Unlock()

	// Signal workers to stop
	wp.cancel()

	// Wait for workers to finish current jobs
	wp.wg.Wait()

	// Drain any remaining jobs in the queue
drainLoop:
	for {
		select {
		case j := <-wp.jobQueue:
			j.result <- &jobResult{output: nil, err: ErrWorkerPoolClosed}
		default:
			break drainLoop
		}
	}

	return wp.BaseNode.Delete()
}
