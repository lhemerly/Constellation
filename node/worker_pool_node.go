package node

import (
	"errors"
	"sync"
)

// ErrPoolClosed is returned when trying to process data on a closed WorkerPoolNode.
var ErrPoolClosed = errors.New("worker pool is closed")

type poolJob struct {
	input  []byte
	result chan poolResult
}

type poolResult struct {
	output []byte
	err    error
}

// WorkerPoolNode extends BaseNode to process requests using a fixed-size goroutine pool.
// This allows controlling the concurrency level.
type WorkerPoolNode struct {
	*BaseNode
	poolSize    int
	jobChan     chan poolJob
	wg          sync.WaitGroup
	closed      bool
	mu          sync.RWMutex
	workerLogic func([]byte) ([]byte, error)
}

// NewWorkerPoolNode creates a new WorkerPoolNode with a given pool size.
func NewWorkerPoolNode(id string, poolSize int) *WorkerPoolNode {
	if poolSize <= 0 {
		poolSize = 1 // default to at least 1 worker
	}
	wp := &WorkerPoolNode{
		BaseNode: NewBaseNode(id),
		poolSize: poolSize,
		jobChan:  make(chan poolJob, poolSize*2), // buffer to prevent blocking callers immediately
	}

	// Override processFunc to distribute work to the pool
	wp.SetProcessFunc(wp.poolProcess)
	return wp
}

// Create starts the worker goroutines.
func (wp *WorkerPoolNode) Create() error {
	if err := wp.BaseNode.Create(); err != nil {
		return err
	}

	wp.wg.Add(wp.poolSize)
	for i := 0; i < wp.poolSize; i++ {
		go wp.worker()
	}
	return nil
}

// Delete stops all workers and cleans up.
func (wp *WorkerPoolNode) Delete() error {
	wp.mu.Lock()
	if wp.closed {
		wp.mu.Unlock()
		return nil
	}
	wp.closed = true
	close(wp.jobChan)
	wp.mu.Unlock()

	wp.wg.Wait()
	return wp.BaseNode.Delete()
}

func (wp *WorkerPoolNode) worker() {
	defer wp.wg.Done()
	for job := range wp.jobChan {
		output, err := wp.executeWorkerLogic(job.input)
		job.result <- poolResult{output: output, err: err}
	}
}

// poolProcess is the entry point that queues work to the pool.
func (wp *WorkerPoolNode) poolProcess(input []byte) ([]byte, error) {
	wp.mu.RLock()
	if wp.closed {
		wp.mu.RUnlock()
		return nil, ErrPoolClosed
	}

	// Clone input to avoid race condition
	inputCopy := make([]byte, len(input))
	copy(inputCopy, input)

	resultChan := make(chan poolResult, 1)
	job := poolJob{
		input:  inputCopy,
		result: resultChan,
	}

	wp.jobChan <- job
	wp.mu.RUnlock()

	res := <-resultChan
	return res.output, res.err
}

// SetWorkerLogic sets the actual processing logic for the workers.
// This is necessary because SetProcessFunc is used for the pool queueing logic.
func (wp *WorkerPoolNode) SetWorkerLogic(logic func([]byte) ([]byte, error)) {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	wp.workerLogic = logic
}

func (wp *WorkerPoolNode) executeWorkerLogic(input []byte) ([]byte, error) {
	wp.mu.RLock()
	logic := wp.workerLogic
	wp.mu.RUnlock()

	if logic == nil {
		return input, nil // default echo behavior
	}
	return logic(input)
}
