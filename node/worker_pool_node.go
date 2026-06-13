package node

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrWorkerPoolClosed = errors.New("worker pool is closed")
	ErrWorkerPoolFull   = errors.New("worker pool queue is full")
)

type job struct {
	input []byte
	res   chan<- jobResult
	ctx   context.Context
}

type jobResult struct {
	output []byte
	err    error
}

// WorkerPoolNode distributes incoming tasks across a pool of concurrent workers.
// Tasks are queued up to the specified queueSize, after which Process returns ErrWorkerPoolFull.
type WorkerPoolNode struct {
	*BaseNode
	workers     []Node
	queue       chan job
	queueSize   int
	cancel      context.CancelFunc
	ctx         context.Context
	workerWg    sync.WaitGroup
	isDeleted   bool
	deleteMutex sync.RWMutex
}

// NewWorkerPoolNode creates a new WorkerPoolNode with the given ID, pool of worker nodes,
// and max queue size for pending requests.
func NewWorkerPoolNode(id string, workers []Node, queueSize int) *WorkerPoolNode {
	if len(workers) == 0 {
		panic("WorkerPoolNode requires at least one worker")
	}

	wp := &WorkerPoolNode{
		BaseNode:  NewBaseNode(id),
		workers:   workers,
		queueSize: queueSize,
	}

	// Override Process directly instead of using SetProcessFunc,
	// because we want users to be able to set their own process function
	// and middlewares on this node without overwriting our queueing logic.
	return wp
}

// Process queues the input for processing by a worker.
func (wp *WorkerPoolNode) Process(input []byte) ([]byte, error) {
	wp.deleteMutex.RLock()
	if wp.isDeleted {
		wp.deleteMutex.RUnlock()
		return nil, ErrWorkerPoolClosed
	}

	// We must not hold deleteMutex during the channel send or context check,
	// otherwise we could deadlock if the channel is full and Delete() is called.
	wp.deleteMutex.RUnlock()

	// Clone input to prevent data races if modified by sender and worker concurrently
	inputCopy := make([]byte, len(input))
	copy(inputCopy, input)

	// Execute through base node to get middlewares/metrics.
	// The actual enqueue logic is mapped in Create() as SetProcessFunc.
	return wp.BaseNode.Process(inputCopy)
}

// Create initializes the node and starts the worker pool goroutines.
func (wp *WorkerPoolNode) Create() error {
	if err := wp.BaseNode.Create(); err != nil {
		return err
	}

	for _, w := range wp.workers {
		if err := w.Create(); err != nil {
			return err
		}
	}

	wp.deleteMutex.Lock()
	defer wp.deleteMutex.Unlock()

	wp.isDeleted = false
	wp.ctx, wp.cancel = context.WithCancel(context.Background())
	wp.queue = make(chan job, wp.queueSize)

	// Rebind BaseNode's process to point to the enqueueing logic, so Middlewares on BaseNode
	// run *before* queuing (e.g. rate limit, circuit breaker apply immediately).
	wp.BaseNode.SetProcessFunc(func(in []byte) ([]byte, error) {
		resCh := make(chan jobResult, 1)
		j := job{
			input: in,
			res:   resCh,
			ctx:   wp.ctx,
		}

		select {
		case <-wp.ctx.Done():
			return nil, ErrWorkerPoolClosed
		case wp.queue <- j:
			select {
			case <-wp.ctx.Done():
				return nil, ErrWorkerPoolClosed
			case res := <-resCh:
				return res.output, res.err
			}
		default:
			return nil, ErrWorkerPoolFull
		}
	})


	for _, worker := range wp.workers {
		wp.workerWg.Add(1)
		go wp.workerLoop(worker)
	}

	return nil
}

func (wp *WorkerPoolNode) workerLoop(worker Node) {
	defer wp.workerWg.Done()
	for {
		select {
		case <-wp.ctx.Done():
			return
		case j, ok := <-wp.queue:
			if !ok {
				return // queue closed
			}

			// Process job
			out, err := worker.Process(j.input)

			// Send result
			select {
			case <-j.ctx.Done():
				// Job's context cancelled (pool shutting down)
			case j.res <- jobResult{output: out, err: err}:
			}
		}
	}
}

// Delete gracefully shuts down the worker pool and its children.
func (wp *WorkerPoolNode) Delete() error {
	wp.deleteMutex.Lock()
	if wp.isDeleted {
		wp.deleteMutex.Unlock()
		return nil
	}
	wp.isDeleted = true
	if wp.cancel != nil {
		wp.cancel() // signal workers and pending Process calls to stop
	}
	wp.deleteMutex.Unlock()

	// Wait for workers to exit
	wp.workerWg.Wait()

	// Safely drain the queue now that workers are dead and no new jobs can be added
	if wp.queue != nil {
		close(wp.queue)
		for j := range wp.queue {
			j.res <- jobResult{output: nil, err: ErrWorkerPoolClosed}
		}
	}

	var errs []error
	if err := wp.BaseNode.Delete(); err != nil {
		errs = append(errs, err)
	}

	for _, w := range wp.workers {
		if err := w.Delete(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}
