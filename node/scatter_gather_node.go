package node

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrScatterGatherTimeout = errors.New("scatter-gather operation timed out")
	ErrScatterGatherFailed  = errors.New("all target nodes failed or timed out")
	ErrNoTargets            = errors.New("no target nodes configured")
)

// ScatterGatherNode broadcasts a request to multiple target nodes,
// gathers their successful responses, and concatenates them.
// It supports a timeout to prevent waiting indefinitely for slow nodes.
type ScatterGatherNode struct {
	*BaseNode
	targetsMutex sync.RWMutex
	targets      []Node
	timeout      time.Duration
}

// NewScatterGatherNode creates a new ScatterGatherNode.
func NewScatterGatherNode(id string, timeout time.Duration) *ScatterGatherNode {
	sg := &ScatterGatherNode{
		BaseNode: NewBaseNode(id),
		targets:  make([]Node, 0),
		timeout:  timeout,
	}

	sg.SetProcessFunc(sg.scatterGather)
	return sg
}

// AddTarget adds a destination node to the scatter-gather group.
func (sg *ScatterGatherNode) AddTarget(node Node) {
	sg.targetsMutex.Lock()
	defer sg.targetsMutex.Unlock()
	sg.targets = append(sg.targets, node)
}

// GetTargets returns the nodes in the scatter-gather group.
func (sg *ScatterGatherNode) GetTargets() []Node {
	sg.targetsMutex.RLock()
	defer sg.targetsMutex.RUnlock()

	targetsCopy := make([]Node, len(sg.targets))
	copy(targetsCopy, sg.targets)
	return targetsCopy
}

// scatterGather broadcasts the input to all targets, waiting for them up to the timeout.
func (sg *ScatterGatherNode) scatterGather(input []byte) ([]byte, error) {
	targets := sg.GetTargets()
	if len(targets) == 0 {
		return nil, ErrNoTargets
	}

	type workerResult struct {
		output []byte
		err    error
	}

	resultChan := make(chan workerResult, len(targets))

	ctx, cancel := context.WithTimeout(context.Background(), sg.timeout)
	defer cancel()

	var wg sync.WaitGroup

	// Scatter phase
	for _, target := range targets {
		wg.Add(1)
		go func(n Node) {
			defer wg.Done()

			// Clone input to prevent data races during concurrent processing
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			res, err := n.Process(inputCopy)
			resultChan <- workerResult{output: res, err: err}
		}(target)
	}

	// Gather phase
	doneChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	var results [][]byte
	var errs []error

	timeoutOccurred := false

	select {
	case <-doneChan:
		// All workers finished before timeout
	case <-ctx.Done():
		timeoutOccurred = true
	}

	// Drain result channel for completed tasks without closing it,
	// so slow background workers don't panic on "send on closed channel".
	for {
		select {
		case res := <-resultChan:
			if res.err != nil {
				errs = append(errs, res.err)
			} else if res.output != nil {
				results = append(results, res.output)
			}
		default:
			goto drainComplete
		}
	}
drainComplete:

	if len(results) == 0 {
		if timeoutOccurred {
			return nil, errors.Join(ErrScatterGatherTimeout, errors.Join(errs...))
		}
		return nil, errors.Join(ErrScatterGatherFailed, errors.Join(errs...))
	}

	// Combine results
	var totalLen int
	for _, res := range results {
		totalLen += len(res)
	}

	combined := make([]byte, 0, totalLen)
	for _, res := range results {
		combined = append(combined, res...)
	}

	// Note: We return combined results even if there are partial errors or a timeout,
	// prioritizing successful responses gathered so far. We attach errors alongside valid results.
	if timeoutOccurred || len(errs) > 0 {
		var combinedErr error
		if timeoutOccurred {
			combinedErr = errors.Join(ErrScatterGatherTimeout, errors.Join(errs...))
		} else {
			combinedErr = errors.Join(errs...)
		}
		return combined, combinedErr
	}

	return combined, nil
}
