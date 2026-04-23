package node

import (
	"errors"
	"sync"
	"sync/atomic"
)

// ErrNoNodes is returned when the LoadBalancerNode has no destinations.
var ErrNoNodes = errors.New("no destination nodes available")

// LoadBalancerNode extends BaseNode to distribute incoming requests across
// multiple destination nodes using a round-robin strategy.
type LoadBalancerNode struct {
	*BaseNode
	nodesMutex sync.RWMutex
	nodes      []Node
	counter    uint64
}

// NewLoadBalancerNode creates a new LoadBalancerNode with the given ID.
func NewLoadBalancerNode(id string) *LoadBalancerNode {
	lb := &LoadBalancerNode{
		BaseNode: NewBaseNode(id),
		nodes:    make([]Node, 0),
	}

	// Set the processing function to load balance requests
	lb.SetProcessFunc(lb.loadBalance)
	return lb
}

// AddNode adds a destination node to the load balancer pool.
func (lb *LoadBalancerNode) AddNode(node Node) {
	lb.nodesMutex.Lock()
	defer lb.nodesMutex.Unlock()
	lb.nodes = append(lb.nodes, node)
}

// RemoveNode removes a destination node from the load balancer pool by its ID.
func (lb *LoadBalancerNode) RemoveNode(nodeID string) {
	lb.nodesMutex.Lock()
	defer lb.nodesMutex.Unlock()

	for i, node := range lb.nodes {
		if node.GetID() == nodeID {
			// Remove the node while preserving order or unordered is fine too.
			// Fast removal since order doesn't strictly matter for round-robin,
			// but we'll preserve order for predictability.
			lb.nodes = append(lb.nodes[:i], lb.nodes[i+1:]...)
			break
		}
	}
}

// GetNodes returns the current list of destination nodes.
func (lb *LoadBalancerNode) GetNodes() []Node {
	lb.nodesMutex.RLock()
	defer lb.nodesMutex.RUnlock()

	// Return a copy to avoid external modifications
	nodesCopy := make([]Node, len(lb.nodes))
	copy(nodesCopy, lb.nodes)
	return nodesCopy
}

// loadBalance distributes the input data to the next available node using round-robin.
func (lb *LoadBalancerNode) loadBalance(input []byte) ([]byte, error) {
	lb.nodesMutex.RLock()
	nodesCount := uint64(len(lb.nodes))

	if nodesCount == 0 {
		lb.nodesMutex.RUnlock()
		return nil, ErrNoNodes
	}

	// Atomically get the current counter, then increment it
	current := atomic.AddUint64(&lb.counter, 1) - 1
	idx := current % nodesCount
	targetNode := lb.nodes[idx]
	lb.nodesMutex.RUnlock()

	return targetNode.Process(input)
}
