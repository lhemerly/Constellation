package node

// TeeNode is a node that duplicates incoming requests and sends a copy to an observer node asynchronously.
// The main request is passed through to a primary node (if any) or simply returned.
// This is useful for traffic shadowing, logging, or monitoring without impacting the main request's critical path.
type TeeNode struct {
	*BaseNode
	observer Node
	primary  Node
}

// NewTeeNode creates a new TeeNode with the given ID.
// Requests are sent to the observer asynchronously and then passed to the primary node.
// If primary is nil, the input is simply returned.
func NewTeeNode(id string, observer Node, primary Node) *TeeNode {
	t := &TeeNode{
		BaseNode: NewBaseNode(id),
		observer: observer,
		primary:  primary,
	}

	t.SetProcessFunc(t.teeProcess)
	return t
}

func (t *TeeNode) teeProcess(input []byte) ([]byte, error) {
	if t.observer != nil {
		// Clone input to avoid race condition if observer modifies it
		inputCopy := make([]byte, len(input))
		copy(inputCopy, input)

		go func() {
			// Best-effort execution on the observer
			_, _ = t.observer.Process(inputCopy)
		}()
	}

	if t.primary != nil {
		return t.primary.Process(input)
	}

	return input, nil
}
