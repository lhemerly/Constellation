package node

import (
	"errors"
	"sync"
)

// BroadcastNode extends BaseNode to broadcast inputs to multiple target nodes
// and collect their outputs.
type BroadcastNode struct {
	*BaseNode
	targets []Node
}

// NewBroadcastNode creates a new BroadcastNode with a given ID and target nodes.
func NewBroadcastNode(id string, targets []Node) *BroadcastNode {
	if len(targets) == 0 {
		panic("BroadcastNode requires at least one target node")
	}

	n := &BroadcastNode{
		BaseNode: NewBaseNode(id),
		targets:  targets,
	}

	n.SetProcessFunc(n.broadcastProcess)

	return n
}

// broadcastProcess sends the input to all target nodes concurrently,
// collects the results, and joins them without mutexes using index-based assignment.
func (n *BroadcastNode) broadcastProcess(input []byte) ([]byte, error) {
	count := len(n.targets)

	type res struct {
		output []byte
		err    error
	}

	results := make([]res, count)
	var wg sync.WaitGroup

	for i, target := range n.targets {
		wg.Add(1)
		go func(idx int, targetNode Node) {
			defer wg.Done()
			out, err := targetNode.Process(input)
			results[idx] = res{output: out, err: err}
		}(i, target)
	}

	wg.Wait()

	var errs []error
	var totalLen int

	for _, r := range results {
		if r.err != nil {
			errs = append(errs, r.err)
		} else {
			totalLen += len(r.output)
		}
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	finalOutput := make([]byte, 0, totalLen)
	for _, r := range results {
		if r.output != nil {
			finalOutput = append(finalOutput, r.output...)
		}
	}

	return finalOutput, nil
}
