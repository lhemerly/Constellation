package node

import (
	"errors"
	"sync"
	"time"
)

// ErrScatterGatherTimeout is returned when the scatter-gather operation times out before all nodes respond.
var ErrScatterGatherTimeout = errors.New("scatter-gather timeout")

// ScatterGatherNode concurrently forwards input to multiple nodes and aggregates their responses
// using a custom gather function within a specified timeout.
type ScatterGatherNode struct {
	*BaseNode
	targets    []Node
	timeout    time.Duration
	gatherFunc func(results [][]byte, errs []error) ([]byte, error)
}

// NewScatterGatherNode creates a new ScatterGatherNode.
func NewScatterGatherNode(id string, targets []Node, timeout time.Duration, gatherFunc func([][]byte, []error) ([]byte, error)) *ScatterGatherNode {
	if len(targets) == 0 {
		panic("ScatterGatherNode requires at least one target node")
	}
	if gatherFunc == nil {
		panic("ScatterGatherNode requires a gather function")
	}

	sgn := &ScatterGatherNode{
		BaseNode:   NewBaseNode(id),
		targets:    targets,
		timeout:    timeout,
		gatherFunc: gatherFunc,
	}

	sgn.SetProcessFunc(sgn.scatterGatherProcess)

	return sgn
}

// scatterGatherProcess handles the scatter and gather lifecycle.
func (sgn *ScatterGatherNode) scatterGatherProcess(input []byte) ([]byte, error) {
	numTargets := len(sgn.targets)
	results := make([][]byte, numTargets)
	errs := make([]error, numTargets)

	var wg sync.WaitGroup
	var mu sync.Mutex

	wg.Add(numTargets)

	for i, target := range sgn.targets {
		go func(index int, n Node) {
			defer wg.Done()
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			res, err := n.Process(inputCopy)
			mu.Lock()
			results[index] = res
			errs[index] = err
			mu.Unlock()
		}(i, target)
	}

	doneChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	select {
	case <-doneChan:
		return sgn.gatherFunc(results, errs)
	case <-time.After(sgn.timeout):
		// Timeout reached, still run gatherFunc with whatever results are available
		mu.Lock()
		resultsCopy := make([][]byte, numTargets)
		copy(resultsCopy, results)
		errsCopy := make([]error, numTargets)
		copy(errsCopy, errs)
		mu.Unlock()
		return sgn.gatherFunc(resultsCopy, append(errsCopy, ErrScatterGatherTimeout))
	}
}

// Create initializes the scatter-gather node and its targets.
func (sgn *ScatterGatherNode) Create() error {
	if err := sgn.BaseNode.Create(); err != nil {
		return err
	}
	for _, target := range sgn.targets {
		if err := target.Create(); err != nil {
			return err
		}
	}
	return nil
}

// Delete cleans up the scatter-gather node and its targets.
func (sgn *ScatterGatherNode) Delete() error {
	var errsList []error
	if err := sgn.BaseNode.Delete(); err != nil {
		errsList = append(errsList, err)
	}
	for _, target := range sgn.targets {
		if err := target.Delete(); err != nil {
			errsList = append(errsList, err)
		}
	}

	if len(errsList) > 0 {
		return errors.Join(errsList...)
	}
	return nil
}
