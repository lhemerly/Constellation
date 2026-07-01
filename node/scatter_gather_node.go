package node

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrScatterGatherTimeout is returned when the scatter-gather operation times out.
var ErrScatterGatherTimeout = errors.New("scatter-gather operation timed out")

// ScatterGatherNode broadcasts an input to multiple nodes concurrently,
// waits for their responses up to a specified timeout, and aggregates the results.
type ScatterGatherNode struct {
	*BaseNode
	nodes   []Node
	timeout time.Duration
	gather  func([][]byte) ([]byte, error)
}

// NewScatterGatherNode creates a new ScatterGatherNode.
func NewScatterGatherNode(id string, nodes []Node, timeout time.Duration) *ScatterGatherNode {
	sg := &ScatterGatherNode{
		BaseNode: NewBaseNode(id),
		nodes:    nodes,
		timeout:  timeout,
		gather:   defaultGather,
	}
	sg.SetProcessFunc(sg.scatterGatherProcess)
	return sg
}

// SetGatherFunc allows customizing the aggregation of the responses.
func (sg *ScatterGatherNode) SetGatherFunc(f func([][]byte) ([]byte, error)) {
	sg.gather = f
}

func (sg *ScatterGatherNode) scatterGatherProcess(input []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), sg.timeout)
	defer cancel()

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results [][]byte
		errs    []error
	)

	for _, n := range sg.nodes {
		wg.Add(1)
		go func(node Node) {
			defer wg.Done()

			// Clone input to prevent data races
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			resChan := make(chan struct {
				res []byte
				err error
			}, 1)

			go func() {
				res, err := node.Process(inputCopy)
				resChan <- struct {
					res []byte
					err error
				}{res, err}
			}()

			select {
			case <-ctx.Done():
				mu.Lock()
				errs = append(errs, ErrScatterGatherTimeout)
				mu.Unlock()
			case r := <-resChan:
				mu.Lock()
				if r.err != nil {
					errs = append(errs, r.err)
				} else {
					results = append(results, r.res)
				}
				mu.Unlock()
			}
		}(n)
	}

	wg.Wait()

	if len(errs) > 0 {
		return nil, errors.Join(append([]error{errors.New("scatter-gather failed")}, errs...)...)
	}

	return sg.gather(results)
}

// defaultGather concatenates the byte slices.
func defaultGather(results [][]byte) ([]byte, error) {
	var aggregated []byte
	for _, res := range results {
		aggregated = append(aggregated, res...)
	}
	return aggregated, nil
}
