package node

import (
	"errors"
	"sync"
)

// WorkerPoolNode uses a fixed pool of goroutines to process requests.
type WorkerPoolNode struct {
	*BaseNode
	workerNode Node
	poolSize   int
	jobChan    chan *workerJob
	wg         sync.WaitGroup
	quitChan   chan struct{}
}

type workerJob struct {
	input      []byte
	resultChan chan workerResult
}

type workerResult struct {
	output []byte
	err    error
}

// NewWorkerPoolNode creates a new WorkerPoolNode with the specified pool size.
func NewWorkerPoolNode(id string, workerNode Node, poolSize int) *WorkerPoolNode {
	if poolSize <= 0 {
		panic("WorkerPoolNode requires poolSize > 0")
	}
	if workerNode == nil {
		panic("WorkerPoolNode requires a worker node")
	}

	wNode := &WorkerPoolNode{
		BaseNode:   NewBaseNode(id),
		workerNode: workerNode,
		poolSize:   poolSize,
		jobChan:    make(chan *workerJob),
		quitChan:   make(chan struct{}),
	}

	wNode.SetProcessFunc(wNode.workerPoolProcess)

	return wNode
}

// workerPoolProcess sends the job to the pool and waits for the result.
func (w *WorkerPoolNode) workerPoolProcess(input []byte) ([]byte, error) {
	resultChan := make(chan workerResult, 1)

	// We copy the input here because we pass it down the channel asynchronously
	inputCopy := make([]byte, len(input))
	copy(inputCopy, input)

	job := &workerJob{
		input:      inputCopy,
		resultChan: resultChan,
	}

	select {
	case w.jobChan <- job:
		res := <-resultChan
		return res.output, res.err
	case <-w.quitChan:
		return nil, errors.New("worker pool is stopped")
	}
}

// startPool starts the worker goroutines.
func (w *WorkerPoolNode) startPool() {
	for i := 0; i < w.poolSize; i++ {
		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			for {
				select {
				case job := <-w.jobChan:
					output, err := w.workerNode.Process(job.input)
					job.resultChan <- workerResult{output: output, err: err}
				case <-w.quitChan:
					return
				}
			}
		}()
	}
}

// Create initializes the worker pool node and its dependencies, and starts the pool.
func (w *WorkerPoolNode) Create() error {
	if err := w.BaseNode.Create(); err != nil {
		return err
	}
	if err := w.workerNode.Create(); err != nil {
		return err
	}

	w.startPool()
	return nil
}

// Delete cleans up the worker pool node, stops the pool, and deletes its dependencies.
func (w *WorkerPoolNode) Delete() error {
	close(w.quitChan)
	w.wg.Wait()

	var errs []error
	if err := w.BaseNode.Delete(); err != nil {
		errs = append(errs, err)
	}
	if err := w.workerNode.Delete(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
