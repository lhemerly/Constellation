package node

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

// WorkerPoolNode manages a fixed-size pool of workers to process requests.
type WorkerPoolNode struct {
	*BaseNode
	poolSize   int
	jobQueue   chan workerJob
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
}

type workerResult struct {
	output []byte
	err    error
}

type workerJob struct {
	input      []byte
	resultChan chan<- workerResult
}

// NewWorkerPoolNode creates a new WorkerPoolNode with the specified pool size.
func NewWorkerPoolNode(id string, poolSize int) *WorkerPoolNode {
	if poolSize <= 0 {
		panic("WorkerPoolNode requires a pool size > 0")
	}

	wpNode := &WorkerPoolNode{
		BaseNode: NewBaseNode(id),
		poolSize: poolSize,
	}

	// We MUST set the base process, otherwise Process() calls processFunc.
	// But SetProcessFunc replaces baseProcess and rebuilds processFunc.
	// If we just use SetProcessFunc(poolProcess), then when processFunc is called (via Process()), it calls poolProcess.
	// Then poolProcess queues the job, and the worker executes... baseProcess? Which is poolProcess!
	// So we have an infinite loop! We need the worker to execute the default logic or the user's logic, and Process() to execute poolProcess.

	// Set the default baseProcess to echo
	wpNode.baseProcess = func(input []byte) ([]byte, error) {
		return input, nil
	}

	return wpNode
}

// Process overrides BaseNode's Process to route requests through the pool.
func (wp *WorkerPoolNode) Process(input []byte) ([]byte, error) {
	atomic.AddUint64(&wp.eventCounter, 1)
	return wp.poolProcess(input)
}

// Create initializes the worker pool node and starts its workers.
func (wp *WorkerPoolNode) Create() error {
	if err := wp.BaseNode.Create(); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	wp.cancelFunc = cancel
	wp.jobQueue = make(chan workerJob, wp.poolSize*2) // Buffer size slightly larger than pool

	for i := 0; i < wp.poolSize; i++ {
		wp.wg.Add(1)
		go wp.worker(ctx)
	}

	return nil
}

// worker represents a single goroutine processing jobs from the queue.
func (wp *WorkerPoolNode) worker(ctx context.Context) {
	defer wp.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-wp.jobQueue:
			if !ok {
				return
			}

			// We want to execute the node's process function (which may have middlewares applied).
			// If we call wp.BaseNode.Process, it will increment eventCounter and call processFunc.
			// Since we manually manage processFunc to be poolProcess during NewWorkerPoolNode... wait.
			// SetProcessFunc on BaseNode actually sets baseProcess and calls rebuildProcessFunc which
			// sets processFunc to the wrapped version of baseProcess.
			// So if we embed BaseNode, and the user calls wp.SetProcessFunc or wp.Use,
			// processFunc will be overwritten with the middlewares wrapping baseProcess.
			// That means processFunc will no longer be poolProcess!
			// This means wp.Process() should just be poolProcess, and worker should call wp.processFunc!

			// Get the wrapped processFunc (without calling Process to avoid double increment or wrong poolProcess call)
			wp.mutex.RLock()
			fn := wp.processFunc
			wp.mutex.RUnlock()

			output, err := fn(job.input)

			job.resultChan <- workerResult{
				output: output,
				err:    err,
			}
		}
	}
}

// poolProcess handles the processing by dispatching the job to the worker pool.
func (wp *WorkerPoolNode) poolProcess(input []byte) ([]byte, error) {
	if wp.jobQueue == nil {
		return nil, errors.New("worker pool is not created")
	}

	resultChan := make(chan workerResult, 1)

	job := workerJob{
		input:      input,
		resultChan: resultChan,
	}

	// Dispatch job to the queue
	wp.jobQueue <- job

	// Wait for the result
	res := <-resultChan
	return res.output, res.err
}

// Delete stops the worker pool and cleans up resources.
func (wp *WorkerPoolNode) Delete() error {
	wp.mutex.Lock()
	if wp.cancelFunc != nil {
		wp.cancelFunc()
		wp.cancelFunc = nil
		wp.wg.Wait()
		close(wp.jobQueue)

		// Drain queue to prevent goroutine leaks for pending callers
		for job := range wp.jobQueue {
			job.resultChan <- workerResult{
				output: nil,
				err:    errors.New("worker pool deleted"),
			}
		}
	}
	wp.mutex.Unlock()
	return wp.BaseNode.Delete()
}
