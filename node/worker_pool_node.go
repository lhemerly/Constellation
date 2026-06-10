package node

import (
	"context"
	"errors"
	"sync"
)

// WorkerPoolNode manages a fixed pool of background worker goroutines to process requests concurrently.
// This limits the number of active `Process` calls that run concurrently and queues excess requests.
type WorkerPoolNode struct {
	*BaseNode
	workers    int
	queueSize  int
	jobQueue   chan workerJob
	ctx        context.Context
	cancel     context.CancelFunc
	workerWg   sync.WaitGroup
}

type workerJob struct {
	input      []byte
	resultChan chan workerResult
}

type workerResult struct {
	output []byte
	err    error
}

// NewWorkerPoolNode creates a new WorkerPoolNode with the specified number of workers and queue size.
func NewWorkerPoolNode(id string, workers int, queueSize int) *WorkerPoolNode {
	if workers <= 0 {
		panic("WorkerPoolNode requires at least 1 worker")
	}
	if queueSize <= 0 {
		panic("WorkerPoolNode requires a queue size of at least 1")
	}

	ctx, cancel := context.WithCancel(context.Background())

	wpNode := &WorkerPoolNode{
		BaseNode:  NewBaseNode(id),
		workers:   workers,
		queueSize: queueSize,
		jobQueue:  make(chan workerJob, queueSize),
		ctx:       ctx,
		cancel:    cancel,
	}

	// Override Process directly instead of using SetProcessFunc,
	// so users can still set their own business logic via SetProcessFunc,
	// while we handle the queuing mechanism around it.
	return wpNode
}

// Create initializes the node and starts the background workers.
func (wp *WorkerPoolNode) Create() error {
	if err := wp.BaseNode.Create(); err != nil {
		return err
	}

	for i := 0; i < wp.workers; i++ {
		wp.workerWg.Add(1)
		go wp.workerLoop()
	}

	return nil
}

// workerLoop continuously pulls jobs from the queue and processes them until context is canceled.
func (wp *WorkerPoolNode) workerLoop() {
	defer wp.workerWg.Done()

	for {
		select {
		case <-wp.ctx.Done():
			return
		case job, ok := <-wp.jobQueue:
			if !ok {
				return
			}
			// Call the base node's Process to run the user's processFunc (with middlewares)
			out, err := wp.BaseNode.Process(job.input)
			job.resultChan <- workerResult{output: out, err: err}
		}
	}
}

// Process enqueues the input for processing by a worker.
// Returns an error if the node is shutting down or the queue is full.
func (wp *WorkerPoolNode) Process(input []byte) ([]byte, error) {
	resultChan := make(chan workerResult, 1)
	job := workerJob{
		input:      input,
		resultChan: resultChan,
	}

	// Try to enqueue the job, checking for context cancellation first
	select {
	case <-wp.ctx.Done():
		return nil, errors.New("node is shutting down")
	default:
		select {
		case <-wp.ctx.Done():
			return nil, errors.New("node is shutting down")
		case wp.jobQueue <- job:
			// Wait for the result
			select {
			case <-wp.ctx.Done():
				return nil, errors.New("node is shutting down while waiting for result")
			case res := <-resultChan:
				return res.output, res.err
			}
		default:
			return nil, errors.New("worker pool queue is full")
		}
	}
}

// Delete cleanly shuts down the worker pool, waiting for active jobs to finish
// and draining any queued requests.
func (wp *WorkerPoolNode) Delete() error {
	wp.cancel()     // Signal workers to stop accepting new jobs
	wp.workerWg.Wait() // Wait for active workers to finish their current job

	// Drain the remaining queue
	close(wp.jobQueue)
	for job := range wp.jobQueue {
		job.resultChan <- workerResult{nil, errors.New("node deleted before processing")}
	}

	return wp.BaseNode.Delete()
}
