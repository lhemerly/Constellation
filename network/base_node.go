package network

import (
    "sync"
    "sync/atomic"
)

// BaseNode provides common functionality for all node types.
type BaseNode struct {
    id            string
    subscriptions map[string]Node
    mutex         sync.RWMutex
    processFunc   func([]byte) ([]byte, error)
    eventCounter  uint64 // Atomic counter for received events
}

// NewBaseNode creates a new BaseNode with a given ID.
func NewBaseNode(id string) *BaseNode {
    return &BaseNode{
        id:            id,
        subscriptions: make(map[string]Node),
        processFunc: func(input []byte) ([]byte, error) {
            return input, nil // Default echo behavior
        },
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
    n.processFunc = processFunc
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
    defer n.mutex.RUnlock()

    var wg sync.WaitGroup
    for _, node := range n.subscriptions {
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