package node

import (
	"errors"
)

// BroadcastNode is a fan-out node that forwards inputs to its targets concurrently in a fire-and-forget manner.
type BroadcastNode struct {
	*BaseNode
	targets []Node
}

// NewBroadcastNode creates a new BroadcastNode.
func NewBroadcastNode(id string, targets []Node) *BroadcastNode {
	bn := &BroadcastNode{
		BaseNode: NewBaseNode(id),
		targets:  targets,
	}

	bn.SetProcessFunc(bn.broadcastProcess)

	return bn
}

// broadcastProcess sends the input to all targets concurrently. It does not wait for results and simply returns the input.
func (bn *BroadcastNode) broadcastProcess(input []byte) ([]byte, error) {
	if len(bn.targets) == 0 {
		return input, nil
	}

	// Copy targets to avoid race conditions if modified
	targets := make([]Node, len(bn.targets))
	copy(targets, bn.targets)

	for _, target := range targets {
		go func(n Node) {
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)
			// Fire and forget
			_, _ = n.Process(inputCopy)
		}(target)
	}

	return input, nil
}

// Create initializes the broadcast node and its targets.
func (bn *BroadcastNode) Create() error {
	if err := bn.BaseNode.Create(); err != nil {
		return err
	}
	for _, target := range bn.targets {
		if err := target.Create(); err != nil {
			return err
		}
	}
	return nil
}

// Delete cleans up the broadcast node and its targets.
func (bn *BroadcastNode) Delete() error {
	var errs []error
	if err := bn.BaseNode.Delete(); err != nil {
		errs = append(errs, err)
	}
	for _, target := range bn.targets {
		if err := target.Delete(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		// Just a simple wrapper if there are multiple errors
		return errors.Join(errs...)
	}
	return nil
}
