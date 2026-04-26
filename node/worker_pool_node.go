package node

import (
	"errors"
)

// ErrQueueFull is returned when the WorkerPoolNode's queue is full and it sheds load.
var ErrQueueFull = errors.New("worker pool queue is full")

// WorkerPoolNode wraps another node to limit its concurrency using a bounded pool of workers.
// It also has a queue to hold requests when all workers are busy. If the queue fills up,
// it will shed load by returning ErrQueueFull.
type WorkerPoolNode struct {
	*BaseNode
	targetNode Node
	jobChan    chan *workerJob
	poolSize   int
	queueSize  int
}

type workerJob struct {
	input []byte
	res   chan workerResult
}

type workerResult struct {
	output []byte
	err    error
}

// NewWorkerPoolNode creates a new WorkerPoolNode with the given ID, target node, pool size, and queue size.
func NewWorkerPoolNode(id string, targetNode Node, poolSize int, queueSize int) *WorkerPoolNode {
	if poolSize <= 0 {
		panic("WorkerPoolNode requires poolSize > 0")
	}
	if queueSize < 0 {
		panic("WorkerPoolNode requires queueSize >= 0")
	}
	if targetNode == nil {
		panic("WorkerPoolNode requires a non-nil targetNode")
	}

	w := &WorkerPoolNode{
		BaseNode:   NewBaseNode(id),
		targetNode: targetNode,
		jobChan:    make(chan *workerJob, queueSize),
		poolSize:   poolSize,
		queueSize:  queueSize,
	}

	w.SetProcessFunc(w.workerPoolProcess)
	return w
}

// Create initializes the worker pool and its target node.
func (w *WorkerPoolNode) Create() error {
	if err := w.BaseNode.Create(); err != nil {
		return err
	}
	if err := w.targetNode.Create(); err != nil {
		return err
	}

	// Start workers
	for i := 0; i < w.poolSize; i++ {
		go func() {
			for job := range w.jobChan {
				out, err := w.targetNode.Process(job.input)
				job.res <- workerResult{output: out, err: err}
			}
		}()
	}

	return nil
}

// Delete cleans up the node and its target node.
func (w *WorkerPoolNode) Delete() error {
	close(w.jobChan)
	var errs []error
	if err := w.BaseNode.Delete(); err != nil {
		errs = append(errs, err)
	}
	if err := w.targetNode.Delete(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// workerPoolProcess delegates the processing to the worker pool.
func (w *WorkerPoolNode) workerPoolProcess(input []byte) ([]byte, error) {
	resChan := make(chan workerResult, 1)
	job := &workerJob{
		input: input,
		res:   resChan,
	}

	select {
	case w.jobChan <- job:
		// Job accepted into queue
		res := <-resChan
		return res.output, res.err
	default:
		// Queue is full, shed load
		return nil, ErrQueueFull
	}
}
