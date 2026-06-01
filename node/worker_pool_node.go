package node

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrWorkerPoolClosed = errors.New("worker pool is closed")
)

type workerJob struct {
	input      []byte
	resultChan chan<- workerResult
}

type workerResult struct {
	output []byte
	err    error
}

// WorkerPoolNode uses a bounded pool of goroutines to process inputs concurrently.
type WorkerPoolNode struct {
	*BaseNode
	workers    int
	jobQueue   chan workerJob
	ctx        context.Context
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
}

// NewWorkerPoolNode creates a new WorkerPoolNode with the specified number of workers.
func NewWorkerPoolNode(id string, workers int, queueSize int) *WorkerPoolNode {
	if workers <= 0 {
		workers = 1
	}
	if queueSize < 0 {
		queueSize = 0
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &WorkerPoolNode{
		BaseNode:   NewBaseNode(id),
		workers:    workers,
		jobQueue:   make(chan workerJob, queueSize),
		ctx:        ctx,
		cancelFunc: cancel,
	}
}

// Create initializes the worker pool node and starts the worker goroutines.
func (wp *WorkerPoolNode) Create() error {
	if err := wp.BaseNode.Create(); err != nil {
		return err
	}

	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.workerLoop()
	}

	return nil
}

// workerLoop continuously processes jobs from the jobQueue until the context is canceled.
func (wp *WorkerPoolNode) workerLoop() {
	defer wp.wg.Done()

	for {
		select {
		case <-wp.ctx.Done():
			return
		case job, ok := <-wp.jobQueue:
			if !ok {
				return
			}
			// Call BaseNode.Process to trigger middlewares, eventCounter, and processFunc
			output, err := wp.BaseNode.Process(job.input)
			job.resultChan <- workerResult{output: output, err: err}
		}
	}
}

// Process submits an input to the worker pool and waits for the result.
func (wp *WorkerPoolNode) Process(input []byte) ([]byte, error) {
	resultChan := make(chan workerResult, 1)
	job := workerJob{
		input:      input,
		resultChan: resultChan,
	}

	// Prioritize context cancellation to avoid hanging
	select {
	case <-wp.ctx.Done():
		return nil, ErrWorkerPoolClosed
	default:
	}

	select {
	case wp.jobQueue <- job:
		// Wait for the worker to process the job
		select {
		case res := <-resultChan:
			return res.output, res.err
		case <-wp.ctx.Done():
			return nil, ErrWorkerPoolClosed
		}
	case <-wp.ctx.Done():
		return nil, ErrWorkerPoolClosed
	}
}

// Delete gracefully shuts down the worker pool and cleans up resources.
func (wp *WorkerPoolNode) Delete() error {
	// Signal workers to stop
	wp.cancelFunc()

	// Wait for all workers to finish their current jobs
	wp.wg.Wait()

	// Drain any pending jobs in the queue
	close(wp.jobQueue)
	for job := range wp.jobQueue {
		job.resultChan <- workerResult{output: nil, err: ErrWorkerPoolClosed}
	}

	return wp.BaseNode.Delete()
}
