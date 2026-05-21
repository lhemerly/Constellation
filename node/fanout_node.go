package node

import (
	"context"
	"errors"
)

// FanOutNode represents a node that dispatches input to a list of destination nodes concurrently,
// and then aggregates their outputs using a provided aggregator function.
type FanOutNode struct {
	*BaseNode
	destinations []Node
	aggregator   func([][]byte) ([]byte, error)
}

// NewFanOutNode creates a new FanOutNode.
func NewFanOutNode(id string, destinations []Node, aggregator func([][]byte) ([]byte, error)) *FanOutNode {
	if len(destinations) == 0 {
		panic("FanOutNode requires at least one destination")
	}
	if aggregator == nil {
		panic("FanOutNode requires an aggregator function")
	}

	fNode := &FanOutNode{
		BaseNode:     NewBaseNode(id),
		destinations: destinations,
		aggregator:   aggregator,
	}

	fNode.SetProcessFunc(fNode.fanOutProcess)

	return fNode
}

// workerResult holds the result of a single destination node's processing.
type workerResult struct {
	index int
	data  []byte
	err   error
}

// fanOutProcess handles the fan-out and aggregation lifecycle.
func (f *FanOutNode) fanOutProcess(input []byte) ([]byte, error) {
	numDestinations := len(f.destinations)
	results := make([][]byte, numDestinations)

	resultChan := make(chan workerResult, numDestinations)

	// Context for cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Dispatch input to all destinations concurrently
	for i, dest := range f.destinations {
		go func(index int, destination Node) {
			// Clone input to prevent data races
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			res, err := destination.Process(inputCopy)

			// Send result to channel, respecting context cancellation
			select {
			case resultChan <- workerResult{index: index, data: res, err: err}:
			case <-ctx.Done():
			}
		}(i, dest)
	}

	// Collect results
	var errs []error
	for i := 0; i < numDestinations; i++ {
		res := <-resultChan
		if res.err != nil {
			errs = append(errs, res.err)
		} else {
			results[res.index] = res.data
		}
	}

	if len(errs) > 0 {
		return nil, errors.Join(append([]error{errors.New("fan-out phase failed")}, errs...)...)
	}

	// Aggregate results
	return f.aggregator(results)
}

// Create initializes the fan-out node and its destinations.
func (f *FanOutNode) Create() error {
	if err := f.BaseNode.Create(); err != nil {
		return err
	}
	for _, dest := range f.destinations {
		if err := dest.Create(); err != nil {
			return err
		}
	}
	return nil
}

// Delete cleans up the fan-out node and its destinations.
func (f *FanOutNode) Delete() error {
	var errs []error
	if err := f.BaseNode.Delete(); err != nil {
		errs = append(errs, err)
	}
	for _, dest := range f.destinations {
		if err := dest.Delete(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
