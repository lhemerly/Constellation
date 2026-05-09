package node

import (
	"errors"
)

var (
	// ErrFilterDropped is returned when the input does not pass the filter predicate
	ErrFilterDropped = errors.New("input dropped by filter")
	// ErrNoTargetNode is returned when FilterNode is missing a target node
	ErrNoTargetNode = errors.New("FilterNode requires a target node")
)

// FilterNode represents a node that applies a predicate function to the input.
// If the predicate returns true, it forwards the data to the underlying node.
// If false, it drops the input gracefully and returns ErrFilterDropped.
type FilterNode struct {
	*BaseNode
	target    Node
	predicate func([]byte) bool
}

// NewFilterNode creates a new FilterNode.
func NewFilterNode(id string, target Node, predicate func([]byte) bool) *FilterNode {
	if target == nil {
		panic(ErrNoTargetNode.Error())
	}
	if predicate == nil {
		panic("FilterNode requires a predicate function")
	}

	filterNode := &FilterNode{
		BaseNode:  NewBaseNode(id),
		target:    target,
		predicate: predicate,
	}

	filterNode.SetProcessFunc(filterNode.filterProcess)

	return filterNode
}

// filterProcess handles the filter lifecycle.
func (fn *FilterNode) filterProcess(input []byte) ([]byte, error) {
	if fn.predicate(input) {
		return fn.target.Process(input)
	}

	return nil, ErrFilterDropped
}

// Create initializes the filter node and its dependencies.
func (fn *FilterNode) Create() error {
	if err := fn.BaseNode.Create(); err != nil {
		return err
	}
	return fn.target.Create()
}

// Delete cleans up the filter node and its dependencies.
func (fn *FilterNode) Delete() error {
	var errs []error
	if err := fn.BaseNode.Delete(); err != nil {
		errs = append(errs, err)
	}
	if err := fn.target.Delete(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
