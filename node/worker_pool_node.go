package node

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

var (
	ErrWorkerPoolClosed = errors.New("worker pool is closed")
)

type workerJob struct {
	input      []byte
	resultChan chan workerResult
	ctx        context.Context
}

type workerResult struct {
	output []byte
	err    error
}

// WorkerPoolNode manages a pool of worker goroutines to process requests concurrently.
type WorkerPoolNode struct {
	*BaseNode
	numWorkers int
	jobQueue   chan workerJob
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	closed     atomic.Bool
}

// NewWorkerPoolNode creates a new WorkerPoolNode with the specified number of workers.
func NewWorkerPoolNode(id string, numWorkers int) *WorkerPoolNode {
	if numWorkers <= 0 {
		numWorkers = 1
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPoolNode{
		BaseNode:   NewBaseNode(id),
		numWorkers: numWorkers,
		// Buffer size could be larger, but setting to numWorkers allows some buffering
		jobQueue: make(chan workerJob, numWorkers),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Create initializes the worker pool and starts the worker goroutines.
func (n *WorkerPoolNode) Create() error {
	if err := n.BaseNode.Create(); err != nil {
		return err
	}

	for i := 0; i < n.numWorkers; i++ {
		n.wg.Add(1)
		go n.workerLoop()
	}

	return nil
}

func (n *WorkerPoolNode) workerLoop() {
	defer n.wg.Done()
	for {
		select {
		case <-n.ctx.Done():
			return
		case job, ok := <-n.jobQueue:
			if !ok {
				return
			}

			// Check if job context is already cancelled
			select {
			case <-job.ctx.Done():
				job.resultChan <- workerResult{nil, job.ctx.Err()}
				continue
			default:
			}

			output, err := n.BaseNode.Process(job.input)

			// Try to send result, but respect job context cancellation
			select {
			case job.resultChan <- workerResult{output, err}:
			case <-job.ctx.Done():
			}
		}
	}
}

// Process submits a job to the worker pool and waits for the result.
func (n *WorkerPoolNode) Process(input []byte) ([]byte, error) {
	if n.closed.Load() {
		return nil, ErrWorkerPoolClosed
	}

	resultChan := make(chan workerResult, 1)
	job := workerJob{
		input:      input,
		resultChan: resultChan,
		ctx:        context.Background(),
	}

	select {
	case <-n.ctx.Done():
		return nil, ErrWorkerPoolClosed
	case n.jobQueue <- job:
	}

	select {
	case <-n.ctx.Done():
		return nil, ErrWorkerPoolClosed
	case res := <-resultChan:
		return res.output, res.err
	}
}

// ProcessWithContext allows submitting a job with a specific context.
func (n *WorkerPoolNode) ProcessWithContext(ctx context.Context, input []byte) ([]byte, error) {
	if n.closed.Load() {
		return nil, ErrWorkerPoolClosed
	}

	resultChan := make(chan workerResult, 1)
	job := workerJob{
		input:      input,
		resultChan: resultChan,
		ctx:        ctx,
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-n.ctx.Done():
		return nil, ErrWorkerPoolClosed
	case n.jobQueue <- job:
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-n.ctx.Done():
		return nil, ErrWorkerPoolClosed
	case res := <-resultChan:
		return res.output, res.err
	}
}

// Delete stops the worker pool and cleans up resources.
func (n *WorkerPoolNode) Delete() error {
	if n.closed.CompareAndSwap(false, true) {
		n.cancel()
		n.wg.Wait()

		// Drain pending jobs in queue
		for {
			select {
			case job := <-n.jobQueue:
				select {
				case job.resultChan <- workerResult{nil, ErrWorkerPoolClosed}:
				default:
				}
			default:
				goto drainDone
			}
		}
	drainDone:
	}
	return n.BaseNode.Delete()
}
