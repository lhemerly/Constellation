package node

import (
	"errors"
	"sync"
)

// ScatterGatherNode broadcasts input to multiple child nodes concurrently,
// waits for all of them to complete, and gathers their results into a single slice.
type ScatterGatherNode struct {
	*BaseNode
	children []Node
}

// NewScatterGatherNode creates a new ScatterGatherNode with a given ID and child nodes.
func NewScatterGatherNode(id string, children []Node) (*ScatterGatherNode, error) {
	if len(children) == 0 {
		return nil, errors.New("ScatterGatherNode requires at least one child node")
	}

	sgNode := &ScatterGatherNode{
		BaseNode: NewBaseNode(id),
		children: children,
	}

	sgNode.SetProcessFunc(sgNode.scatterGatherProcess)

	return sgNode, nil
}

// scatterGatherProcess handles the broadcast and aggregation lifecycle.
func (sg *ScatterGatherNode) scatterGatherProcess(input []byte) ([]byte, error) {
	var (
		wg      sync.WaitGroup
		errs    []error
		errLock sync.Mutex
	)

	// Pre-allocate the results slice based on the number of child nodes.
	results := make([][]byte, len(sg.children))

	for i, child := range sg.children {
		wg.Add(1)
		go func(index int, n Node) {
			defer wg.Done()

			// Clone input to prevent data races
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			res, err := n.Process(inputCopy)

			if err != nil {
				errLock.Lock()
				errs = append(errs, err)
				errLock.Unlock()
			} else {
				// Assign result to its corresponding index to avoid lock contention
				results[index] = res
			}
		}(i, child)
	}

	wg.Wait()

	if len(errs) > 0 {
		return nil, errors.Join(append([]error{errors.New("scatter-gather phase failed")}, errs...)...)
	}

	// Calculate total length to pre-allocate flattened array
	totalLen := 0
	for _, res := range results {
		totalLen += len(res)
	}

	flattenedResults := make([]byte, 0, totalLen)
	for _, res := range results {
		flattenedResults = append(flattenedResults, res...)
	}

	return flattenedResults, nil
}

// Create initializes the scatter-gather node and its children.
func (sg *ScatterGatherNode) Create() error {
	if err := sg.BaseNode.Create(); err != nil {
		return err
	}
	for _, child := range sg.children {
		if err := child.Create(); err != nil {
			return err
		}
	}
	return nil
}

// Delete cleans up the scatter-gather node and its children.
func (sg *ScatterGatherNode) Delete() error {
	var errs []error
	if err := sg.BaseNode.Delete(); err != nil {
		errs = append(errs, err)
	}
	for _, child := range sg.children {
		if err := child.Delete(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
