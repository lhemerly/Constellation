package node

import (
	"errors"
	"sync"
)

// ErrNoFanOutNodes is returned when the FanOutNode has no destination nodes configured.
var ErrNoFanOutNodes = errors.New("no destination nodes available for fan-out")

// FanOutNode extends BaseNode to broadcast a single payload to multiple
// registered destination nodes concurrently, gathering all results and errors.
type FanOutNode struct {
	*BaseNode
	nodesMutex sync.RWMutex
	nodes      []Node
}

// NewFanOutNode creates a new FanOutNode with the given ID.
func NewFanOutNode(id string) *FanOutNode {
	fn := &FanOutNode{
		BaseNode: NewBaseNode(id),
		nodes:    make([]Node, 0),
	}

	// Set the processing function to broadcast requests
	fn.SetProcessFunc(fn.fanOut)
	return fn
}

// AddNode adds a destination node to the fan-out pool.
func (fn *FanOutNode) AddNode(node Node) {
	fn.nodesMutex.Lock()
	defer fn.nodesMutex.Unlock()
	fn.nodes = append(fn.nodes, node)
}

// fanOut distributes the input data to all nodes concurrently.
func (fn *FanOutNode) fanOut(input []byte) ([]byte, error) {
	fn.nodesMutex.RLock()
	nodesCount := len(fn.nodes)
	if nodesCount == 0 {
		fn.nodesMutex.RUnlock()
		return nil, ErrNoFanOutNodes
	}

	// Copy the nodes to process them without holding the lock.
	nodesCopy := make([]Node, nodesCount)
	copy(nodesCopy, fn.nodes)
	fn.nodesMutex.RUnlock()

	var wg sync.WaitGroup
	errs := make([]error, nodesCount)
	results := make([][]byte, nodesCount)

	for i, node := range nodesCopy {
		wg.Add(1)
		go func(idx int, n Node) {
			defer wg.Done()

			// Clone input to prevent in-place modification by concurrent processors
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			res, err := n.Process(inputCopy)
			results[idx] = res
			errs[idx] = err
		}(i, node)
	}

	wg.Wait()

	// Combine errors
	var finalErr error
	for _, err := range errs {
		if err != nil {
			finalErr = errors.Join(finalErr, err)
		}
	}

	// Combine results (concatenate them all)
	// Alternatively, we could just return the input or nil if we only care about completion.
	// But concatenating results might be useful in some scenarios, similar to MapReduce.
	var totalLen int
	for _, res := range results {
		totalLen += len(res)
	}

	finalRes := make([]byte, 0, totalLen)
	for _, res := range results {
		if res != nil {
			finalRes = append(finalRes, res...)
		}
	}

	return finalRes, finalErr
}
