package node

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrQueueFull  = errors.New("worker pool queue is full")
	ErrNodeClosed = errors.New("node is closed")
)

type workerTask struct {
	input []byte
	resCh chan workerResult
}

type workerResult struct {
	output []byte
	err    error
}

// WorkerPoolNode processes inputs concurrently using a fixed pool of workers.
type WorkerPoolNode struct {
	*BaseNode
	workers    int
	queueSize  int
	queue      chan workerTask
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
	ctx        context.Context
	workerFunc func([]byte) ([]byte, error)
}

// NewWorkerPoolNode creates a new WorkerPoolNode with a specified number of workers and queue size.
func NewWorkerPoolNode(id string, workers, queueSize int) *WorkerPoolNode {
	ctx, cancel := context.WithCancel(context.Background())
	wp := &WorkerPoolNode{
		BaseNode:   NewBaseNode(id),
		workers:    workers,
		queueSize:  queueSize,
		queue:      make(chan workerTask, queueSize),
		cancelFunc: cancel,
		ctx:        ctx,
		workerFunc: func(input []byte) ([]byte, error) { return input, nil },
	}
	// BaseNode's SetProcessFunc wraps the pool process with any middlewares
	wp.BaseNode.SetProcessFunc(wp.poolProcess)
	return wp
}

// SetProcessFunc sets the function that will be executed by the background workers.
// Note: Middlewares added via Use() will wrap the pool enqueueing process, not the worker execution.
func (wp *WorkerPoolNode) SetProcessFunc(processFunc func([]byte) ([]byte, error)) {
	wp.mutex.Lock()
	defer wp.mutex.Unlock()
	wp.workerFunc = processFunc
}

// Create initializes the worker pool by starting the worker goroutines.
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

func (wp *WorkerPoolNode) worker() {
	defer wp.wg.Done()

	// Drain the queue using a simple loop.
	// We don't close the channel during shutdown, so we rely on context cancellation.
	for {
		select {
		case <-wp.ctx.Done():
			// Try to process remaining items in the queue without blocking
			for {
				select {
				case task := <-wp.queue:
					wp.executeTask(task)
				default:
					return
				}
			}
		case task := <-wp.queue:
			wp.executeTask(task)
		}
	}
}

func (wp *WorkerPoolNode) executeTask(task workerTask) {
	wp.mutex.RLock()
	wf := wp.workerFunc
	wp.mutex.RUnlock()

	// Execute the underlying worker logic
	output, err := wf(task.input)
	task.resCh <- workerResult{output: output, err: err}
}

// Delete gracefully shuts down the worker pool, waiting for workers to complete remaining tasks.
func (wp *WorkerPoolNode) Delete() error {
	// Cancel the context to signal workers to stop waiting for new tasks and drain the queue.
	wp.cancelFunc()

	// Wait for all workers to finish
	wp.wg.Wait()

	return wp.BaseNode.Delete()
}

func (wp *WorkerPoolNode) poolProcess(input []byte) ([]byte, error) {
	// Fast-fail if the node is already shutting down
	select {
	case <-wp.ctx.Done():
		return nil, ErrNodeClosed
	default:
	}

	// Clone the input to avoid data races before enqueuing to background workers
	inputCopy := make([]byte, len(input))
	copy(inputCopy, input)

	resCh := make(chan workerResult, 1)
	task := workerTask{
		input: inputCopy,
		resCh: resCh,
	}

	// Try to enqueue the task
	select {
	case <-wp.ctx.Done():
		return nil, ErrNodeClosed
	case wp.queue <- task:
		// Task enqueued, wait for result
		select {
		case <-wp.ctx.Done():
			return nil, ErrNodeClosed
		case res := <-resCh:
			return res.output, res.err
		}
	default:
		return nil, ErrQueueFull
	}
}
