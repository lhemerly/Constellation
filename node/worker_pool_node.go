package node

import (
	"context"
	"errors"
	"sync"
)

// ErrQueueFull is returned when the internal job queue is full and cannot accept more requests.
var ErrQueueFull = errors.New("worker pool queue is full")

// ErrWorkerPoolClosed is returned if a process is attempted on a closed worker pool.
var ErrWorkerPoolClosed = errors.New("worker pool is closed")

// WorkerPoolNode processes requests asynchronously using a fixed pool of background goroutines.
type WorkerPoolNode struct {
	*BaseNode
	workers    int
	queue      chan []byte
	cancelFunc context.CancelFunc
	ctx        context.Context
	wg         sync.WaitGroup
	closed     bool
	closeMutex sync.RWMutex
}

// NewWorkerPoolNode creates a new WorkerPoolNode with the given ID, number of workers, and queue capacity.
func NewWorkerPoolNode(id string, workers int, queueCapacity int) *WorkerPoolNode {
	ctx, cancel := context.WithCancel(context.Background())

	wp := &WorkerPoolNode{
		BaseNode:   NewBaseNode(id),
		workers:    workers,
		queue:      make(chan []byte, queueCapacity),
		cancelFunc: cancel,
		ctx:        ctx,
	}

	// Override Process directly, as intercepting via SetProcessFunc for queueing
	// is not recommended (users might overwrite it).
	// We will handle queueing in Process, and worker processing via BaseNode.Process
	// to ensure middlewares and event counters are respected.

	return wp
}

// Create starts the worker pool goroutines.
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

// Process queues the input for asynchronous processing.
// Returns an immediate "accepted" response or ErrQueueFull if the queue is full.
func (wp *WorkerPoolNode) Process(input []byte) ([]byte, error) {
	wp.closeMutex.RLock()
	defer wp.closeMutex.RUnlock()

	if wp.closed {
		return nil, ErrWorkerPoolClosed
	}

	// Priority: check context cancellation first
	select {
	case <-wp.ctx.Done():
		return nil, ErrWorkerPoolClosed
	default:
	}

	// Clone input for async processing
	inputCopy := make([]byte, len(input))
	copy(inputCopy, input)

	select {
	case wp.queue <- inputCopy:
		return []byte("accepted"), nil
	case <-wp.ctx.Done():
		return nil, ErrWorkerPoolClosed
	default:
		return nil, ErrQueueFull
	}
}

// worker continuously processes jobs from the queue until context is cancelled.
func (wp *WorkerPoolNode) worker() {
	defer wp.wg.Done()

	for {
		select {
		case <-wp.ctx.Done():
			return
		case input, ok := <-wp.queue:
			if !ok {
				return
			}
			// Invoke base Process to run middlewares and processFunc
			_, _ = wp.BaseNode.Process(input)
		}
	}
}

// Delete gracefully shuts down the worker pool and its base node.
func (wp *WorkerPoolNode) Delete() error {
	wp.closeMutex.Lock()
	if wp.closed {
		wp.closeMutex.Unlock()
		return nil
	}
	wp.closed = true
	wp.closeMutex.Unlock()

	// Signal workers to stop
	wp.cancelFunc()

	// Wait for workers to finish
	wp.wg.Wait()

	// Safely drain the queue without closing the channel, as concurrent
	// Process calls might still hold RLock, but since we cancel ctx first,
	// they will return ErrWorkerPoolClosed. However, some might be blocked
	// on channel send. Context cancellation handles this.
	// We still drain the queue for clean shutdown.
	close(wp.queue)
	for range wp.queue {
		// drain
	}

	return wp.BaseNode.Delete()
}
