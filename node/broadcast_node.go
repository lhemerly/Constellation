package node

import (
	"errors"
	"sync"
)

// ErrNoBroadcastNodes is returned when the BroadcastNode has no destinations.
var ErrNoBroadcastNodes = errors.New("no destination nodes available for broadcast")

// BroadcastNode extends BaseNode to distribute incoming requests across
// multiple destination nodes concurrently and collects their responses.
type BroadcastNode struct {
	*BaseNode
	nodesMutex sync.RWMutex
	nodes      []Node
}

// NewBroadcastNode creates a new BroadcastNode with the given ID.
func NewBroadcastNode(id string) *BroadcastNode {
	b := &BroadcastNode{
		BaseNode: NewBaseNode(id),
		nodes:    make([]Node, 0),
	}

	b.SetProcessFunc(b.broadcast)
	return b
}

// AddNode adds a destination node to the broadcast pool.
func (b *BroadcastNode) AddNode(node Node) {
	b.nodesMutex.Lock()
	defer b.nodesMutex.Unlock()
	b.nodes = append(b.nodes, node)
}

// GetNodes returns the current list of destination nodes.
func (b *BroadcastNode) GetNodes() []Node {
	b.nodesMutex.RLock()
	defer b.nodesMutex.RUnlock()

	nodesCopy := make([]Node, len(b.nodes))
	copy(nodesCopy, b.nodes)
	return nodesCopy
}

// broadcast distributes the input data to all destination nodes concurrently.
func (b *BroadcastNode) broadcast(input []byte) ([]byte, error) {
	b.nodesMutex.RLock()
	nodesCount := len(b.nodes)

	if nodesCount == 0 {
		b.nodesMutex.RUnlock()
		return nil, ErrNoBroadcastNodes
	}

	nodesCopy := make([]Node, nodesCount)
	copy(nodesCopy, b.nodes)
	b.nodesMutex.RUnlock()

	var wg sync.WaitGroup
	errs := make([]error, nodesCount)
	results := make([][]byte, nodesCount)

	for i, node := range nodesCopy {
		wg.Add(1)
		go func(idx int, n Node) {
			defer wg.Done()

			// Clone input to avoid race conditions if a node modifies it
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			res, err := n.Process(inputCopy)
			errs[idx] = err
			results[idx] = res
		}(i, node)
	}

	wg.Wait()

	var collectedErrs []error
	var finalOutput []byte

	for i := 0; i < nodesCount; i++ {
		if errs[i] != nil {
			collectedErrs = append(collectedErrs, errs[i])
		} else {
			finalOutput = append(finalOutput, results[i]...)
		}
	}

	if len(collectedErrs) > 0 {
		return finalOutput, errors.Join(append([]error{errors.New("broadcast partial failure")}, collectedErrs...)...)
	}

	return finalOutput, nil
}
