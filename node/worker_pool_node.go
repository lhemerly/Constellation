package node

import (
	"context"
	"errors"
	"sync"
)

var ErrWorkerPoolDeleted = errors.New("worker pool deleted")

type jobRequest struct {
	input      []byte
	resultChan chan<- workerResult
}

// WorkerPoolNode manages a pool of worker goroutines to process requests,
// limiting maximum concurrency.
type WorkerPoolNode struct {
	*BaseNode
	workerCount int
	jobChan     chan jobRequest
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewWorkerPoolNode creates a new WorkerPoolNode.
func NewWorkerPoolNode(id string, workerCount int, queueSize int) *WorkerPoolNode {
	ctx, cancel := context.WithCancel(context.Background())
	wp := &WorkerPoolNode{
		BaseNode:    NewBaseNode(id),
		workerCount: workerCount,
		jobChan:     make(chan jobRequest, queueSize),
		ctx:         ctx,
		cancel:      cancel,
	}

	// Important: We don't overwrite SetProcessFunc so users can provide their business logic.
	// Instead, we wrap the baseProcess so we can queue calls.
	// Actually, overriding Process directly is safer so users don't break the queue mechanism.
	// If users call SetProcessFunc, it updates processFunc. We should queue the call to processFunc.

	// Default process function if none is set
	return wp
}

// Create starts the worker goroutines.
func (wp *WorkerPoolNode) Create() error {
	if err := wp.BaseNode.Create(); err != nil {
		return err
	}

	for i := 0; i < wp.workerCount; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}

	return nil
}

// Delete shuts down the worker pool gracefully.
func (wp *WorkerPoolNode) Delete() error {
	// Signal workers to stop
	wp.cancel()

	// Wait for all workers to exit
	wp.wg.Wait()

	// Drain any pending jobs and return error
	// Safe to do this now since workers have exited and context is canceled,
	// so no new jobs can successfully send to jobChan. Wait, concurrent Process
	// calls might be blocked on sending. The select in Process handles ctx.Done().

	// To prevent "send on closed channel" if a late Process caller tries to send
	// while we're closing, we don't actually close(wp.jobChan). We just drain it.
	// Process checks ctx.Done() and avoids sending.

	// Drain the channel
	for {
		select {
		case req := <-wp.jobChan:
			req.resultChan <- workerResult{nil, ErrWorkerPoolDeleted}
		default:
			// Done draining
			return wp.BaseNode.Delete()
		}
	}
}

func (wp *WorkerPoolNode) worker() {
	defer wp.wg.Done()
	for {
		select {
		case <-wp.ctx.Done():
			return
		case req, ok := <-wp.jobChan:
			if !ok {
				return
			}

			// Use the base node's process function (which includes middlewares)
			out, err := wp.BaseNode.Process(req.input)

			// Important: don't block forever if the caller went away,
			// though resultChan is buffered by 1 so this won't block.
			req.resultChan <- workerResult{output: out, err: err}
		}
	}
}

// Process overrides BaseNode.Process to route requests through the worker pool.
func (wp *WorkerPoolNode) Process(input []byte) ([]byte, error) {
	// Check if already shutting down
	select {
	case <-wp.ctx.Done():
		return nil, ErrWorkerPoolDeleted
	default:
	}

	// We buffer by 1 so the worker doesn't block if we return early
	resultChan := make(chan workerResult, 1)

	// Clone input to avoid race condition if caller mutates it while it's in the queue
	inputCopy := make([]byte, len(input))
	copy(inputCopy, input)

	req := jobRequest{
		input:      inputCopy,
		resultChan: resultChan,
	}

	// Try to send the job
	select {
	case <-wp.ctx.Done():
		return nil, ErrWorkerPoolDeleted
	case wp.jobChan <- req:
		// Job queued, wait for result
		select {
		case <-wp.ctx.Done():
			return nil, ErrWorkerPoolDeleted
		case res := <-resultChan:
			return res.output, res.err
		}
	}
}
