package node

import (
	"context"
	"errors"
	"sync"
)

// WorkerPoolNode manages a pool of worker goroutines reading from a job channel.
type WorkerPoolNode struct {
	*BaseNode
	workers    int
	jobQueue   chan job
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	shutdownMu sync.Mutex
	isShutdown bool
}

type job struct {
	input      []byte
	resultChan chan jobResult
}

type jobResult struct {
	output []byte
	err    error
}

// NewWorkerPoolNode creates a new WorkerPoolNode with the specified number of workers and queue capacity.
func NewWorkerPoolNode(id string, workers int, queueCapacity int) *WorkerPoolNode {
	if workers <= 0 {
		workers = 1
	}
	if queueCapacity < 0 {
		queueCapacity = 0
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPoolNode{
		BaseNode: NewBaseNode(id),
		workers:  workers,
		jobQueue: make(chan job, queueCapacity),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Create initializes the node and starts the worker pool.
func (n *WorkerPoolNode) Create() error {
	if err := n.BaseNode.Create(); err != nil {
		return err
	}

	n.wg.Add(n.workers)
	for i := 0; i < n.workers; i++ {
		go n.workerLoop()
	}

	return nil
}

// workerLoop is the loop executed by each worker goroutine.
func (n *WorkerPoolNode) workerLoop() {
	defer n.wg.Done()
	for {
		select {
		case <-n.ctx.Done():
			return
		case j, ok := <-n.jobQueue:
			if !ok {
				return
			}
			// Process the job
			output, err := n.BaseNode.Process(j.input)
			j.resultChan <- jobResult{output: output, err: err}
		}
	}
}

// Process enqueues the input into the job queue and waits for the result.
func (n *WorkerPoolNode) Process(input []byte) ([]byte, error) {
	n.shutdownMu.Lock()
	if n.isShutdown {
		n.shutdownMu.Unlock()
		return nil, errors.New("node is shutdown")
	}
	n.shutdownMu.Unlock()

	resultChan := make(chan jobResult, 1)
	j := job{
		input:      input,
		resultChan: resultChan,
	}

	select {
	case <-n.ctx.Done():
		return nil, errors.New("node is shutdown")
	case n.jobQueue <- j:
	}

	select {
	case <-n.ctx.Done():
		return nil, errors.New("node is shutdown")
	case res := <-resultChan:
		return res.output, res.err
	}
}

// Delete stops the worker pool and cleans up resources.
func (n *WorkerPoolNode) Delete() error {
	n.shutdownMu.Lock()
	if n.isShutdown {
		n.shutdownMu.Unlock()
		return nil
	}
	n.isShutdown = true
	n.shutdownMu.Unlock()

	// Cancel the context to signal workers to stop
	n.cancel()

	// Wait for all workers to finish
	n.wg.Wait()

	// Drain the job queue and notify pending jobs
	for {
		select {
		case j := <-n.jobQueue:
			j.resultChan <- jobResult{output: nil, err: errors.New("worker pool deleted")}
		default:
			goto DRAINED
		}
	}
DRAINED:

	return n.BaseNode.Delete()
}
