package node

import (
	"errors"
	"sync"
	"time"
)

// ErrScatterGatherTimeout is returned when the ScatterGatherNode times out
// and no successful results were gathered.
var ErrScatterGatherTimeout = errors.New("scatter-gather timeout with no successful results")

// ScatterGatherNode broadcasts input to multiple destination nodes concurrently,
// waits for their responses up to a configurable timeout, and aggregates successful results.
type ScatterGatherNode struct {
	*BaseNode
	destinationsMutex sync.RWMutex
	destinations      []Node
	timeout           time.Duration
}

// NewScatterGatherNode creates a new ScatterGatherNode with the given ID and timeout.
func NewScatterGatherNode(id string, timeout time.Duration) *ScatterGatherNode {
	sg := &ScatterGatherNode{
		BaseNode:     NewBaseNode(id),
		destinations: make([]Node, 0),
		timeout:      timeout,
	}

	sg.SetProcessFunc(sg.scatterGatherProcess)
	return sg
}

// AddDestination adds a destination node to broadcast to.
func (sg *ScatterGatherNode) AddDestination(node Node) {
	sg.destinationsMutex.Lock()
	defer sg.destinationsMutex.Unlock()
	sg.destinations = append(sg.destinations, node)
}

// scatterGatherProcess broadcasts the input to all destinations, gathers results
// within the timeout, and concatenates successful outputs.
func (sg *ScatterGatherNode) scatterGatherProcess(input []byte) ([]byte, error) {
	sg.destinationsMutex.RLock()
	dests := make([]Node, len(sg.destinations))
	copy(dests, sg.destinations)
	sg.destinationsMutex.RUnlock()

	if len(dests) == 0 {
		return nil, nil // No destinations to broadcast to
	}

	type workerResult struct {
		output []byte
		err    error
	}

	// We use a buffered channel to prevent goroutine leaks if they finish after the timeout.
	resultsChan := make(chan workerResult, len(dests))

	for _, dest := range dests {
		go func(n Node) {
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			res, err := n.Process(inputCopy)
			resultsChan <- workerResult{output: res, err: err}
		}(dest)
	}

	var finalOutput []byte
	var errs []error
	received := 0

	timeoutChan := time.After(sg.timeout)

GatherLoop:
	for received < len(dests) {
		select {
		case result := <-resultsChan:
			received++
			if result.err != nil {
				errs = append(errs, result.err)
			} else {
				finalOutput = append(finalOutput, result.output...)
			}
		case <-timeoutChan:
			errs = append(errs, ErrScatterGatherTimeout)
			break GatherLoop
		}
	}

	// Check if we have complete failure (no successful outputs, but we had destinations)
	// We check len(finalOutput) == 0 and len(dests) > 0.
	if len(finalOutput) == 0 && len(dests) > 0 {
		return nil, errors.Join(append([]error{errors.New("scatter-gather failed completely")}, errs...)...)
	}

	return finalOutput, nil
}
