package node

import (
	"errors"
	"sync"
)

// ErrNoTargets is returned when ScatterGatherNode has no target nodes.
var ErrNoTargets = errors.New("no target nodes available")

// ScatterGatherNode extends BaseNode to distribute an input payload
// to multiple target nodes concurrently, waits for their responses,
// and concatenates the resulting byte slices in order of completion.
type ScatterGatherNode struct {
	*BaseNode
	targetsMutex sync.RWMutex
	targets      []Node
}

// NewScatterGatherNode creates a new ScatterGatherNode with the given ID.
func NewScatterGatherNode(id string) *ScatterGatherNode {
	sg := &ScatterGatherNode{
		BaseNode: NewBaseNode(id),
		targets:  make([]Node, 0),
	}

	// Set process func to scatter and gather
	sg.SetProcessFunc(sg.scatterGather)
	return sg
}

// AddTarget adds a target node to the broadcast list.
func (sg *ScatterGatherNode) AddTarget(node Node) {
	sg.targetsMutex.Lock()
	defer sg.targetsMutex.Unlock()
	sg.targets = append(sg.targets, node)
}

// RemoveTarget removes a target node from the broadcast list.
func (sg *ScatterGatherNode) RemoveTarget(nodeID string) {
	sg.targetsMutex.Lock()
	defer sg.targetsMutex.Unlock()
	for i, target := range sg.targets {
		if target.GetID() == nodeID {
			sg.targets = append(sg.targets[:i], sg.targets[i+1:]...)
			break
		}
	}
}

// scatterGather is the core processing logic for ScatterGatherNode.
func (sg *ScatterGatherNode) scatterGather(input []byte) ([]byte, error) {
	sg.targetsMutex.RLock()
	targetsCount := len(sg.targets)
	targetsCopy := make([]Node, targetsCount)
	copy(targetsCopy, sg.targets)
	sg.targetsMutex.RUnlock()

	if targetsCount == 0 {
		return nil, ErrNoTargets
	}

	type workerResult struct {
		output []byte
		err    error
	}

	// Use a channel to gather results concurrently
	resultsChan := make(chan workerResult, targetsCount)
	var wg sync.WaitGroup

	wg.Add(targetsCount)
	for _, target := range targetsCopy {
		go func(n Node) {
			defer wg.Done()
			out, err := n.Process(input)
			resultsChan <- workerResult{output: out, err: err}
		}(target)
	}

	// Wait for all to finish
	wg.Wait()
	close(resultsChan)

	var combinedOutput []byte
	var errs []error

	for res := range resultsChan {
		if res.err != nil {
			errs = append(errs, res.err)
		} else {
			combinedOutput = append(combinedOutput, res.output...)
		}
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return combinedOutput, nil
}
