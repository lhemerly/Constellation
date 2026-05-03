package node

import (
	"errors"
	"sync"
)

type task struct {
	input      []byte
	resultChan chan struct {
		res []byte
		err error
	}
}

// WorkerPoolNode distributes process requests to a fixed number of background workers.
type WorkerPoolNode struct {
	*BaseNode
	workersCount   int
	taskChan       chan task
	wg             sync.WaitGroup
	workerProcess  func([]byte) ([]byte, error)
	workerProcLock sync.RWMutex
}

// NewWorkerPoolNode creates a new WorkerPoolNode with a given ID and number of workers.
func NewWorkerPoolNode(id string, workersCount int) (*WorkerPoolNode, error) {
	if workersCount <= 0 {
		return nil, errors.New("WorkerPoolNode requires at least one worker")
	}

	wpNode := &WorkerPoolNode{
		BaseNode:     NewBaseNode(id),
		workersCount: workersCount,
		taskChan:     make(chan task),
		workerProcess: func(input []byte) ([]byte, error) {
			return input, nil
		},
	}

	wpNode.SetProcessFunc(wpNode.poolProcess)

	return wpNode, nil
}

// SetWorkerProcessFunc allows configuring the logic that background workers will run.
func (wp *WorkerPoolNode) SetWorkerProcessFunc(fn func([]byte) ([]byte, error)) {
	wp.workerProcLock.Lock()
	defer wp.workerProcLock.Unlock()
	wp.workerProcess = fn
}

// poolProcess is the entrypoint that puts the input into the task channel and waits.
func (wp *WorkerPoolNode) poolProcess(input []byte) ([]byte, error) {
	// Clone input to prevent data races
	inputCopy := make([]byte, len(input))
	copy(inputCopy, input)

	resultChan := make(chan struct {
		res []byte
		err error
	}, 1)

	wp.taskChan <- task{
		input:      inputCopy,
		resultChan: resultChan,
	}

	res := <-resultChan
	return res.res, res.err
}

// Create spins up the worker goroutines.
func (wp *WorkerPoolNode) Create() error {
	if err := wp.BaseNode.Create(); err != nil {
		return err
	}

	wp.wg.Add(wp.workersCount)
	for i := 0; i < wp.workersCount; i++ {
		go func() {
			defer wp.wg.Done()
			for t := range wp.taskChan {
				wp.workerProcLock.RLock()
				fn := wp.workerProcess
				wp.workerProcLock.RUnlock()

				res, err := fn(t.input)
				t.resultChan <- struct {
					res []byte
					err error
				}{res, err}
			}
		}()
	}

	return nil
}

// Delete stops the workers and cleans up the node.
func (wp *WorkerPoolNode) Delete() error {
	close(wp.taskChan)
	wp.wg.Wait()

	return wp.BaseNode.Delete()
}
