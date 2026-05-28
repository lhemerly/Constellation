package node

import (
	"errors"
	"sync"
)

// BroadcastNode extends BaseNode to distribute incoming requests to all
// registered destination nodes concurrently.
type BroadcastNode struct {
	*BaseNode
	nodesMutex sync.RWMutex
	nodes      []Node
}

// NewBroadcastNode creates a new BroadcastNode with the given ID.
func NewBroadcastNode(id string) *BroadcastNode {
	bn := &BroadcastNode{
		BaseNode: NewBaseNode(id),
		nodes:    make([]Node, 0),
	}

	bn.SetProcessFunc(bn.broadcast)
	return bn
}

// AddNode adds a destination node to the broadcast pool.
func (bn *BroadcastNode) AddNode(node Node) {
	bn.nodesMutex.Lock()
	defer bn.nodesMutex.Unlock()
	bn.nodes = append(bn.nodes, node)
}

// RemoveNode removes a destination node from the broadcast pool by its ID.
func (bn *BroadcastNode) RemoveNode(nodeID string) {
	bn.nodesMutex.Lock()
	defer bn.nodesMutex.Unlock()

	for i, node := range bn.nodes {
		if node.GetID() == nodeID {
			bn.nodes = append(bn.nodes[:i], bn.nodes[i+1:]...)
			break
		}
	}
}

// GetNodes returns the current list of destination nodes.
func (bn *BroadcastNode) GetNodes() []Node {
	bn.nodesMutex.RLock()
	defer bn.nodesMutex.RUnlock()

	nodesCopy := make([]Node, len(bn.nodes))
	copy(nodesCopy, bn.nodes)
	return nodesCopy
}

// broadcast sends the input data to all nodes concurrently and waits for them.
func (bn *BroadcastNode) broadcast(input []byte) ([]byte, error) {
	bn.nodesMutex.RLock()
	nodesCount := len(bn.nodes)
	if nodesCount == 0 {
		bn.nodesMutex.RUnlock()
		return nil, ErrNoNodes
	}

	targetNodes := make([]Node, nodesCount)
	copy(targetNodes, bn.nodes)
	bn.nodesMutex.RUnlock()

	var wg sync.WaitGroup
	errs := make([]error, nodesCount)
	results := make([][]byte, nodesCount)

	wg.Add(nodesCount)
	for i, targetNode := range targetNodes {
		go func(idx int, n Node) {
			defer wg.Done()

			// We must pass a copy of the input since a destination node could modify it
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			output, err := n.Process(inputCopy)
			errs[idx] = err
			results[idx] = output
		}(i, targetNode)
	}

	wg.Wait()

	// Aggregate errors
	var finalErr error
	for _, err := range errs {
		if err != nil {
			finalErr = errors.Join(finalErr, err)
		}
	}

	// Aggregate output
	var totalLen int
	for _, res := range results {
		totalLen += len(res)
	}

	finalOutput := make([]byte, 0, totalLen)
	for _, res := range results {
		finalOutput = append(finalOutput, res...)
	}

	return finalOutput, finalErr
}
