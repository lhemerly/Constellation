package node

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrQueueFull = errors.New("worker queue is full")
)

type asyncJob struct {
	input []byte
	resp  chan asyncResult
}

type asyncResult struct {
	output []byte
	err    error
}

// AsyncWorkerNode extends BaseNode to provide a background worker pool and buffered queue.
type AsyncWorkerNode struct {
	*BaseNode
	workers    int
	queueSize  int
	jobQueue   chan asyncJob
	ctx        context.Context
	cancelFunc context.CancelFunc
	workerWg   sync.WaitGroup
}

// NewAsyncWorkerNode creates a new AsyncWorkerNode with the given ID, number of workers, and queue size.
func NewAsyncWorkerNode(id string, workers, queueSize int) *AsyncWorkerNode {
	ctx, cancel := context.WithCancel(context.Background())
	a := &AsyncWorkerNode{
		BaseNode:   NewBaseNode(id),
		workers:    workers,
		queueSize:  queueSize,
		jobQueue:   make(chan asyncJob, queueSize),
		ctx:        ctx,
		cancelFunc: cancel,
	}

	return a
}

// Create overrides BaseNode.Create to initialize workers.
func (a *AsyncWorkerNode) Create() error {
	if err := a.BaseNode.Create(); err != nil {
		return err
	}

	a.startWorkers()
	return nil
}

// startWorkers launches the worker goroutines.
func (a *AsyncWorkerNode) startWorkers() {
	for i := 0; i < a.workers; i++ {
		a.workerWg.Add(1)
		go func() {
			defer a.workerWg.Done()
			for {
				select {
				case <-a.ctx.Done():
					return
				case job, ok := <-a.jobQueue:
					if !ok {
						return
					}
					// Invoke the base process function which includes middlewares and the underlying logic
					out, err := a.BaseNode.Process(job.input)
					job.resp <- asyncResult{output: out, err: err}
				}
			}
		}()
	}
}

// Process overrides BaseNode.Process to queue the work instead of running it inline.
func (a *AsyncWorkerNode) Process(input []byte) ([]byte, error) {
	respChan := make(chan asyncResult, 1)
	job := asyncJob{
		input: input,
		resp:  respChan,
	}

	select {
	case a.jobQueue <- job:
		// Job enqueued successfully
	case <-a.ctx.Done():
		return nil, errors.New("worker pool deleted")
	default:
		return nil, ErrQueueFull
	}

	select {
	case res := <-respChan:
		return res.output, res.err
	case <-a.ctx.Done():
		return nil, errors.New("worker pool deleted")
	}
}

// Delete overrides BaseNode.Delete to cleanly shutdown the worker pool.
func (a *AsyncWorkerNode) Delete() error {
	a.cancelFunc()
	a.workerWg.Wait()

	// Drain the remaining queue safely. We don't close the channel
	// if Process might still attempt to write to it concurrently
	// even with context cancellation (due to non-deterministic select resolution).
	// Instead, we just drain it non-blockingly.
	for {
		select {
		case job := <-a.jobQueue:
			job.resp <- asyncResult{output: nil, err: errors.New("worker pool deleted")}
		default:
			return a.BaseNode.Delete()
		}
	}
}
