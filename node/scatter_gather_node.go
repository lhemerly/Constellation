package node

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrScatterGatherTimeout is returned when the scatter-gather operation times out.
var ErrScatterGatherTimeout = errors.New("scatter-gather operation timed out")

// ScatterGatherNode broadcasts a request to multiple destination nodes and gathers their results.
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
		timeout:  timeout,
		nodes:    make([]Node, 0),
	}
	// Note: We don't use SetProcessFunc here, we override Process directly.
	return sg
}

// AddNode adds a destination node to the scatter-gather pool.
func (sg *ScatterGatherNode) AddNode(node Node) {
	sg.nodesMutex.Lock()
	defer sg.nodesMutex.Unlock()
	sg.nodes = append(sg.nodes, node)
}

// GetNodes returns the current list of destination nodes.
func (sg *ScatterGatherNode) GetNodes() []Node {
	sg.nodesMutex.RLock()
	defer sg.nodesMutex.RUnlock()

	nodesCopy := make([]Node, len(sg.nodes))
	copy(nodesCopy, sg.nodes)
	return nodesCopy
}

// Process scatters the input to all destination nodes and gathers their results within the timeout.
func (sg *ScatterGatherNode) Process(input []byte) ([]byte, error) {
	// First, run our own BaseNode process (for middlewares/metrics)
	_, err := sg.BaseNode.Process(input)
	if err != nil {
		return nil, err
	}

	targets := sg.GetNodes()
	if len(targets) == 0 {
		return nil, errors.New("no destination nodes available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), sg.timeout)
	defer cancel()

	type result struct {
		nodeID string
		output []byte
		err    error
	}

	resultChan := make(chan result, len(targets))
	var wg sync.WaitGroup

	for _, n := range targets {
		wg.Add(1)
		go func(targetNode Node) {
			defer wg.Done()

			// Clone input to prevent data races
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			output, err := targetNode.Process(inputCopy)
			resultChan <- result{
				nodeID: targetNode.GetID(),
				output: output,
				err:    err,
			}
		}(n)
	}

	// Wait for all workers to finish
	doneChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	var allErrors error
	var gatheredOutput []byte
	timeoutOccurred := false

	select {
	case <-doneChan:
		// All nodes finished before timeout
	case <-ctx.Done():
		// Timeout occurred
		timeoutOccurred = true
	}

	// We do NOT close resultChan here because in the event of a timeout,
	// background workers might still be executing and will panic if they
	// try to send to a closed channel. The channel is fully buffered to
	// len(targets), so the background workers will not block on send.

	// Drain all currently available results without blocking.
drainLoop:
	for {
		select {
		case res := <-resultChan:
			if res.err != nil {
				allErrors = errors.Join(allErrors, res.err)
			} else {
				// Append results simply for this example
				gatheredOutput = append(gatheredOutput, res.output...)
			}
		default:
			break drainLoop
		}
	}

	if timeoutOccurred {
		return gatheredOutput, errors.Join(allErrors, ErrScatterGatherTimeout)
	}

	if allErrors != nil && len(gatheredOutput) == 0 {
		return nil, allErrors
	}

	return gatheredOutput, allErrors
}
