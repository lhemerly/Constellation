package node

import (
	"errors"
	"sync"
)

// ScatterGatherNode fans out requests to multiple target nodes concurrently
// and gathers their successful results by concatenating them.
// If any node fails, the entire process fails and returns an error.
type ScatterGatherNode struct {
	*BaseNode
	targets []Node
}

// NewScatterGatherNode creates a new ScatterGatherNode with a set of target nodes.
func NewScatterGatherNode(id string, targets []Node) *ScatterGatherNode {
	if len(targets) == 0 {
		panic("ScatterGatherNode must have at least one target node")
	}

	sgn := &ScatterGatherNode{
		BaseNode: NewBaseNode(id),
		targets:  targets,
	}

	sgn.SetProcessFunc(sgn.scatterGatherProcess)
	return sgn
}

func (n *ScatterGatherNode) scatterGatherProcess(input []byte) ([]byte, error) {
	var wg sync.WaitGroup

	// Channels for gathering results and errors concurrently
	results := make(chan []byte, len(n.targets))
	errs := make(chan error, len(n.targets))

	for _, target := range n.targets {
		wg.Add(1)
		go func(node Node) {
			defer wg.Done()

			// Clone input to prevent race conditions if targets modify the slice in-place
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			out, err := node.Process(inputCopy)
			if err != nil {
				errs <- err
				return
			}
			results <- out
		}(target)
	}

	// Wait for all goroutines to finish
	wg.Wait()
	close(results)
	close(errs)

	// Check if any errors occurred
	var errList []error
	for err := range errs {
		errList = append(errList, err)
	}

	if len(errList) > 0 {
		return nil, errors.Join(errList...)
	}

	// Gather and concatenate results
	// Pre-calculate length to avoid reallocations
	var totalLen int
	var resultList [][]byte
	for res := range results {
		resultList = append(resultList, res)
		totalLen += len(res)
	}

	finalOutput := make([]byte, 0, totalLen)
	for _, res := range resultList {
		finalOutput = append(finalOutput, res...)
	}

	return finalOutput, nil
}
