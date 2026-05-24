package node

import (
	"context"
	"errors"
	"sync"
)

var ErrWorkerPoolClosed = errors.New("worker pool deleted")

type job struct {
	input []byte
	res   chan result
	ctx   context.Context
}

type result struct {
	output []byte
	err    error
}

// WorkerPoolNode extends BaseNode to process inputs using a fixed-size pool of workers.
type WorkerPoolNode struct {
	*BaseNode
	jobs   chan job
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

// NewWorkerPoolNode creates a new WorkerPoolNode with a given number of workers and buffer size.
func NewWorkerPoolNode(id string, workers int, bufferSize int) *WorkerPoolNode {
	ctx, cancel := context.WithCancel(context.Background())
	n := &WorkerPoolNode{
		BaseNode: NewBaseNode(id),
		jobs:     make(chan job, bufferSize),
		ctx:      ctx,
		cancel:   cancel,
	}

	for i := 0; i < workers; i++ {
		n.wg.Add(1)
		go n.worker()
	}

	return n
}

func (n *WorkerPoolNode) worker() {
	defer n.wg.Done()
	for {
		select {
		case <-n.ctx.Done():
			return
		case j, ok := <-n.jobs:
			if !ok {
				return
			}

			// Process using the base node's logic to execute middlewares and base functions
			out, err := n.BaseNode.Process(j.input)

			select {
			case <-j.ctx.Done():
				// Request was cancelled/timed out, no need to send back
			case j.res <- result{output: out, err: err}:
			}
		}
	}
}

// Process overrides BaseNode.Process to route inputs through the worker pool.
func (n *WorkerPoolNode) Process(input []byte) ([]byte, error) {
	resChan := make(chan result, 1)

	j := job{
		input: input,
		res:   resChan,
		ctx:   context.Background(), // Context support could be added later for cancellation
	}

	select {
	case <-n.ctx.Done():
		return nil, ErrWorkerPoolClosed
	case n.jobs <- j:
	}

	// Wait for result
	select {
	case <-n.ctx.Done():
		return nil, ErrWorkerPoolClosed
	case r := <-resChan:
		return r.output, r.err
	}
}

// Delete gracefully shuts down the worker pool and delegates to BaseNode.Delete().
func (n *WorkerPoolNode) Delete() error {
	n.cancel()
	n.wg.Wait()

	// Drain any remaining jobs in the buffer and send an error.
	// Since we don't close the channel, we loop until it's empty using select.
	for {
		select {
		case j := <-n.jobs:
			select {
			case j.res <- result{output: nil, err: ErrWorkerPoolClosed}:
			default:
			}
		default:
			return n.BaseNode.Delete()
		}
	}
}
