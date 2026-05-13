package node

import (
	"errors"
	"sync"
)

// BroadcastNode extends BaseNode to distribute incoming requests across
// multiple destination nodes concurrently. It waits for all nodes to finish
// and aggregates their results and errors.
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

	bn.SetProcessFunc(bn.broadcastProcess)
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

// broadcastProcess handles broadcasting the input to all nodes.
func (bn *BroadcastNode) broadcastProcess(input []byte) ([]byte, error) {
	nodes := bn.GetNodes()

	if len(nodes) == 0 {
		return nil, errors.New("no destination nodes available")
	}

	var (
		wg      sync.WaitGroup
		errs    []error
		results [][]byte
		mu      sync.Mutex
	)

	results = make([][]byte, len(nodes))
	errs = make([]error, 0, len(nodes))

	for i, n := range nodes {
		wg.Add(1)
		go func(idx int, target Node) {
			defer wg.Done()

			// Clone input to prevent data races
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			res, err := target.Process(inputCopy)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
			} else {
				results[idx] = res
			}
		}(i, n)
	}

	wg.Wait()

	if len(errs) > 0 {
		return nil, errors.Join(append([]error{errors.New("broadcast failed")}, errs...)...)
	}

	var aggregatedResult []byte
	for _, res := range results {
		aggregatedResult = append(aggregatedResult, res...)
	}

	return aggregatedResult, nil
}

// Create initializes the broadcast node and its dependencies.
func (bn *BroadcastNode) Create() error {
	if err := bn.BaseNode.Create(); err != nil {
		return err
	}
	for _, n := range bn.GetNodes() {
		if err := n.Create(); err != nil {
			return err
		}
	}
	return nil
}

// Delete cleans up the broadcast node and its dependencies.
func (bn *BroadcastNode) Delete() error {
	var errs []error
	if err := bn.BaseNode.Delete(); err != nil {
		errs = append(errs, err)
	}
	for _, n := range bn.GetNodes() {
		if err := n.Delete(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
