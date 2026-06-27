package node

import (
	"context"
	"errors"
	"sync"
)

var ErrQueueFull = errors.New("worker pool queue full")
var ErrNodeDeleted = errors.New("worker pool node is deleted")

type task struct {
	input      []byte
	resultChan chan taskResult
}

type taskResult struct {
	output []byte
	err    error
}

// WorkerPoolNode dispatches incoming requests to a background worker goroutine pool.
type WorkerPoolNode struct {
	*BaseNode
	workers    int
	queueSize  int
	taskQueue  chan task
	ctx        context.Context
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
}

// NewWorkerPoolNode creates a new WorkerPoolNode with the specified ID,
// number of workers, and size of the task queue.
func NewWorkerPoolNode(id string, workers int, queueSize int) *WorkerPoolNode {
	ctx, cancel := context.WithCancel(context.Background())
	node := &WorkerPoolNode{
		BaseNode:   NewBaseNode(id),
		workers:    workers,
		queueSize:  queueSize,
		taskQueue:  make(chan task, queueSize),
		ctx:        ctx,
		cancelFunc: cancel,
	}
	return node
}

// Create initializes the node and starts the worker pool.
func (n *WorkerPoolNode) Create() error {
	for i := 0; i < n.workers; i++ {
		n.wg.Add(1)
		go n.worker()
	}
	return n.BaseNode.Create()
}

// worker is a background goroutine that processes tasks from the queue.
func (n *WorkerPoolNode) worker() {
	defer n.wg.Done()
	for {
		select {
		case <-n.ctx.Done():
			return
		case t, ok := <-n.taskQueue:
			if !ok {
				return
			}
			// Internally call the base node's Process method so that
			// middlewares and processFunc handle the input.
			output, err := n.BaseNode.Process(t.input)
			t.resultChan <- taskResult{output: output, err: err}
		}
	}
}

// Process overrides the BaseNode Process method to queue the input for processing
// by the background worker pool. It returns an error if the queue is full.
func (n *WorkerPoolNode) Process(input []byte) ([]byte, error) {
	// Fast fail check for shutdown signal
	select {
	case <-n.ctx.Done():
		return nil, ErrNodeDeleted
	default:
	}

	// Clone the input to prevent race conditions from the caller
	inputCopy := make([]byte, len(input))
	copy(inputCopy, input)

	resultChan := make(chan taskResult, 1)
	t := task{
		input:      inputCopy,
		resultChan: resultChan,
	}

	// Enqueue the task, returning ErrQueueFull if the channel is full.
	select {
	case n.taskQueue <- t:
	case <-n.ctx.Done():
		return nil, ErrNodeDeleted
	default:
		return nil, ErrQueueFull
	}

	// Wait for the result
	select {
	case res := <-resultChan:
		return res.output, res.err
	case <-n.ctx.Done():
		return nil, ErrNodeDeleted
	}
}

// Delete signals the workers to shutdown and waits for them to exit.
func (n *WorkerPoolNode) Delete() error {
	n.cancelFunc()
	n.wg.Wait()

	// Safely drain the task queue so blocked processes can exit
	// We do not close n.taskQueue while concurrent processes might be trying to send
	for {
		select {
		case t := <-n.taskQueue:
			t.resultChan <- taskResult{output: nil, err: ErrNodeDeleted}
		default:
			return n.BaseNode.Delete()
		}
	}
}
