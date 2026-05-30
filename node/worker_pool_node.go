package node

import (
	"context"
	"errors"
	"sync"
)

// ErrWorkerPoolClosed is returned when trying to process a message on a closed WorkerPoolNode.
var ErrWorkerPoolClosed = errors.New("worker pool is closed")

// job represents a unit of work to be processed by a worker.
type job struct {
	input      []byte
	resultChan chan<- jobResult
}

// jobResult represents the outcome of a processed job.
type jobResult struct {
	output []byte
	err    error
}

// WorkerPoolNode extends BaseNode to process data concurrently using a fixed-size pool of workers.
type WorkerPoolNode struct {
	*BaseNode
	numWorkers int
	jobQueue   chan job
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	isClosed   bool
	mu         sync.RWMutex
}

// NewWorkerPoolNode creates a new WorkerPoolNode with the given ID and number of workers.
func NewWorkerPoolNode(id string, numWorkers int) *WorkerPoolNode {
	if numWorkers <= 0 {
		numWorkers = 1
	}
	ctx, cancel := context.WithCancel(context.Background())
	w := &WorkerPoolNode{
		BaseNode:   NewBaseNode(id),
		numWorkers: numWorkers,
		jobQueue:   make(chan job, numWorkers*2), // Optional buffering
		ctx:        ctx,
		cancel:     cancel,
	}
	return w
}

// Create initializes the WorkerPoolNode and spawns worker goroutines.
func (w *WorkerPoolNode) Create() error {
	if err := w.BaseNode.Create(); err != nil {
		return err
	}

	w.mu.Lock()
	if w.isClosed {
		w.mu.Unlock()
		return ErrWorkerPoolClosed
	}
	w.mu.Unlock()

	for i := 0; i < w.numWorkers; i++ {
		w.wg.Add(1)
		go w.worker()
	}

	return nil
}

// worker represents a single goroutine processing jobs from the queue.
func (w *WorkerPoolNode) worker() {
	defer w.wg.Done()
	for {
		select {
		case <-w.ctx.Done():
			return
		case j, ok := <-w.jobQueue:
			if !ok {
				return
			}
			// Invoke BaseNode.Process to apply middlewares and update counters.
			output, err := w.BaseNode.Process(j.input)
			j.resultChan <- jobResult{output: output, err: err}
		}
	}
}

// Process enqueues the input data to be processed by an available worker.
func (w *WorkerPoolNode) Process(input []byte) ([]byte, error) {
	w.mu.RLock()
	if w.isClosed {
		w.mu.RUnlock()
		return nil, ErrWorkerPoolClosed
	}
	w.mu.RUnlock()

	resultChan := make(chan jobResult, 1)

	select {
	case <-w.ctx.Done():
		return nil, ErrWorkerPoolClosed
	case w.jobQueue <- job{input: input, resultChan: resultChan}:
		// Wait for the result
		select {
		case <-w.ctx.Done():
			return nil, ErrWorkerPoolClosed
		case res := <-resultChan:
			return res.output, res.err
		}
	}
}

// Delete cleanly shuts down the WorkerPoolNode, draining the queue and waiting for workers to finish.
func (w *WorkerPoolNode) Delete() error {
	w.mu.Lock()
	if w.isClosed {
		w.mu.Unlock()
		return nil
	}
	w.isClosed = true
	w.cancel() // Signal workers to stop
	w.mu.Unlock()

	w.wg.Wait() // Wait for active workers to finish current job and exit

	// Drain the queue for any pending callers
drainLoop:
	for {
		select {
		case j := <-w.jobQueue:
			j.resultChan <- jobResult{output: nil, err: ErrWorkerPoolClosed}
		default:
			break drainLoop
		}
	}
	close(w.jobQueue)

	return w.BaseNode.Delete()
}
