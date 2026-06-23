package node

import (
	"errors"
	"sync"
	"time"
)

// ScatterGatherNode broadcasts input to multiple target nodes and gathers their responses.
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

// AddTarget adds a destination node.
func (sgn *ScatterGatherNode) AddTarget(node Node) {
	sgn.targetsMutex.Lock()
	defer sgn.targetsMutex.Unlock()
	sgn.targets = append(sgn.targets, node)
}

// RemoveTarget removes a target by its ID.
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

// scatterGather processes the input by dispatching it to all targets concurrently.
func (sgn *ScatterGatherNode) scatterGather(input []byte) ([]byte, error) {
	sgn.targetsMutex.RLock()
	targetsCount := len(sgn.targets)
	targetsCopy := make([]Node, targetsCount)
	copy(targetsCopy, sgn.targets)
	sgn.targetsMutex.RUnlock()

	if targetsCount == 0 {
		return nil, errors.New("no targets configured")
	}

	var wg sync.WaitGroup
	// Create buffered channels so slow workers won't block forever if we timeout
	results := make(chan []byte, targetsCount)
	errs := make(chan error, targetsCount)

	for _, target := range targetsCopy {
		wg.Add(1)
		go func(t Node) {
			defer wg.Done()
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			res, err := t.Process(inputCopy)
			if err != nil {
				errs <- err
			} else {
				results <- res
			}
		}(target)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	timeoutChan := time.After(sgn.timeout)
	timeoutOccurred := false
	select {
	case <-done:
		// all finished normally
	case <-timeoutChan:
		// timed out
		timeoutOccurred = true
	}

	// Drain results and errors
	var finalResults [][]byte
	var allErrs []error

drainLoop:
	for {
		select {
		case r := <-results:
			finalResults = append(finalResults, r)
		case e := <-errs:
			allErrs = append(allErrs, e)
		default:
			break drainLoop
		}
	}

	var combinedResults []byte
	for _, res := range finalResults {
		combinedResults = append(combinedResults, res...)
	}

	// We consider it a failure only if ALL targets returned an error AND we have targets,
	// OR if it timed out and we gathered 0 successful results.
	if len(allErrs) == targetsCount && targetsCount > 0 {
		return nil, errors.Join(append([]error{errors.New("all targets failed")}, allErrs...)...)
	}

	if timeoutOccurred && len(finalResults) == 0 {
		return nil, errors.New("timed out and no targets succeeded")
	}

	return combinedResults, nil
}
