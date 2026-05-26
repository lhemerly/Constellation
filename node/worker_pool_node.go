package node

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrWorkerPoolDeleted = errors.New("worker pool deleted")
)

type job struct {
	input      []byte
	resultChan chan<- workerResult
}

type workerResult struct {
	output []byte
	err    error
}

// WorkerPoolNode processes requests concurrently using a fixed-size pool of goroutines.
type WorkerPoolNode struct {
	*BaseNode
	numWorkers int
	jobChan    chan job
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	mu         sync.RWMutex
	deleted    bool
}

// NewWorkerPoolNode creates a new WorkerPoolNode with a given ID and number of workers.
func NewWorkerPoolNode(id string, numWorkers int) *WorkerPoolNode {
	if numWorkers <= 0 {
		numWorkers = 1
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPoolNode{
		BaseNode:   NewBaseNode(id),
		numWorkers: numWorkers,
		jobChan:    make(chan job, numWorkers),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Create initializes the node and starts the worker pool.
func (n *WorkerPoolNode) Create() error {
	if err := n.BaseNode.Create(); err != nil {
		return err
	}

	n.wg.Add(n.numWorkers)
	for i := 0; i < n.numWorkers; i++ {
		go n.worker()
	}

	return nil
}

// worker processes jobs from the job channel.
func (n *WorkerPoolNode) worker() {
	defer n.wg.Done()
	for {
		select {
		case <-n.ctx.Done():
			return
		case j, ok := <-n.jobChan:
			if !ok {
				return
			}
			// Use n.BaseNode.Process to apply middlewares, tracking, etc.
			output, err := n.BaseNode.Process(j.input)
			j.resultChan <- workerResult{output: output, err: err}
		}
	}
}

// Process handles a request by sending it to the worker pool.
func (n *WorkerPoolNode) Process(input []byte) ([]byte, error) {
	n.mu.RLock()
	if n.deleted {
		n.mu.RUnlock()
		return nil, ErrWorkerPoolDeleted
	}
	n.mu.RUnlock()

	resultChan := make(chan workerResult, 1)

	// Create the job
	j := job{
		input:      input,
		resultChan: resultChan,
	}

	// Try to queue the job
	select {
	case <-n.ctx.Done():
		return nil, ErrWorkerPoolDeleted
	case n.jobChan <- j:
	}

	// Wait for the result
	select {
	case <-n.ctx.Done():
		return nil, ErrWorkerPoolDeleted
	case res := <-resultChan:
		return res.output, res.err
	}
}

// Delete gracefully shuts down the worker pool and cleans up resources.
func (n *WorkerPoolNode) Delete() error {
	n.mu.Lock()
	if n.deleted {
		n.mu.Unlock()
		return nil
	}
	n.deleted = true
	n.mu.Unlock()

	// Signal workers to stop
	n.cancel()

	// Wait for all workers to finish their current jobs
	n.wg.Wait()

	// Drain any pending jobs that were queued but not picked up without closing the channel
	// to prevent panics from slow writers in Process() that passed the deleted check.
drainLoop:
	for {
		select {
		case j := <-n.jobChan:
			j.resultChan <- workerResult{nil, ErrWorkerPoolDeleted}
		default:
			break drainLoop
		}
	}

	return n.BaseNode.Delete()
}
