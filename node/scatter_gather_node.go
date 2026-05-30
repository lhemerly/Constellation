package node

import (
	"errors"
	"sync"
	"time"
)

// ErrScatterGatherTimeout is returned when gathering results takes longer than the optional timeout.
var ErrScatterGatherTimeout = errors.New("scatter-gather timed out")

// ErrEmptyScatterGather is returned when there are no nodes to scatter to.
var ErrEmptyScatterGather = errors.New("no nodes to scatter to")

// ScatterGatherNode extends BaseNode to scatter an input to multiple nodes concurrently and gather their results.
type ScatterGatherNode struct {
	*BaseNode
	nodesMutex sync.RWMutex
	nodes      []Node
	timeout    time.Duration
}

// NewScatterGatherNode creates a new ScatterGatherNode with a given ID and an optional timeout (0 means no timeout).
func NewScatterGatherNode(id string, timeout time.Duration) *ScatterGatherNode {
	s := &ScatterGatherNode{
		BaseNode: NewBaseNode(id),
		nodes:    make([]Node, 0),
		timeout:  timeout,
	}
	s.SetProcessFunc(s.scatterGatherProcess)
	return s
}

// AddNode adds a destination node for scattering.
func (s *ScatterGatherNode) AddNode(node Node) {
	s.nodesMutex.Lock()
	defer s.nodesMutex.Unlock()
	s.nodes = append(s.nodes, node)
}

// GetNodes returns the current list of destination nodes.
func (s *ScatterGatherNode) GetNodes() []Node {
	s.nodesMutex.RLock()
	defer s.nodesMutex.RUnlock()

	nodesCopy := make([]Node, len(s.nodes))
	copy(nodesCopy, s.nodes)
	return nodesCopy
}

type scatterResult struct {
	index  int
	output []byte
	err    error
}

// scatterGatherProcess executes the scatter-gather logic.
func (s *ScatterGatherNode) scatterGatherProcess(input []byte) ([]byte, error) {
	s.nodesMutex.RLock()
	if len(s.nodes) == 0 {
		s.nodesMutex.RUnlock()
		return nil, ErrEmptyScatterGather
	}

	nodesCopy := make([]Node, len(s.nodes))
	copy(nodesCopy, s.nodes)
	s.nodesMutex.RUnlock()

	numNodes := len(nodesCopy)
	resultsChan := make(chan scatterResult, numNodes)

	for i, n := range nodesCopy {
		go func(index int, node Node) {
			output, err := node.Process(input)
			resultsChan <- scatterResult{index: index, output: output, err: err}
		}(i, n)
	}

	results := make([][]byte, numNodes)
	var errs []error

	timeoutChan := make(<-chan time.Time)
	if s.timeout > 0 {
		timeoutChan = time.After(s.timeout)
	}

	for i := 0; i < numNodes; i++ {
		select {
		case res := <-resultsChan:
			if res.err != nil {
				errs = append(errs, res.err)
			} else {
				results[res.index] = res.output
			}
		case <-timeoutChan:
			return nil, ErrScatterGatherTimeout
		}
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	// Flatten gathered results
	var totalLen int
	for _, res := range results {
		totalLen += len(res)
	}

	finalOutput := make([]byte, 0, totalLen)
	for _, res := range results {
		finalOutput = append(finalOutput, res...)
	}

	return finalOutput, nil
}
