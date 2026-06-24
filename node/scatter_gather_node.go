package node

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrScatterGatherTimeout = errors.New("scatter gather operation timed out")
	ErrNoTargets            = errors.New("no target nodes configured for scatter gather")
)

// ScatterGatherNode concurrently broadcasts an input to multiple target nodes
// and gathers their results. It includes a timeout mechanism.
type ScatterGatherNode struct {
	*BaseNode
	targetsMutex sync.RWMutex
	targets      []Node
	timeout      time.Duration
}

// NewScatterGatherNode creates a new ScatterGatherNode with a specific timeout.
func NewScatterGatherNode(id string, timeout time.Duration) *ScatterGatherNode {
	sgn := &ScatterGatherNode{
		BaseNode: NewBaseNode(id),
		targets:  make([]Node, 0),
		timeout:  timeout,
	}
	sgn.SetProcessFunc(sgn.scatterGather)
	return sgn
}

// AddTarget adds a node to the scatter targets list.
func (sgn *ScatterGatherNode) AddTarget(node Node) {
	sgn.targetsMutex.Lock()
	defer sgn.targetsMutex.Unlock()
	sgn.targets = append(sgn.targets, node)
}

// RemoveTarget removes a node from the scatter targets list by its ID.
func (sgn *ScatterGatherNode) RemoveTarget(nodeID string) {
	sgn.targetsMutex.Lock()
	defer sgn.targetsMutex.Unlock()

	for i, target := range sgn.targets {
		if target.GetID() == nodeID {
			sgn.targets = append(sgn.targets[:i], sgn.targets[i+1:]...)
			break
		}
	}
}

// GetTargets returns a copy of the current target nodes list.
func (sgn *ScatterGatherNode) GetTargets() []Node {
	sgn.targetsMutex.RLock()
	defer sgn.targetsMutex.RUnlock()

	targetsCopy := make([]Node, len(sgn.targets))
	copy(targetsCopy, sgn.targets)
	return targetsCopy
}

type scatterResult struct {
	nodeID string
	output []byte
	err    error
}

// scatterGather processes the input by concurrently sending it to all target nodes.
func (sgn *ScatterGatherNode) scatterGather(input []byte) ([]byte, error) {
	targets := sgn.GetTargets()
	if len(targets) == 0 {
		return nil, ErrNoTargets
	}

	resultsChan := make(chan scatterResult, len(targets))
	var wg sync.WaitGroup

	ctx, cancel := context.WithTimeout(context.Background(), sgn.timeout)
	defer cancel()

	for _, target := range targets {
		wg.Add(1)
		go func(t Node) {
			defer wg.Done()

			// Deep copy input for each goroutine to prevent data races
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			out, err := t.Process(inputCopy)
			resultsChan <- scatterResult{
				nodeID: t.GetID(),
				output: out,
				err:    err,
			}
		}(target)
	}

	// Wait for all workers to finish
	doneChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	var combinedResults []byte
	var finalErr error
	var errs []error
	var timeoutOccurred bool

	select {
	case <-doneChan:
		// All targets finished within the timeout
	case <-ctx.Done():
		// Timeout occurred before all targets finished
		timeoutOccurred = true
	}

	// Drain results channel without blocking
	// The channel is never closed by workers to prevent panics, GC will collect it.
drainLoop:
	for {
		select {
		case res := <-resultsChan:
			if res.err != nil {
				errs = append(errs, res.err)
			} else {
				if res.output != nil {
					combinedResults = append(combinedResults, res.output...)
				}
			}
		default:
			break drainLoop
		}
	}

	if timeoutOccurred {
		if len(errs) > 0 {
			errs = append(errs, ErrScatterGatherTimeout)
			finalErr = errors.Join(errs...)
		} else {
			finalErr = ErrScatterGatherTimeout
		}
	} else if len(errs) > 0 {
		finalErr = errors.Join(errs...)
	}

	return combinedResults, finalErr
}
