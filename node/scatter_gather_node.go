package node

import (
	"context"
	"errors"
	"time"
)

// ErrScatterGatherTimeout is returned when the scatter-gather operation times out.
// Notice this is used for individual node timeouts within the scatter gather, not the whole operation.
var ErrScatterGatherNodeFail = errors.New("scatter-gather node failed or timed out")

// ScatterGatherNode sends input to multiple nodes concurrently and gathers
// the successful results within a specified timeout. It is tolerant to partial failures.
type ScatterGatherNode struct {
	*BaseNode
	nodes   []Node
	timeout time.Duration
}

// NewScatterGatherNode creates a new ScatterGatherNode with a set of target nodes and a timeout.
func NewScatterGatherNode(id string, nodes []Node, timeout time.Duration) *ScatterGatherNode {
	if len(nodes) == 0 {
		panic("ScatterGatherNode requires at least one node")
	}

	sgNode := &ScatterGatherNode{
		BaseNode: NewBaseNode(id),
		nodes:    nodes,
		timeout:  timeout,
	}

	sgNode.SetProcessFunc(sgNode.scatterGatherProcess)

	return sgNode
}

type scatterResult struct {
	index int
	data  []byte
	err   error
}

func (sg *ScatterGatherNode) scatterGatherProcess(input []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), sg.timeout)
	defer cancel()

	results := make([][]byte, len(sg.nodes))
	resultChan := make(chan scatterResult, len(sg.nodes))

	for i, n := range sg.nodes {
		go func(idx int, target Node) {
			// Clone input for each node
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			// Execute process in a goroutine
			resChan := make(chan struct {
				res []byte
				err error
			}, 1)

			go func() {
				res, err := target.Process(inputCopy)
				resChan <- struct {
					res []byte
					err error
				}{res, err}
			}()

			select {
			case <-ctx.Done():
				resultChan <- scatterResult{index: idx, err: ErrScatterGatherNodeFail}
			case r := <-resChan:
				if r.err != nil {
					resultChan <- scatterResult{index: idx, err: r.err}
				} else {
					resultChan <- scatterResult{index: idx, data: r.res}
				}
			}
		}(i, n)
	}

	successCount := 0
	for i := 0; i < len(sg.nodes); i++ {
		res := <-resultChan
		if res.err == nil {
			results[res.index] = res.data
			successCount++
		}
	}

	if successCount == 0 {
		return nil, errors.New("all nodes failed or timed out in scatter-gather")
	}

	// Gather successful results
	var gathered []byte
	for _, res := range results {
		if res != nil {
			gathered = append(gathered, res...)
		}
	}

	return gathered, nil
}

// Create initializes the scatter-gather node and its dependencies.
func (sg *ScatterGatherNode) Create() error {
	if err := sg.BaseNode.Create(); err != nil {
		return err
	}
	for _, n := range sg.nodes {
		if err := n.Create(); err != nil {
			return err
		}
	}
	return nil
}

// Delete cleans up the scatter-gather node and its dependencies.
func (sg *ScatterGatherNode) Delete() error {
	var errs []error
	if err := sg.BaseNode.Delete(); err != nil {
		errs = append(errs, err)
	}
	for _, n := range sg.nodes {
		if err := n.Delete(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
