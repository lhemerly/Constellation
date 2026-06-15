package node

import (
	"context"
	"sync"
	"time"
)

// ScatterGatherNode implements the scatter-gather pattern. It fans out an
// incoming request to multiple destination nodes concurrently, waits for their
// responses (up to a configurable timeout), and then gathers all successful outputs
// into a single aggregated response.
type ScatterGatherNode struct {
	*BaseNode
	destinationsMutex sync.RWMutex
	destinations      []Node
	timeout           time.Duration
}

// NewScatterGatherNode creates a new ScatterGatherNode.
func NewScatterGatherNode(id string, timeout time.Duration) *ScatterGatherNode {
	n := &ScatterGatherNode{
		BaseNode:     NewBaseNode(id),
		destinations: make([]Node, 0),
		timeout:      timeout,
	}

	n.SetProcessFunc(n.scatterGather)
	return n
}

// AddDestination adds a node to the list of destinations for the fan-out.
func (n *ScatterGatherNode) AddDestination(dest Node) {
	n.destinationsMutex.Lock()
	defer n.destinationsMutex.Unlock()
	n.destinations = append(n.destinations, dest)
}

// scatterGather fans out the input to all destination nodes and collects the results.
func (n *ScatterGatherNode) scatterGather(input []byte) ([]byte, error) {
	// Copy destinations to avoid holding lock during Process calls
	n.destinationsMutex.RLock()
	dests := make([]Node, len(n.destinations))
	copy(dests, n.destinations)
	n.destinationsMutex.RUnlock()

	if len(dests) == 0 {
		return nil, nil // No destinations to gather from
	}

	ctx, cancel := context.WithTimeout(context.Background(), n.timeout)
	defer cancel()

	type result struct {
		output []byte
		err    error
	}

	resultsChan := make(chan result, len(dests))
	var wg sync.WaitGroup

	for _, dest := range dests {
		wg.Add(1)
		go func(d Node) {
			defer wg.Done()

			// Clone input to prevent race conditions if destinations modify it
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			output, err := d.Process(inputCopy)
			resultsChan <- result{output: output, err: err}
		}(dest)
	}

	// Wait for all workers to finish
	doneChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	var finalOutput []byte
	var wgWaitDone bool
	var timeoutOccurred bool

	// Collect results until either all finish or timeout occurs
	select {
	case <-doneChan:
		wgWaitDone = true
	case <-ctx.Done():
		timeoutOccurred = true
	}

	var errorsList []error

	// Drain the results channel non-blockingly since workers might still write to it
	// if we hit the timeout branch. If we hit the doneChan branch, the channel
	// has exactly len(dests) elements and will be drained safely.
	if wgWaitDone {
		close(resultsChan)
		for res := range resultsChan {
			if res.err == nil {
				finalOutput = append(finalOutput, res.output...)
			} else {
				errorsList = append(errorsList, res.err)
			}
		}
	} else {
		// Timeout scenario: drain what we have using a non-blocking select
		// Avoid closing the channel because pending workers might still try to send.
		for {
			select {
			case res := <-resultsChan:
				if res.err == nil {
					finalOutput = append(finalOutput, res.output...)
				} else {
					errorsList = append(errorsList, res.err)
				}
			default:
				goto DRAIN_DONE
			}
		}
	DRAIN_DONE:
	}

	// If no success and we hit a timeout, return timeout
	if timeoutOccurred && len(finalOutput) == 0 && len(errorsList) == 0 {
		return nil, ErrProcessTimeout
	}

	// If everything failed, returning first error for simplicity
	if len(finalOutput) == 0 && len(errorsList) == len(dests) {
		return nil, errorsList[0]
	}

	return finalOutput, nil
}
