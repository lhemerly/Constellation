package node

import (
	"errors"
	"sync"
)

// ErrPoolClosed is returned when trying to process data on a closed WorkerPoolNode.
var ErrPoolClosed = errors.New("worker pool is closed")

// ErrPoolFull is returned when the worker pool queue is full and non-blocking behavior is expected.
var ErrPoolFull = errors.New("worker pool queue is full")

type job struct {
	input  []byte
	result chan result
}

type result struct {
	output []byte
	err    error
}

// WorkerPoolNode extends BaseNode to process incoming data concurrently using
// a fixed-size pool of worker goroutines, bounding concurrency and resource usage.
type WorkerPoolNode struct {
	*BaseNode
	workers      int
	queueSize    int
	jobsChan     chan job
	wg           sync.WaitGroup
	internalNode Node
	mu           sync.Mutex
	closed       bool
}

// NewWorkerPoolNode creates a new WorkerPoolNode with the specified number of workers
// and queue size. The internalNode is the node that will perform the actual processing.
func NewWorkerPoolNode(id string, workers int, queueSize int, internalNode Node) *WorkerPoolNode {
	if workers <= 0 {
		panic("WorkerPoolNode requires at least 1 worker")
	}
	if queueSize < 0 {
		panic("WorkerPoolNode requires a non-negative queue size")
	}
	if internalNode == nil {
		panic("WorkerPoolNode requires an internal node")
	}

	wp := &WorkerPoolNode{
		BaseNode:     NewBaseNode(id),
		workers:      workers,
		queueSize:    queueSize,
		internalNode: internalNode,
		jobsChan:     make(chan job, queueSize),
	}

	wp.SetProcessFunc(wp.poolProcess)

	return wp
}

// Create starts the worker goroutines and calls Create on the internal node.
func (wp *WorkerPoolNode) Create() error {
	if err := wp.BaseNode.Create(); err != nil {
		return err
	}
	if err := wp.internalNode.Create(); err != nil {
		return err
	}

	wp.mu.Lock()
	defer wp.mu.Unlock()
	if wp.closed {
		return ErrPoolClosed
	}

	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}

	return nil
}

// Delete shuts down the worker pool and calls Delete on the internal node.
func (wp *WorkerPoolNode) Delete() error {
	var errs []error

	wp.mu.Lock()
	if !wp.closed {
		wp.closed = true
		close(wp.jobsChan)
	}
	wp.mu.Unlock()

	wp.wg.Wait()

	if err := wp.internalNode.Delete(); err != nil {
		errs = append(errs, err)
	}
	if err := wp.BaseNode.Delete(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func (wp *WorkerPoolNode) worker() {
	defer wp.wg.Done()
	for j := range wp.jobsChan {
		// Create a copy to prevent data races during parallel processing
		inputCopy := make([]byte, len(j.input))
		copy(inputCopy, j.input)

		out, err := wp.internalNode.Process(inputCopy)
		j.result <- result{output: out, err: err}
	}
}

func (wp *WorkerPoolNode) poolProcess(input []byte) ([]byte, error) {
	resChan := make(chan result, 1)
	j := job{input: input, result: resChan}

	// We use a defer with recover to safely handle the case where
	// Delete() is called and closes the jobsChan concurrently
	// while we are trying to send a job to it.
	err := func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				// This panic means the channel was closed concurrently.
				err = ErrPoolClosed
			}
		}()

		wp.mu.Lock()
		if wp.closed {
			wp.mu.Unlock()
			return ErrPoolClosed
		}
		wp.mu.Unlock()

		wp.jobsChan <- j
		return nil
	}()

	if err != nil {
		return nil, err
	}

	res := <-resChan
	return res.output, res.err
}
