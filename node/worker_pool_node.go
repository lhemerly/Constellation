package node

import (
	"context"
	"errors"
	"sync"
)

// ErrWorkerPoolFull is returned when the worker pool queue is full.
var ErrWorkerPoolFull = errors.New("worker pool queue is full")

// ErrWorkerPoolDeleted is returned when a process is attempted on a deleted pool.
var ErrWorkerPoolDeleted = errors.New("worker pool has been deleted")

type job struct {
	input      []byte
	resultChan chan result
}

type result struct {
	output []byte
	err    error
}

// WorkerPoolNode extends BaseNode to process requests concurrently
// using a bounded pool of worker goroutines and a job queue.
type WorkerPoolNode struct {
	*BaseNode
	workers    int
	jobChan    chan job
	wg         sync.WaitGroup
	ctx        context.Context
	cancelFunc context.CancelFunc
}

// NewWorkerPoolNode creates a new WorkerPoolNode.
func NewWorkerPoolNode(id string, workers int, queueSize int) *WorkerPoolNode {
	ctx, cancel := context.WithCancel(context.Background())
	wp := &WorkerPoolNode{
		BaseNode:   NewBaseNode(id),
		workers:    workers,
		jobChan:    make(chan job, queueSize),
		ctx:        ctx,
		cancelFunc: cancel,
	}

	return wp
}

// Create initializes the node and starts the worker pool.
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

// Delete cleanly shuts down the worker pool, preventing new jobs and draining the queue.
func (wp *WorkerPoolNode) Delete() error {
	wp.cancelFunc()
	wp.wg.Wait()

	// Drain any remaining jobs in the channel to prevent hanging callers
DrainLoop:
	for {
		select {
		case j := <-wp.jobChan:
			j.resultChan <- result{nil, ErrWorkerPoolDeleted}
		default:
			break DrainLoop
		}
	}

	return wp.BaseNode.Delete()
}

func (wp *WorkerPoolNode) worker() {
	defer wp.wg.Done()
	for {
		select {
		case <-wp.ctx.Done():
			return
		case j := <-wp.jobChan:
			// Process via BaseNode to execute middlewares and custom logic
			out, err := wp.BaseNode.Process(j.input)
			j.resultChan <- result{out, err}
		}
	}
}

// Process enqueues a job or returns an error if the queue is full or deleted.
func (wp *WorkerPoolNode) Process(input []byte) ([]byte, error) {
	// Check context first to avoid sending to channel if deleted
	select {
	case <-wp.ctx.Done():
		return nil, ErrWorkerPoolDeleted
	default:
	}

	resChan := make(chan result, 1)
	j := job{input: input, resultChan: resChan}

	select {
	case <-wp.ctx.Done():
		return nil, ErrWorkerPoolDeleted
	case wp.jobChan <- j:
		// Job queued successfully
	default:
		return nil, ErrWorkerPoolFull
	}

	// Wait for processing to complete or pool to be deleted
	select {
	case res := <-resChan:
		return res.output, res.err
	case <-wp.ctx.Done():
		return nil, ErrWorkerPoolDeleted
	}
}
