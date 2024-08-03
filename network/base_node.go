package network

import (
    "sync"
)

// BaseNode provides common functionality for all node types.
type BaseNode struct {
    id            string
    subscriptions map[string]Node
    mutex         sync.Mutex
    processFunc   func([]byte) ([]byte, error)
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
    return n.processFunc(input)
}

// SetProcessFunc allows setting a custom process function.
func (n *BaseNode) SetProcessFunc(processFunc func([]byte) ([]byte, error)) {
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

// Notify sends an event to all subscribed nodes asynchronously.
func (n *BaseNode) Notify(event []byte) error {
    n.mutex.Lock()
    defer n.mutex.Unlock()
    for _, node := range n.subscriptions {
        go node.Process(event)
    }
    return nil
}

// GetID returns the node's unique identifier.
func (n *BaseNode) GetID() string {
    return n.id
}

// GetSubscription returns a subscribed node by ID, or nil if not found.
func (n *BaseNode) GetSubscription(id string) Node {
    n.mutex.Lock()
    defer n.mutex.Unlock()
    return n.subscriptions[id]
}
