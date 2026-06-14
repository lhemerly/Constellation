package node

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrWorkerPoolQueueFull = errors.New("worker pool queue is full")
	ErrWorkerPoolDeleted   = errors.New("worker pool deleted")
)

type poolJob struct {
	input      []byte
	resultChan chan poolResult
	ctx        context.Context
}

type poolResult struct {
	output []byte
	err    error
}

// WorkerPoolNode manages a pool of worker goroutines to process requests concurrently,
// up to a specified limit. Requests that exceed the queue capacity return an error immediately.
type WorkerPoolNode struct {
	*BaseNode
	numWorkers int
	queueSize  int
	jobQueue   chan poolJob
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewWorkerPoolNode creates a new WorkerPoolNode.
func NewWorkerPoolNode(id string, numWorkers, queueSize int) *WorkerPoolNode {
	ctx, cancel := context.WithCancel(context.Background())
	wp := &WorkerPoolNode{
		BaseNode:   NewBaseNode(id),
		numWorkers: numWorkers,
		queueSize:  queueSize,
		jobQueue:   make(chan poolJob, queueSize),
		ctx:        ctx,
		cancel:     cancel,
	}

	return wp
}

// Create initializes the node and starts the worker pool.
func (wp *WorkerPoolNode) Create() error {
	if err := wp.BaseNode.Create(); err != nil {
		return err
	}

	for i := 0; i < wp.numWorkers; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}
	return nil
}

// Delete shuts down the node, cancels the workers, and drains the queue.
func (wp *WorkerPoolNode) Delete() error {
	var errs []error
	if err := wp.BaseNode.Delete(); err != nil {
		errs = append(errs, err)
	}

	// Signal workers to exit
	wp.cancel()

	// Wait for all workers to finish
	wp.wg.Wait()

	// Drain remaining jobs in the queue
drainLoop:
	for {
		select {
		case job := <-wp.jobQueue:
			job.resultChan <- poolResult{nil, ErrWorkerPoolDeleted}
		default:
			break drainLoop
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// worker runs continuously, picking up jobs from the queue until the context is canceled.
func (wp *WorkerPoolNode) worker() {
	defer wp.wg.Done()

	for {
		select {
		case <-wp.ctx.Done():
			return
		case job := <-wp.jobQueue:
			// Check if the individual job context is already done
			if job.ctx != nil && job.ctx.Err() != nil {
				job.resultChan <- poolResult{nil, job.ctx.Err()}
				continue
			}

			// Process via BaseNode to handle custom SetProcessFunc and middlewares
			res, err := wp.BaseNode.Process(job.input)
			job.resultChan <- poolResult{res, err}
		}
	}
}

// Process overrides the BaseNode Process method to queue the input for the worker pool.
func (wp *WorkerPoolNode) Process(input []byte) ([]byte, error) {
	// First check if the pool is already deleted
	select {
	case <-wp.ctx.Done():
		return nil, ErrWorkerPoolDeleted
	default:
	}

	// Create job and enqueue
	resultChan := make(chan poolResult, 1)
	job := poolJob{
		input:      input,
		resultChan: resultChan,
		ctx:        context.Background(),
	}

	select {
	case wp.jobQueue <- job:
	default:
		return nil, ErrWorkerPoolQueueFull
	}

	// Wait for result or pool deletion
	select {
	case res := <-resultChan:
		return res.output, res.err
	case <-wp.ctx.Done():
		return nil, ErrWorkerPoolDeleted
	}
}
