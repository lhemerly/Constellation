package node

import (
	"sync"
	"sync/atomic"
)

// Middleware defines a function that wraps a process function.
type Middleware func(func([]byte) ([]byte, error)) func([]byte) ([]byte, error)

// BaseNode provides common functionality for all node types.
type BaseNode struct {
	id            string
	subscriptions map[string]Node
	mutex         sync.RWMutex
	processFunc   func([]byte) ([]byte, error)
	baseProcess   func([]byte) ([]byte, error)
	middlewares   []Middleware
	eventCounter  uint64 // Atomic counter for received events
}

// NewBaseNode creates a new BaseNode with a given ID.
func NewBaseNode(id string) *BaseNode {
	defaultProcess := func(input []byte) ([]byte, error) {
		return input, nil // Default echo behavior
	}
	return &BaseNode{
		id:            id,
		subscriptions: make(map[string]Node),
		baseProcess:   defaultProcess,
		processFunc:   defaultProcess,
	}
}

// Create initializes the node, setting up any necessary resources.
func (n *BaseNode) Create() error {
	// Initialization logic, if any.
	return nil
}

// Delete removes the node, releasing any resources.
func (n *BaseNode) Delete() error {
	// Cleanup logic, if any.
	return nil
}

// Process processes the input and returns the output.
func (n *BaseNode) Process(input []byte) ([]byte, error) {
	atomic.AddUint64(&n.eventCounter, 1)
	return n.processFunc(input)
}

// SetProcessFunc allows setting a custom process function.
func (n *BaseNode) SetProcessFunc(processFunc func([]byte) ([]byte, error)) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	n.baseProcess = processFunc
	n.rebuildProcessFunc()
}

// Use adds middlewares to the node's processing chain.
func (n *BaseNode) Use(middlewares ...Middleware) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	n.middlewares = append(n.middlewares, middlewares...)
	n.rebuildProcessFunc()
}

// rebuildProcessFunc re-applies all middlewares to the base process function.
// Must be called with the mutex locked.
func (n *BaseNode) rebuildProcessFunc() {
	fn := n.baseProcess
	// Apply middlewares in reverse order so that the first middleware added is the outermost
	for i := len(n.middlewares) - 1; i >= 0; i-- {
		fn = n.middlewares[i](fn)
	}
	n.processFunc = fn
}

// Subscribe adds a node to the subscription list for event notifications.
func (n *BaseNode) Subscribe(node Node) error {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	n.subscriptions[node.GetID()] = node
	return nil
}

// Unsubscribe removes a node from the subscription list.
func (n *BaseNode) Unsubscribe(node Node) error {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	delete(n.subscriptions, node.GetID())
	return nil
}

// Notify sends an event to all subscribed nodes and waits for all to complete.
func (n *BaseNode) Notify(event []byte) error {
	n.mutex.RLock()
	// Copy subscriptions to avoid holding the lock during potentially long-running or blocking Process calls
	subs := make([]Node, 0, len(n.subscriptions))
	for _, node := range n.subscriptions {
		subs = append(subs, node)
	}
	n.mutex.RUnlock()

	var wg sync.WaitGroup
	for _, node := range subs {
		wg.Add(1)
		go func(n Node) {
			defer wg.Done()
			n.Process(event)
		}(node)
	}
	wg.Wait()
	return nil
}

// GetID returns the node's unique identifier.
func (n *BaseNode) GetID() string {
	return n.id
}

// GetSubscription returns a subscribed node by ID, or nil if not found.
func (n *BaseNode) GetSubscription(id string) Node {
	n.mutex.RLock()
	defer n.mutex.RUnlock()
	return n.subscriptions[id]
}

// GetEventCount returns the number of events received by this node.
func (n *BaseNode) GetEventCount() uint64 {
	return atomic.LoadUint64(&n.eventCounter)
}
