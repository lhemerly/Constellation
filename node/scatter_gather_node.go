package node

import (
	"errors"
	"sync"
	"time"
)

// ErrScatterGatherTimeout is returned when the scatter-gather process times out without successful responses.
var ErrScatterGatherTimeout = errors.New("scatter-gather timed out")

// ErrScatterGatherFailed is returned when all destination nodes fail to process the request.
var ErrScatterGatherFailed = errors.New("scatter-gather failed on all destinations")

// ScatterGatherNode extends BaseNode to broadcast requests to multiple destinations
// and gather the first successful results within a timeout.
type ScatterGatherNode struct {
	*BaseNode
	destinationsMutex sync.RWMutex
	destinations      []Node
	timeout           time.Duration
}

// NewScatterGatherNode creates a new ScatterGatherNode.
func NewScatterGatherNode(id string, timeout time.Duration) *ScatterGatherNode {
	sg := &ScatterGatherNode{
		BaseNode:     NewBaseNode(id),
		destinations: make([]Node, 0),
		timeout:      timeout,
	}
	sg.SetProcessFunc(sg.scatterGatherProcess)
	return sg
}

// AddDestination adds a target node to the scatter-gather group.
func (sg *ScatterGatherNode) AddDestination(n Node) {
	sg.destinationsMutex.Lock()
	defer sg.destinationsMutex.Unlock()
	sg.destinations = append(sg.destinations, n)
}

func (sg *ScatterGatherNode) scatterGatherProcess(input []byte) ([]byte, error) {
	sg.destinationsMutex.RLock()
	dests := make([]Node, len(sg.destinations))
	copy(dests, sg.destinations)
	sg.destinationsMutex.RUnlock()

	if len(dests) == 0 {
		return nil, errors.New("no destinations configured")
	}

	type result struct {
		output []byte
		err    error
	}

	// Buffered channel to prevent goroutine leaks if they finish after timeout
	resultsChan := make(chan result, len(dests))

	for _, dest := range dests {
		go func(n Node) {
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			out, err := n.Process(inputCopy)
			resultsChan <- result{output: out, err: err}
		}(dest)
	}

	var errs []error
	var finalOutput [][]byte

	timeoutChan := time.After(sg.timeout)

GatherLoop:
	for i := 0; i < len(dests); i++ {
		select {
		case res := <-resultsChan:
			if res.err != nil {
				errs = append(errs, res.err)
			} else {
				finalOutput = append(finalOutput, res.output)
			}
		case <-timeoutChan:
			break GatherLoop
		}
	}

	if len(finalOutput) > 0 {
		// Just concatenating the outputs for this node type
		var combined []byte
		for _, out := range finalOutput {
			combined = append(combined, out...)
		}
		return combined, nil
	}

	if len(errs) == len(dests) {
		return nil, errors.Join(append([]error{ErrScatterGatherFailed}, errs...)...)
	}

	return nil, ErrScatterGatherTimeout
}
