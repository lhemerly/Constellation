package node

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrScatterGatherTimeout = errors.New("scatter-gather operation timed out")
)

// ScatterGatherNode broadcasts a message to multiple worker nodes concurrently,
// gathers their responses, and returns an aggregated result deterministically.
type ScatterGatherNode struct {
	*BaseNode
	workers      []Node
	timeout      time.Duration
	ignoreErrors bool // if true, errors from individual workers are ignored, their partial results are appended, or skipped
}

// NewScatterGatherNode creates a new ScatterGatherNode.
func NewScatterGatherNode(id string, workers []Node, timeout time.Duration, ignoreErrors bool) *ScatterGatherNode {
	if len(workers) == 0 {
		panic("ScatterGatherNode requires at least one worker node")
	}

	sgNode := &ScatterGatherNode{
		BaseNode:     NewBaseNode(id),
		workers:      workers,
		timeout:      timeout,
		ignoreErrors: ignoreErrors,
	}

	sgNode.SetProcessFunc(sgNode.scatterGatherProcess)

	return sgNode
}

// scatterGatherProcess broadcasts to workers and gathers responses concurrently.
func (sg *ScatterGatherNode) scatterGatherProcess(input []byte) ([]byte, error) {
	workerCount := len(sg.workers)
	results := make([][]byte, workerCount)
	errs := make([]error, workerCount)

	ctx, cancel := context.WithTimeout(context.Background(), sg.timeout)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(workerCount)

	for i, worker := range sg.workers {
		go func(idx int, w Node) {
			defer wg.Done()

			type workerResult struct {
				res []byte
				err error
			}
			resCh := make(chan workerResult, 1)

			go func() {
				// Clone input
				inputCopy := make([]byte, len(input))
				copy(inputCopy, input)

				res, err := w.Process(inputCopy)
				resCh <- workerResult{res, err}
			}()

			select {
			case r := <-resCh:
				results[idx] = r.res
				errs[idx] = r.err
			case <-ctx.Done():
				errs[idx] = ErrScatterGatherTimeout
			}
		}(i, worker)
	}

	wg.Wait()

	var finalErrs []error
	var totalLen int

	for i := 0; i < workerCount; i++ {
		if errs[i] != nil {
			if !sg.ignoreErrors {
				finalErrs = append(finalErrs, errs[i])
			}
		} else {
			totalLen += len(results[i])
		}
	}

	if len(finalErrs) > 0 {
		return nil, errors.Join(append([]error{errors.New("scatter-gather failed")}, finalErrs...)...)
	}

	// Flatten results deterministically based on index order
	aggregatedOutput := make([]byte, 0, totalLen)
	for i := 0; i < workerCount; i++ {
		if errs[i] == nil && results[i] != nil {
			aggregatedOutput = append(aggregatedOutput, results[i]...)
		}
	}

	return aggregatedOutput, nil
}

// Create initializes the node and its workers.
func (sg *ScatterGatherNode) Create() error {
	if err := sg.BaseNode.Create(); err != nil {
		return err
	}
	for _, w := range sg.workers {
		if err := w.Create(); err != nil {
			return err
		}
	}
	return nil
}

// Delete cleans up the node and its workers.
func (sg *ScatterGatherNode) Delete() error {
	var errs []error
	if err := sg.BaseNode.Delete(); err != nil {
		errs = append(errs, err)
	}
	for _, w := range sg.workers {
		if err := w.Delete(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
