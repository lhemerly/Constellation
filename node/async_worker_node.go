package node

import (
	"context"
	"errors"
	"sync"
)

var (
	// ErrQueueFull is returned when the AsyncWorkerNode's internal queue is at capacity.
	ErrQueueFull = errors.New("async worker queue is full")
	// ErrNodeDeleted is returned when attempting to process on a deleted node.
	ErrNodeDeleted = errors.New("node is deleted")
)

// AsyncWorkerNode extends BaseNode to process inputs asynchronously using a worker pool.
type AsyncWorkerNode struct {
	*BaseNode
	jobChan    chan []byte
	workers    int
	wg         sync.WaitGroup
	ctx        context.Context
	cancelFunc context.CancelFunc
}

// NewAsyncWorkerNode creates a new AsyncWorkerNode with the given ID, number of workers, and queue capacity.
func NewAsyncWorkerNode(id string, workers int, queueCapacity int) *AsyncWorkerNode {
	ctx, cancel := context.WithCancel(context.Background())
	a := &AsyncWorkerNode{
		BaseNode:   NewBaseNode(id),
		jobChan:    make(chan []byte, queueCapacity),
		workers:    workers,
		ctx:        ctx,
		cancelFunc: cancel,
	}

	// We do not use a custom base process here, we want to intercept Process itself
	// so it acts as fire-and-forget but executes the full BaseNode.Process in the worker.
	return a
}

// Create initializes the worker pool.
func (a *AsyncWorkerNode) Create() error {
	if err := a.BaseNode.Create(); err != nil {
		return err
	}

	for i := 0; i < a.workers; i++ {
		a.wg.Add(1)
		go a.worker()
	}
	return nil
}

// Delete gracefully shuts down the worker pool and cleans up.
func (a *AsyncWorkerNode) Delete() error {
	a.cancelFunc() // Signal workers to stop
	a.wg.Wait()    // Wait for all workers to finish their current tasks

	// Drain any remaining jobs in the channel non-blockingly
drainLoop:
	for {
		select {
		case <-a.jobChan:
		default:
			break drainLoop
		}
	}

	return a.BaseNode.Delete()
}

// worker represents a background goroutine that processes jobs.
func (a *AsyncWorkerNode) worker() {
	defer a.wg.Done()
	for {
		select {
		case <-a.ctx.Done():
			return
		case job := <-a.jobChan:
			// Process job via the full BaseNode processing chain (middlewares, etc)
			_, _ = a.BaseNode.Process(job)
		}
	}
}

// Process adds the input to the job queue for asynchronous processing.
// It returns immediately. If the queue is full, it returns ErrQueueFull.
func (a *AsyncWorkerNode) Process(input []byte) ([]byte, error) {
	select {
	case <-a.ctx.Done():
		return nil, ErrNodeDeleted
	default:
	}

	// Try to add the job to the channel without blocking
	select {
	case <-a.ctx.Done():
		return nil, ErrNodeDeleted
	case a.jobChan <- input:
		return nil, nil // Fire and forget, no direct output
	default:
		return nil, ErrQueueFull
	}
}
