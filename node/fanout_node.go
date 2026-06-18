package node

import (
	"errors"
	"sync"
	"time"
)

var (
	// ErrNoFanOutNodes is returned when the FanOutNode has no targets to send to.
	ErrNoFanOutNodes = errors.New("no destination nodes available for fan-out")
	// ErrFanOutTimeout is returned when the scatter-gather operation times out.
	ErrFanOutTimeout = errors.New("fan-out processing timed out")
)

// FanOutNode extends BaseNode to broadcast incoming data to multiple
// destination nodes concurrently and gathers their results.
type FanOutNode struct {
	*BaseNode
	nodesMutex sync.RWMutex
	nodes      []Node
	timeout    time.Duration
}

// NewFanOutNode creates a new FanOutNode with the given ID and timeout for the gather phase.
func NewFanOutNode(id string, timeout time.Duration) *FanOutNode {
	f := &FanOutNode{
		BaseNode: NewBaseNode(id),
		nodes:    make([]Node, 0),
		timeout:  timeout,
	}

	f.SetProcessFunc(f.fanOut)
	return f
}

// AddNode adds a destination node to the fan-out targets.
func (f *FanOutNode) AddNode(node Node) {
	f.nodesMutex.Lock()
	defer f.nodesMutex.Unlock()
	f.nodes = append(f.nodes, node)
}

// RemoveNode removes a destination node from the fan-out targets by its ID.
func (f *FanOutNode) RemoveNode(nodeID string) {
	f.nodesMutex.Lock()
	defer f.nodesMutex.Unlock()
	for i, node := range f.nodes {
		if node.GetID() == nodeID {
			f.nodes = append(f.nodes[:i], f.nodes[i+1:]...)
			break
		}
	}
}

// GetNodes returns the current list of destination nodes.
func (f *FanOutNode) GetNodes() []Node {
	f.nodesMutex.RLock()
	defer f.nodesMutex.RUnlock()
	nodesCopy := make([]Node, len(f.nodes))
	copy(nodesCopy, f.nodes)
	return nodesCopy
}

type workerResult struct {
	output []byte
	err    error
}

// fanOut broadcasts the input data to all target nodes concurrently,
// waits for all to finish (or timeout), and aggregates the successful results.
func (f *FanOutNode) fanOut(input []byte) ([]byte, error) {
	f.nodesMutex.RLock()
	targetsCount := len(f.nodes)
	if targetsCount == 0 {
		f.nodesMutex.RUnlock()
		return nil, ErrNoFanOutNodes
	}

	targets := make([]Node, targetsCount)
	copy(targets, f.nodes)
	f.nodesMutex.RUnlock()

	var wg sync.WaitGroup
	resultChan := make(chan workerResult, targetsCount)

	for _, target := range targets {
		wg.Add(1)
		go func(n Node) {
			defer wg.Done()

			// Clone input to prevent data races if nodes modify it
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			out, err := n.Process(inputCopy)
			resultChan <- workerResult{output: out, err: err}
		}(target)
	}

	doneChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	var errs []error
	var accumulatedResults [][]byte
	var totalLen int

	timeoutChan := time.After(f.timeout)
	timeoutOccurred := false

	select {
	case <-doneChan:
		// All finished successfully before timeout
	case <-timeoutChan:
		timeoutOccurred = true
	}

	// Drain the results channel safely non-blockingly
drainLoop:
	for {
		select {
		case res := <-resultChan:
			if res.err != nil {
				errs = append(errs, res.err)
			} else if len(res.output) > 0 {
				accumulatedResults = append(accumulatedResults, res.output)
				totalLen += len(res.output)
			}
		default:
			break drainLoop
		}
	}

	if timeoutOccurred {
		// Even if some succeeded, a timeout occurred for the whole operation
		errs = append([]error{ErrFanOutTimeout}, errs...)
		return nil, errors.Join(errs...)
	}

	if len(errs) > 0 {
		return nil, errors.Join(append([]error{errors.New("fan-out processing failed for one or more nodes")}, errs...)...)
	}

	// Concatenate successful outputs
	finalOutput := make([]byte, 0, totalLen)
	for _, res := range accumulatedResults {
		finalOutput = append(finalOutput, res...)
	}

	return finalOutput, nil
}
