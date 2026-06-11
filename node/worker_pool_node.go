package node

import (
	"context"
	"errors"
	"sync"
)

// ErrQueueFull is returned when the worker pool queue is at capacity.
var ErrQueueFull = errors.New("worker pool queue is full")
var ErrWorkerPoolDeleted = errors.New("worker pool deleted")

type workerJob struct {
	input      []byte
	resultChan chan<- workerResult
}

type workerResult struct {
	output []byte
	err    error
}

// WorkerPoolNode extends BaseNode to process data concurrently using a fixed
// number of background workers. Requests are enqueued up to a maximum queue size.
type WorkerPoolNode struct {
	*BaseNode
	numWorkers int
	queueSize  int
	jobQueue   chan workerJob
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// NewWorkerPoolNode creates a new WorkerPoolNode with the specified ID,
// number of workers, and maximum queue size.
func NewWorkerPoolNode(id string, numWorkers, queueSize int) *WorkerPoolNode {
	if numWorkers <= 0 {
		numWorkers = 1
	}
	if queueSize < 0 {
		queueSize = 0
	}

	wp := &WorkerPoolNode{
		BaseNode:   NewBaseNode(id),
		numWorkers: numWorkers,
		queueSize:  queueSize,
		jobQueue:   make(chan workerJob, queueSize),
	}

	return wp
}

// Create initializes the node, the background context, and starts worker goroutines.
func (wp *WorkerPoolNode) Create() error {
	if err := wp.BaseNode.Create(); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	wp.cancel = cancel

	for i := 0; i < wp.numWorkers; i++ {
		wp.wg.Add(1)
		go wp.worker(ctx)
	}

	return nil
}

// Process overrides BaseNode.Process. It enqueues the request and returns the result
// computed asynchronously by a worker goroutine.
func (wp *WorkerPoolNode) Process(input []byte) ([]byte, error) {
	resultChan := make(chan workerResult, 1)

	job := workerJob{
		input:      input,
		resultChan: resultChan,
	}

	select {
	// First check if canceled/deleted. If so, don't queue.
	// But actually, we don't have ctx here directly, we rely on the channel.
	// We should just attempt to enqueue.
	case wp.jobQueue <- job:
		// Enqueued successfully. Wait for result.
		res := <-resultChan
		return res.output, res.err
	default:
		// Queue is full.
		return nil, ErrQueueFull
	}
}

func (wp *WorkerPoolNode) worker(ctx context.Context) {
	defer wp.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case job := <-wp.jobQueue:
			// Process the job
			// We invoke BaseNode.Process to ensure middlewares, processFunc locking, and metrics run.
			output, err := wp.BaseNode.Process(job.input)
			job.resultChan <- workerResult{output: output, err: err}
		}
	}
}

// Delete signals the workers to shut down, waits for them, and cleans up.
func (wp *WorkerPoolNode) Delete() error {
	if wp.cancel != nil {
		wp.cancel()
	}
	wp.wg.Wait()

	// Safely drain the job channel using a non-blocking select loop
	for {
		select {
		case job := <-wp.jobQueue:
			job.resultChan <- workerResult{output: nil, err: ErrWorkerPoolDeleted}
		default:
			return wp.BaseNode.Delete()
		}
	}
}
