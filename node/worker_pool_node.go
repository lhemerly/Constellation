package node

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrQueueFull = errors.New("worker pool queue is full")
)

type task struct {
	input      []byte
	resultChan chan taskResult
}

type taskResult struct {
	output []byte
	err    error
}

// WorkerPoolNode manages a concurrent pool of workers for processing tasks.
type WorkerPoolNode struct {
	*BaseNode
	numWorkers int
	queueSize  int
	taskQueue  chan task
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	workerFunc func([]byte) ([]byte, error)
	mu         sync.RWMutex
	closeOnce  sync.Once
}

// NewWorkerPoolNode creates a new WorkerPoolNode with the specified number of workers and queue size.
func NewWorkerPoolNode(id string, numWorkers, queueSize int) *WorkerPoolNode {
	if numWorkers <= 0 {
		panic("WorkerPoolNode requires at least one worker")
	}
	if queueSize <= 0 {
		panic("WorkerPoolNode requires a positive queue size")
	}

	ctx, cancel := context.WithCancel(context.Background())

	wp := &WorkerPoolNode{
		BaseNode:   NewBaseNode(id),
		numWorkers: numWorkers,
		queueSize:  queueSize,
		taskQueue:  make(chan task, queueSize),
		ctx:        ctx,
		cancel:     cancel,
		workerFunc: func(input []byte) ([]byte, error) {
			return input, nil // Default no-op processing
		},
	}

	// Ensure the base node's process function routes to our queueing logic
	wp.BaseNode.SetProcessFunc(wp.queueProcess)

	return wp
}

// SetProcessFunc overrides the default worker processing logic.
func (wp *WorkerPoolNode) SetProcessFunc(processFunc func([]byte) ([]byte, error)) {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	wp.workerFunc = processFunc
}

func (wp *WorkerPoolNode) queueProcess(input []byte) ([]byte, error) {
	resultChan := make(chan taskResult, 1)

	// Clone the input byte slice to prevent data races if modified by the caller
	inputCopy := make([]byte, len(input))
	copy(inputCopy, input)

	t := task{
		input:      inputCopy,
		resultChan: resultChan,
	}

	// Prioritize context cancellation to prevent enqueuing when shutting down
	select {
	case <-wp.ctx.Done():
		return nil, errors.New("node is shutting down")
	default:
	}

	select {
	case wp.taskQueue <- t:
		res := <-resultChan
		return res.output, res.err
	case <-wp.ctx.Done():
		return nil, errors.New("node is shutting down")
	default:
		return nil, ErrQueueFull
	}
}

func (wp *WorkerPoolNode) workerLoop(id int) {
	defer wp.wg.Done()
	for {
		select {
		case <-wp.ctx.Done():
			return
		case t, ok := <-wp.taskQueue:
			if !ok {
				return
			}

			wp.mu.RLock()
			wFunc := wp.workerFunc
			wp.mu.RUnlock()

			output, err := wFunc(t.input)
			t.resultChan <- taskResult{output: output, err: err}
		}
	}
}

// Create initializes and starts the worker pool.
func (wp *WorkerPoolNode) Create() error {
	if err := wp.BaseNode.Create(); err != nil {
		return err
	}

	for i := 0; i < wp.numWorkers; i++ {
		wp.wg.Add(1)
		go wp.workerLoop(i)
	}

	return nil
}

// Delete gracefully shuts down the worker pool and cleans up.
func (wp *WorkerPoolNode) Delete() error {
	var errs []error

	wp.closeOnce.Do(func() {
		// Stop accepting new tasks and signal workers to stop
		wp.cancel()

		// Wait for workers to finish current tasks
		wp.wg.Wait()

		// Safe to close queue now that no senders or receivers are active
		close(wp.taskQueue)

		// Drain any remaining tasks in the queue (if any)
		for t := range wp.taskQueue {
			t.resultChan <- taskResult{nil, errors.New("node was shut down")}
		}
	})

	if err := wp.BaseNode.Delete(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
