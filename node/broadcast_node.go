package node

import (
	"sync"
)

// BroadcastNode extends BaseNode to broadcast an input to multiple nodes concurrently.
// It does not gather results; it's fire-and-forget for processing paths.
type BroadcastNode struct {
	*BaseNode
	nodesMutex sync.RWMutex
	nodes      []Node
}

// NewBroadcastNode creates a new BroadcastNode with a given ID.
func NewBroadcastNode(id string) *BroadcastNode {
	b := &BroadcastNode{
		BaseNode: NewBaseNode(id),
		nodes:    make([]Node, 0),
	}
	b.SetProcessFunc(b.broadcastProcess)
	return b
}

// AddNode adds a destination node for broadcasting.
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

// broadcastProcess executes the broadcast logic.
func (b *BroadcastNode) broadcastProcess(input []byte) ([]byte, error) {
	b.nodesMutex.RLock()
	nodesCopy := make([]Node, len(b.nodes))
	copy(nodesCopy, b.nodes)
	b.nodesMutex.RUnlock()

	var wg sync.WaitGroup
	for _, n := range nodesCopy {
		wg.Add(1)
		go func(node Node) {
			defer wg.Done()
			_, _ = node.Process(input) // Ignore output and errors for broadcast
		}(n)
	}
	wg.Wait()

	return input, nil // Return the original input
}
