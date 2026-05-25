package node

import (
	"context"
	"errors"
	"sync"
)

var (
	// ErrWorkerPoolClosed is returned when attempting to process on a closed pool.
	ErrWorkerPoolClosed = errors.New("worker pool is closed")
)

// job represents a single unit of work for the worker pool.
type job struct {
	input []byte
	resp  chan result
	ctx   context.Context
}

// result represents the outcome of a processed job.
type result struct {
	output []byte
	err    error
}

// WorkerPoolNode processes messages concurrently using a pool of workers.
type WorkerPoolNode struct {
	*BaseNode
	workers     int
	jobQueue    chan job
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	closedMutex sync.RWMutex
	closed      bool
}

// NewWorkerPoolNode creates a new WorkerPoolNode with a specified number of workers
// and queue size.
func NewWorkerPoolNode(id string, workers int, queueSize int) *WorkerPoolNode {
	ctx, cancel := context.WithCancel(context.Background())
	wp := &WorkerPoolNode{
		BaseNode: NewBaseNode(id),
		workers:  workers,
		jobQueue: make(chan job, queueSize),
		ctx:      ctx,
		cancel:   cancel,
	}

	return wp
}

// Create initializes the worker pool, starting the worker goroutines.
func (wp *WorkerPoolNode) Create() error {
	if err := wp.BaseNode.Create(); err != nil {
		return err
	}
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
	return nil
}

// worker represents a single goroutine processing jobs from the queue.
func (wp *WorkerPoolNode) worker(id int) {
	defer wp.wg.Done()
	for {
		select {
		case <-wp.ctx.Done():
			return
		case j, ok := <-wp.jobQueue:
			if !ok {
				return
			}
			// Process input using the base node's logic to handle middlewares etc.
			out, err := wp.BaseNode.Process(j.input)
			select {
			case j.resp <- result{output: out, err: err}:
			case <-j.ctx.Done():
				// Caller gave up
			case <-wp.ctx.Done():
				// Pool is shutting down
			}
		}
	}
}

// Process sends input to the job queue and waits for a worker to process it.
func (wp *WorkerPoolNode) Process(input []byte) ([]byte, error) {
	wp.closedMutex.RLock()
	if wp.closed {
		wp.closedMutex.RUnlock()
		return nil, ErrWorkerPoolClosed
	}
	wp.closedMutex.RUnlock()

	// Create a response channel for this specific job
	respChan := make(chan result, 1)
	j := job{
		input: input,
		resp:  respChan,
		ctx:   context.Background(), // Can be improved to accept request context
	}

	// Try to enqueue the job
	select {
	case <-wp.ctx.Done():
		return nil, ErrWorkerPoolClosed
	case wp.jobQueue <- j:
		// Enqueued successfully
	}

	// Wait for the result
	select {
	case <-wp.ctx.Done():
		return nil, ErrWorkerPoolClosed
	case res := <-respChan:
		return res.output, res.err
	}
}

// Delete shuts down the worker pool and cleans up resources.
func (wp *WorkerPoolNode) Delete() error {
	wp.closedMutex.Lock()
	if wp.closed {
		wp.closedMutex.Unlock()
		return nil
	}
	wp.closed = true
	wp.cancel() // Signal workers to stop
	wp.closedMutex.Unlock()

	wp.wg.Wait() // Wait for all workers to exit completely

	// Drain any pending jobs that were in the channel (without closing it)
drainLoop:
	for {
		select {
		case j := <-wp.jobQueue:
			select {
			case j.resp <- result{err: ErrWorkerPoolClosed}:
			default:
			}
		default:
			break drainLoop
		}
	}

	return wp.BaseNode.Delete()
}
