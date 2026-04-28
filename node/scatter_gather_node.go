package node

import (
	"sync"
)

// ScatterGatherNode sends the input to multiple target nodes concurrently
// and gathers their results using a provided gather function.
type ScatterGatherNode struct {
	*BaseNode
	targets    []Node
	gatherFunc func(results [][]byte, errs []error) ([]byte, error)
}

// NewScatterGatherNode creates a new ScatterGatherNode.
func NewScatterGatherNode(id string, targets []Node, gatherFunc func([][]byte, []error) ([]byte, error)) *ScatterGatherNode {
	sgNode := &ScatterGatherNode{
		BaseNode:   NewBaseNode(id),
		targets:    targets,
		gatherFunc: gatherFunc,
	}

	sgNode.SetProcessFunc(sgNode.scatterGatherProcess)
	return sgNode
}

// scatterGatherProcess sends the input to all target nodes and aggregates the results.
func (n *ScatterGatherNode) scatterGatherProcess(input []byte) ([]byte, error) {
	numTargets := len(n.targets)
	if numTargets == 0 {
		return n.gatherFunc(nil, nil)
	}

	results := make([][]byte, numTargets)
	errs := make([]error, numTargets)

	var wg sync.WaitGroup
	wg.Add(numTargets)

	for i, target := range n.targets {
		go func(index int, t Node) {
			defer wg.Done()

			// Clone input to prevent race conditions if target modifies the input slice
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			res, err := t.Process(inputCopy)
			results[index] = res
			errs[index] = err
		}(i, target)
	}

	wg.Wait()

	return n.gatherFunc(results, errs)
}
