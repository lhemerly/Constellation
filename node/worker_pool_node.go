package node

import (
	"context"
	"errors"
	"sync"
)

// ErrQueueFull is returned when the WorkerPoolNode's task queue is full.
var ErrQueueFull = errors.New("task queue is full")

type workerTask struct {
	input      []byte
	resultChan chan struct {
		output []byte
		err    error
	}
}

// WorkerPoolNode manages a pool of worker goroutines that process tasks concurrently.
type WorkerPoolNode struct {
	*BaseNode
	workers    int
	queueSize  int
	taskQueue  chan workerTask
	ctx        context.Context
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
}

// NewWorkerPoolNode creates a new WorkerPoolNode.
func NewWorkerPoolNode(id string, workers, queueSize int) *WorkerPoolNode {
	ctx, cancel := context.WithCancel(context.Background())
	wp := &WorkerPoolNode{
		BaseNode:   NewBaseNode(id),
		workers:    workers,
		queueSize:  queueSize,
		taskQueue:  make(chan workerTask, queueSize),
		ctx:        ctx,
		cancelFunc: cancel,
	}

	// We don't use SetProcessFunc here because we want to intercept the entire
	// Process call to enqueue it, but we still need the BaseNode.Process to execute
	// so that middlewares, eventCounter, and processFunc are invoked properly by the workers.
	return wp
}

// Create spawns the worker pool goroutines.
func (n *WorkerPoolNode) Create() error {
	for i := 0; i < n.workers; i++ {
		n.wg.Add(1)
		go n.worker()
	}
	return n.BaseNode.Create()
}

func (n *WorkerPoolNode) worker() {
	defer n.wg.Done()
	for {
		select {
		case <-n.ctx.Done():
			return
		case task, ok := <-n.taskQueue:
			if !ok {
				return
			}
			// Clone input to prevent data races if modified by caller
			inputCopy := make([]byte, len(task.input))
			copy(inputCopy, task.input)

			// Invoke BaseNode.Process to trigger middlewares and actual processFunc
			output, err := n.BaseNode.Process(inputCopy)
			task.resultChan <- struct {
				output []byte
				err    error
			}{output, err}
		}
	}
}

// Process enqueues a task for the worker pool and waits for the result.
func (n *WorkerPoolNode) Process(input []byte) ([]byte, error) {
	// First, check for shutdown signal to avoid pseudo-random select
	select {
	case <-n.ctx.Done():
		return nil, errors.New("worker pool is shutting down")
	default:
	}

	resultChan := make(chan struct {
		output []byte
		err    error
	}, 1)

	task := workerTask{
		input:      input,
		resultChan: resultChan,
	}

	// Try to enqueue the task
	select {
	case n.taskQueue <- task:
		// Wait for the result or context cancellation
		select {
		case res := <-resultChan:
			return res.output, res.err
		case <-n.ctx.Done():
			return nil, errors.New("worker pool shut down before processing completed")
		}
	case <-n.ctx.Done():
		return nil, errors.New("worker pool is shutting down")
	default:
		return nil, ErrQueueFull
	}
}

// Delete gracefully shuts down the worker pool.
func (n *WorkerPoolNode) Delete() error {
	// Cancel the context to signal workers to stop reading from the queue and shutdown
	n.cancelFunc()

	// Wait for all workers to finish currently executing tasks
	n.wg.Wait()

	// Safe to close the channel now that no workers are reading and Process will fail-fast
	close(n.taskQueue)

	// Drain the queue to unblock any pending tasks
	for task := range n.taskQueue {
		if task.resultChan != nil {
			task.resultChan <- struct {
				output []byte
				err    error
			}{nil, errors.New("worker pool shut down before processing")}
		}
	}

	return n.BaseNode.Delete()
}
