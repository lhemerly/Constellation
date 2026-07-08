package node

import (
	"context"
	"errors"
	"sync"
)

// WorkerPoolNode uses a pool of concurrent goroutines to process incoming
// requests using a channel-based task queue.
type WorkerPoolNode struct {
	*BaseNode
	numWorkers  int
	taskQueue   chan *workerTask
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	shutdownOne sync.Once
	workerFunc  func([]byte) ([]byte, error)
}

type workerTask struct {
	input      []byte
	resultChan chan workerResult
}

type workerResult struct {
	output []byte
	err    error
}

// NewWorkerPoolNode creates a new WorkerPoolNode with the specified number of workers.
func NewWorkerPoolNode(id string, numWorkers int) *WorkerPoolNode {
	if numWorkers <= 0 {
		numWorkers = 1
	}

	ctx, cancel := context.WithCancel(context.Background())
	wpNode := &WorkerPoolNode{
		BaseNode:   NewBaseNode(id),
		numWorkers: numWorkers,
		taskQueue:  make(chan *workerTask, numWorkers*2), // buffer to allow some queuing
		ctx:        ctx,
		cancel:     cancel,
		workerFunc: func(input []byte) ([]byte, error) {
			return input, nil
		},
	}

	// We overwrite the Process function to handle the queuing logic.
	// We do NOT want the default BaseNode process function logic since this node
	// works differently (it dispatches to the pool).
	wpNode.BaseNode.SetProcessFunc(wpNode.dispatchProcess)

	return wpNode
}

// SetProcessFunc sets the function that the workers will execute.
// We override BaseNode's SetProcessFunc because we need to store the function
// to be executed *by the workers*, not by the dispatch mechanism itself.
func (wp *WorkerPoolNode) SetProcessFunc(processFunc func([]byte) ([]byte, error)) {
	wp.BaseNode.mutex.Lock()
	defer wp.BaseNode.mutex.Unlock()
	wp.workerFunc = processFunc
}

func (wp *WorkerPoolNode) dispatchProcess(input []byte) ([]byte, error) {
	// Fast fail if shutting down
	select {
	case <-wp.ctx.Done():
		return nil, errors.New("worker pool is shutting down")
	default:
	}

	resChan := make(chan workerResult, 1)

	// Clone input to prevent data races
	inputCopy := make([]byte, len(input))
	copy(inputCopy, input)

	task := &workerTask{
		input:      inputCopy,
		resultChan: resChan,
	}

	select {
	case wp.taskQueue <- task:
		// Task enqueued
	case <-wp.ctx.Done():
		return nil, errors.New("worker pool is shutting down")
	}

	select {
	case res := <-resChan:
		return res.output, res.err
	case <-wp.ctx.Done():
		return nil, errors.New("worker pool is shutting down")
	}
}

// Create initializes the worker pool and starts the worker goroutines.
func (wp *WorkerPoolNode) Create() error {
	if err := wp.BaseNode.Create(); err != nil {
		return err
	}

	for i := 0; i < wp.numWorkers; i++ {
		wp.wg.Add(1)
		go wp.workerLoop()
	}

	return nil
}

func (wp *WorkerPoolNode) workerLoop() {
	defer wp.wg.Done()
	for {
		select {
		case <-wp.ctx.Done():
			return
		case task := <-wp.taskQueue:
			wp.BaseNode.mutex.RLock()
			wFunc := wp.workerFunc
			wp.BaseNode.mutex.RUnlock()

			out, err := wFunc(task.input)
			task.resultChan <- workerResult{output: out, err: err}
		}
	}
}

// Delete cleanly shuts down the worker pool.
func (wp *WorkerPoolNode) Delete() error {
	wp.shutdownOne.Do(func() {
		// Signal shutdown to all workers
		wp.cancel()

		// Wait for active workers to finish current task and exit loop
		wp.wg.Wait()

		// Drain the channel safely without closing it
		// to avoid "send on closed channel" panic from concurrent active dispatches.
	drainLoop:
		for {
			select {
			case t := <-wp.taskQueue:
				// Notify queued tasks they were cancelled during shutdown
				t.resultChan <- workerResult{err: errors.New("worker pool shut down before task could be processed")}
			default:
				break drainLoop
			}
		}
	})

	return wp.BaseNode.Delete()
}
