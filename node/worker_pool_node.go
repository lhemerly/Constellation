package node

import (
	"context"
	"errors"
	"sync"
)

var ErrQueueFull = errors.New("worker pool queue is full")

// WorkerPoolNode processes inputs concurrently using a fixed-size pool of workers.
type WorkerPoolNode struct {
	*BaseNode
	queueSize  int
	numWorkers int
	taskQueue  chan []byte
	ctx        context.Context
	cancel     context.CancelFunc
	workerWg   sync.WaitGroup
}

// NewWorkerPoolNode creates a new WorkerPoolNode with the given queue size and worker count.
func NewWorkerPoolNode(id string, queueSize, numWorkers int) *WorkerPoolNode {
	wp := &WorkerPoolNode{
		BaseNode:   NewBaseNode(id),
		queueSize:  queueSize,
		numWorkers: numWorkers,
	}

	// Override Process directly to queue inputs rather than setting it via SetProcessFunc
	return wp
}

// Create initializes the worker pool.
func (wp *WorkerPoolNode) Create() error {
	if err := wp.BaseNode.Create(); err != nil {
		return err
	}

	wp.taskQueue = make(chan []byte, wp.queueSize)
	wp.ctx, wp.cancel = context.WithCancel(context.Background())

	for i := 0; i < wp.numWorkers; i++ {
		wp.workerWg.Add(1)
		go wp.workerLoop()
	}

	return nil
}

// Process queues the input for processing. If the queue is full, it returns ErrQueueFull.
// It clones the input byte slice to prevent data races.
func (wp *WorkerPoolNode) Process(input []byte) ([]byte, error) {
	// Fast-fail if the context is already done
	select {
	case <-wp.ctx.Done():
		return nil, errors.New("worker pool shutting down")
	default:
	}

	inputCopy := make([]byte, len(input))
	copy(inputCopy, input)

	select {
	case <-wp.ctx.Done():
		return nil, errors.New("worker pool shutting down")
	case wp.taskQueue <- inputCopy:
		return nil, nil // Asynchronous, no output returned immediately
	default:
		return nil, ErrQueueFull
	}
}

// Delete cleanly shuts down the worker pool, preventing new tasks and draining the queue.
func (wp *WorkerPoolNode) Delete() error {
	// Signal workers to stop
	if wp.cancel != nil {
		wp.cancel()
	}

	// Wait for workers to finish current tasks
	wp.workerWg.Wait()

	// Drain the remaining queue safely (non-blocking select loop)
	if wp.taskQueue != nil {
		for {
			select {
			case <-wp.taskQueue:
			default:
				goto EndDrain
			}
		}
	EndDrain:
		close(wp.taskQueue)
	}

	return wp.BaseNode.Delete()
}

// workerLoop continuously processes tasks from the queue until the context is canceled.
func (wp *WorkerPoolNode) workerLoop() {
	defer wp.workerWg.Done()

	for {
		select {
		case <-wp.ctx.Done():
			return
		case task := <-wp.taskQueue:
			// Process via BaseNode to execute middlewares, metrics, and base process function
			_, _ = wp.BaseNode.Process(task)
		}
	}
}
