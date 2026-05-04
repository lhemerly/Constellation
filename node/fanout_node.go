package node

import (
	"errors"
	"sync"
)

var (
	ErrNoNodesFanOut = errors.New("no child nodes provided for fan out")
	ErrFanOutFailed  = errors.New("fan out processing failed")
)

// FanOutNode distributes the same input to multiple child nodes in parallel.
// It waits for all child nodes to complete processing. If any node fails,
// it aggregates the errors. It returns a concatenation of all successful outputs.
type FanOutNode struct {
	*BaseNode
	children []Node
}

// NewFanOutNode creates a new FanOutNode with the given ID and child nodes.
func NewFanOutNode(id string, children []Node) (*FanOutNode, error) {
	if len(children) == 0 {
		return nil, ErrNoNodesFanOut
	}

	fanOutNode := &FanOutNode{
		BaseNode: NewBaseNode(id),
		children: children,
	}

	// Set the custom process function to handle parallel execution
	fanOutNode.SetProcessFunc(fanOutNode.fanOutProcess)

	return fanOutNode, nil
}

// fanOutProcess handles the parallel execution of the input across all child nodes.
func (f *FanOutNode) fanOutProcess(input []byte) ([]byte, error) {
	var wg sync.WaitGroup
	results := make([][]byte, len(f.children))
	errs := make([]error, len(f.children))

	for i, child := range f.children {
		wg.Add(1)
		go func(index int, node Node) {
			defer wg.Done()

			// Copy input to avoid data races if child nodes modify it in-place
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			output, err := node.Process(inputCopy)
			if err != nil {
				errs[index] = err
			} else {
				results[index] = output
			}
		}(i, child)
	}

	wg.Wait()

	// Aggregate errors
	var finalErr error
	for _, err := range errs {
		if err != nil {
			finalErr = errors.Join(finalErr, err)
		}
	}

	if finalErr != nil {
		return nil, errors.Join(ErrFanOutFailed, finalErr)
	}

	// Calculate total length for pre-allocation
	totalLen := 0
	for _, res := range results {
		totalLen += len(res)
	}

	// Concatenate successful results deterministically
	finalOutput := make([]byte, 0, totalLen)
	for _, res := range results {
		finalOutput = append(finalOutput, res...)
	}

	return finalOutput, nil
}
