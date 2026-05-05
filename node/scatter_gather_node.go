package node

import (
	"errors"
	"sync"
)

// ScatterGatherNode broadcasts a single input to multiple scatter nodes in parallel,
// and aggregates their results using a provided gather function.
type ScatterGatherNode struct {
	*BaseNode
	scatterNodes []Node
	gatherFunc   func([][]byte) ([]byte, error)
}

// NewScatterGatherNode creates a new ScatterGatherNode with a unique ID.
// It requires a slice of scatter nodes to process the input concurrently,
// and a gather function to aggregate their successful outputs.
func NewScatterGatherNode(id string, scatterNodes []Node, gatherFunc func([][]byte) ([]byte, error)) (*ScatterGatherNode, error) {
	if len(scatterNodes) == 0 {
		return nil, errors.New("scatter nodes cannot be empty")
	}
	if gatherFunc == nil {
		return nil, errors.New("gather function cannot be nil")
	}

	n := &ScatterGatherNode{
		BaseNode:     NewBaseNode(id),
		scatterNodes: scatterNodes,
		gatherFunc:   gatherFunc,
	}

	n.SetProcessFunc(n.scatterGatherProcess)
	return n, nil
}

// scatterGatherProcess executes the scatter and gather phases.
func (n *ScatterGatherNode) scatterGatherProcess(input []byte) ([]byte, error) {
	var wg sync.WaitGroup
	numNodes := len(n.scatterNodes)

	// Pre-allocate result slices to avoid mutexes
	results := make([][]byte, numNodes)
	errs := make([]error, numNodes)

	for i, node := range n.scatterNodes {
		wg.Add(1)
		go func(idx int, target Node) {
			defer wg.Done()

			// Copy input to prevent race conditions if scatter nodes modify it
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			out, err := target.Process(inputCopy)
			results[idx] = out
			errs[idx] = err
		}(i, node)
	}

	wg.Wait()

	// Check for errors during scatter phase
	var joinedErrs error
	var successfulResults [][]byte

	for i, err := range errs {
		if err != nil {
			joinedErrs = errors.Join(joinedErrs, err)
		} else {
			successfulResults = append(successfulResults, results[i])
		}
	}

	if joinedErrs != nil {
		return nil, joinedErrs
	}

	// Gather phase
	return n.gatherFunc(successfulResults)
}
