package node

import (
	"errors"
	"sync"
	"time"
)

// ErrScatterGatherTimeout is returned when the node times out before receiving results.
var ErrScatterGatherTimeout = errors.New("scatter-gather timeout")
var ErrScatterGatherFailed = errors.New("scatter-gather completely failed")

// ScatterGatherNode dispatches a request to multiple destination nodes concurrently
// and gathers their successful results, up to a specified timeout.
type ScatterGatherNode struct {
	*BaseNode
	nodesMutex sync.RWMutex
	nodes      []Node
	timeout    time.Duration
}

// NewScatterGatherNode creates a new ScatterGatherNode.
func NewScatterGatherNode(id string, timeout time.Duration) *ScatterGatherNode {
	sg := &ScatterGatherNode{
		BaseNode: NewBaseNode(id),
		nodes:    make([]Node, 0),
		timeout:  timeout,
	}
	sg.SetProcessFunc(sg.scatterGatherProcess)
	return sg
}

// AddNode adds a destination node to the scatter-gather pool.
func (sg *ScatterGatherNode) AddNode(node Node) {
	sg.nodesMutex.Lock()
	defer sg.nodesMutex.Unlock()
	sg.nodes = append(sg.nodes, node)
}

// scatterGatherProcess handles the dispatching and gathering logic.
func (sg *ScatterGatherNode) scatterGatherProcess(input []byte) ([]byte, error) {
	sg.nodesMutex.RLock()
	dests := make([]Node, len(sg.nodes))
	copy(dests, sg.nodes)
	sg.nodesMutex.RUnlock()

	if len(dests) == 0 {
		return nil, errors.New("no nodes to scatter to")
	}

	var wg sync.WaitGroup
	resultChan := make(chan []byte, len(dests))

	for _, dest := range dests {
		wg.Add(1)
		go func(n Node) {
			defer wg.Done()
			// Clone input
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			output, err := n.Process(inputCopy)
			if err == nil {
				resultChan <- output
			}
		}(dest)
	}

	doneChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	var finalOutput []byte
	var timedOut bool

	// Create the timeout channel outside the loop so it's an absolute timeout
	timeoutChan := time.After(sg.timeout)

GatherLoop:
	for {
		select {
		case res := <-resultChan:
			// Just concatenate the first one we receive, or all of them?
			// The instructions say "gathers their results".
			// Let's concatenate them.
			finalOutput = append(finalOutput, res...)
		case <-timeoutChan:
			timedOut = true
			break GatherLoop
		case <-doneChan:
			break GatherLoop
		}
	}

	// Wait, if it timed out or doneChan closed, there might still be results in resultChan.
	// We can optionally drain resultChan using a non-blocking select, but since we have finalOutput
	// and it's a simple concatenation, we'll just check if we have any successful output.
	// However, we should drain the channel after the loop to ensure we got everything that was
	// sent just before the timeout/doneChan.
DrainLoop:
	for {
		select {
		case res := <-resultChan:
			finalOutput = append(finalOutput, res...)
		default:
			break DrainLoop
		}
	}

	if len(finalOutput) == 0 && len(dests) > 0 {
		if timedOut {
			return nil, ErrScatterGatherTimeout
		}
		return nil, ErrScatterGatherFailed
	}

	return finalOutput, nil
}
