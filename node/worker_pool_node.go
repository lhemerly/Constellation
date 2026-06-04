package node

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrQueueFull = errors.New("worker pool queue is full")
)

// WorkerPoolNode wraps a target Node and limits the concurrency of its execution
// using a worker pool. It provides backpressure by returning ErrQueueFull if
// the internal job queue is full.
type WorkerPoolNode struct {
	*BaseNode
	target      Node
	concurrency int
	queueSize   int
	queue       chan job
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

type job struct {
	input      []byte
	resultChan chan jobResult
}

type jobResult struct {
	output []byte
	err    error
}

// NewWorkerPoolNode creates a new WorkerPoolNode with the given concurrency limit
// and queue size. If queueSize is 0, the node provides no queueing and blocks
// or drops requests immediately if workers are busy.
func NewWorkerPoolNode(id string, target Node, concurrency int, queueSize int) *WorkerPoolNode {
	if concurrency <= 0 {
		panic("WorkerPoolNode requires concurrency > 0")
	}

	wp := &WorkerPoolNode{
		BaseNode:    NewBaseNode(id),
		target:      target,
		concurrency: concurrency,
		queueSize:   queueSize,
	}

	wp.SetProcessFunc(wp.processJob)
	return wp
}

// Create initializes the worker pool and starts the worker goroutines.
func (wp *WorkerPoolNode) Create() error {
	if err := wp.BaseNode.Create(); err != nil {
		return err
	}

	if err := wp.target.Create(); err != nil {
		return err
	}

	wp.queue = make(chan job, wp.queueSize)
	wp.ctx, wp.cancel = context.WithCancel(context.Background())

	for i := 0; i < wp.concurrency; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}

	return nil
}

// Delete stops the worker pool and cleans up resources.
func (wp *WorkerPoolNode) Delete() error {
	var errs []error

	if err := wp.BaseNode.Delete(); err != nil {
		errs = append(errs, err)
	}

	if wp.cancel != nil {
		wp.cancel() // Signal workers to stop
	}

	wp.wg.Wait() // Wait for workers to exit

	// Safely drain the job channel
	if wp.queue != nil {
		for {
			select {
			case j := <-wp.queue:
				j.resultChan <- jobResult{nil, errors.New("worker pool deleted")}
			default:
				goto DRAIN_DONE
			}
		}
	DRAIN_DONE:
	}

	if err := wp.target.Delete(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func (wp *WorkerPoolNode) worker() {
	defer wp.wg.Done()

	for {
		select {
		case <-wp.ctx.Done():
			return
		case j := <-wp.queue:
			res, err := wp.target.Process(j.input)
			j.resultChan <- jobResult{output: res, err: err}
		}
	}
}

// processJob queues a job for execution and waits for the result.
func (wp *WorkerPoolNode) processJob(input []byte) ([]byte, error) {
	// Fast-fail if context is done
	select {
	case <-wp.ctx.Done():
		return nil, errors.New("worker pool deleted")
	default:
	}

	resultChan := make(chan jobResult, 1)
	j := job{
		input:      input,
		resultChan: resultChan,
	}

	select {
	case <-wp.ctx.Done():
		return nil, errors.New("worker pool deleted")
	case wp.queue <- j:
		// Job enqueued, wait for result
		select {
		case <-wp.ctx.Done():
			return nil, errors.New("worker pool deleted")
		case res := <-resultChan:
			return res.output, res.err
		}
	default:
		// Queue is full, backpressure
		return nil, ErrQueueFull
	}
}
