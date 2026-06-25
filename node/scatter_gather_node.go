package node

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrScatterGatherTimeout is returned when the scatter-gather operation times out.
var ErrScatterGatherTimeout = errors.New("scatter-gather operation timed out")

// ScatterGatherNode broadcasts a request to multiple nodes concurrently
// and gathers their responses.
type ScatterGatherNode struct {
	*BaseNode
	nodesMutex sync.RWMutex
	targets    []Node
	timeout    time.Duration
}

// NewScatterGatherNode creates a new ScatterGatherNode.
func NewScatterGatherNode(id string, timeout time.Duration) *ScatterGatherNode {
	sg := &ScatterGatherNode{
		BaseNode: NewBaseNode(id),
		targets:  make([]Node, 0),
		timeout:  timeout,
	}

	sg.SetProcessFunc(sg.scatterGatherProcess)

	return sg
}

// AddTarget adds a destination node to the scatter-gather pool.
func (sg *ScatterGatherNode) AddTarget(node Node) {
	sg.nodesMutex.Lock()
	defer sg.nodesMutex.Unlock()
	sg.targets = append(sg.targets, node)
}

// GetTargets returns the current list of destination nodes.
func (sg *ScatterGatherNode) GetTargets() []Node {
	sg.nodesMutex.RLock()
	defer sg.nodesMutex.RUnlock()

	targetsCopy := make([]Node, len(sg.targets))
	copy(targetsCopy, sg.targets)
	return targetsCopy
}

// scatterGatherProcess broadcasts the input and gathers the results.
func (sg *ScatterGatherNode) scatterGatherProcess(input []byte) ([]byte, error) {
	targets := sg.GetTargets()
	if len(targets) == 0 {
		return nil, errors.New("no target nodes available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), sg.timeout)
	defer cancel()

	type result struct {
		output []byte
		err    error
	}

	resultChan := make(chan result, len(targets))
	var wg sync.WaitGroup

	for _, target := range targets {
		wg.Add(1)
		go func(n Node) {
			defer wg.Done()

			// Clone input to prevent data races
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			// Fast fail if context is already done
			select {
			case <-ctx.Done():
				resultChan <- result{nil, ctx.Err()}
				return
			default:
			}

			// We don't want the node process itself to block indefinitely,
			// but we handle the overall scatter-gather timeout below.
			output, err := n.Process(inputCopy)
			resultChan <- result{output, err}
		}(target)
	}

	// Goroutine to close result channel when all targets finish
	doneChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	var allResults [][]byte
	var allErrors []error
	timeoutOccurred := false

	select {
	case <-ctx.Done():
		timeoutOccurred = true
	case <-doneChan:
		// all workers finished before timeout
	}

	// Drain result channel
drainLoop:
	for {
		select {
		case res := <-resultChan:
			if res.err != nil {
				allErrors = append(allErrors, res.err)
			} else {
				allResults = append(allResults, res.output)
			}
		default:
			break drainLoop
		}
	}

	if timeoutOccurred {
		return nil, errors.Join(append([]error{ErrScatterGatherTimeout}, allErrors...)...)
	}

	if len(allErrors) > 0 {
		return nil, errors.Join(append([]error{errors.New("scatter-gather failed")}, allErrors...)...)
	}

	// Simple concatenation for results
	var finalResult []byte
	for _, res := range allResults {
		finalResult = append(finalResult, res...)
	}

	return finalResult, nil
}
