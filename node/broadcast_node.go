package node

import (
	"errors"
	"sync"
	"time"
)

// BroadcastNode extends BaseNode to broadcast incoming requests to multiple
// destination nodes concurrently.
type BroadcastNode struct {
	*BaseNode
	nodesMutex sync.RWMutex
	nodes      []Node
	timeout    time.Duration
}

// NewBroadcastNode creates a new BroadcastNode with the given ID.
// The optional timeout restricts how long to wait for all nodes to complete.
func NewBroadcastNode(id string, timeout time.Duration) *BroadcastNode {
	bn := &BroadcastNode{
		BaseNode: NewBaseNode(id),
		nodes:    make([]Node, 0),
		timeout:  timeout,
	}

	// Set the processing function to broadcast requests
	// Note: We override Process directly instead of SetProcessFunc
	// to ensure it handles concurrency and we don't accidentally
	// let users overwrite the core broadcast logic with SetProcessFunc.
	bn.BaseNode.baseProcess = bn.broadcast
	bn.BaseNode.rebuildProcessFunc()

	return bn
}

// AddNode adds a destination node to the broadcast pool.
func (bn *BroadcastNode) AddNode(node Node) {
	bn.nodesMutex.Lock()
	defer bn.nodesMutex.Unlock()
	bn.nodes = append(bn.nodes, node)
}

// GetNodes returns the current list of destination nodes.
func (bn *BroadcastNode) GetNodes() []Node {
	bn.nodesMutex.RLock()
	defer bn.nodesMutex.RUnlock()

	// Return a copy to avoid external modifications
	nodesCopy := make([]Node, len(bn.nodes))
	copy(nodesCopy, bn.nodes)
	return nodesCopy
}

type workerResult struct {
	output []byte
	err    error
}

// broadcast sends the input to all nodes concurrently and waits for all of them to finish.
func (bn *BroadcastNode) broadcast(input []byte) ([]byte, error) {
	bn.nodesMutex.RLock()
	nodesCount := len(bn.nodes)
	if nodesCount == 0 {
		bn.nodesMutex.RUnlock()
		return nil, nil // Or an error, depending on desired semantics. Returning nil for now.
	}

	// Copy nodes to avoid holding lock during processing
	nodesCopy := make([]Node, nodesCount)
	copy(nodesCopy, bn.nodes)
	bn.nodesMutex.RUnlock()

	var wg sync.WaitGroup
	resultChan := make(chan workerResult, nodesCount)

	for _, n := range nodesCopy {
		wg.Add(1)
		go func(targetNode Node) {
			defer wg.Done()
			// Clone input to avoid concurrent modifications if downstream nodes mutate it
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			out, err := targetNode.Process(inputCopy)
			resultChan <- workerResult{output: out, err: err}
		}(n)
	}

	// Wait for all goroutines in a separate goroutine so we can use select with a timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	if bn.timeout > 0 {
		select {
		case <-done:
		case <-time.After(bn.timeout):
			return nil, errors.New("broadcast timed out")
		}
	} else {
		<-done
	}

	close(resultChan)

	var combinedErrors []error
	var totalLen int

	// First pass: collect errors and calculate total output length
	// We read everything from the buffered channel. We know it has nodesCount items at most.
	// Since done is closed, all goroutines finished (or timed out, in which case we don't reach here).
	var results []workerResult
	for res := range resultChan {
		results = append(results, res)
		if res.err != nil {
			combinedErrors = append(combinedErrors, res.err)
		}
		if res.output != nil {
			totalLen += len(res.output)
		}
	}

	if len(combinedErrors) > 0 {
		return nil, errors.Join(combinedErrors...)
	}

	// Second pass: Combine outputs efficiently
	combinedOutput := make([]byte, 0, totalLen)
	for _, res := range results {
		if res.output != nil {
			combinedOutput = append(combinedOutput, res.output...)
		}
	}

	return combinedOutput, nil
}
