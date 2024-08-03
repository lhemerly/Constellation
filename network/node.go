package network

// Node defines the basic operations of any node in the network.
type Node interface {
	// Create initializes the node, setting up any necessary resources.
	Create() error

	// Delete removes the node, releasing any resources.
	Delete() error

	// Process takes an input, processes it, and returns the output.
	Process(input []byte) ([]byte, error)

	// GetID returns the node's unique identifier.
	GetID() string

	// Subscribe adds a node to the subscription list for event notifications.
	Subscribe(node Node) error

	// Unsubscribe removes a node from the subscription list.
	Unsubscribe(node Node) error

	// Notify sends an event to all subscribed nodes asynchronously.
	Notify(event []byte) error
}
