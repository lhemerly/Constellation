package node

import (
	"sync"
)

// BroadcastNode extends BaseNode to provide concurrent broadcasting to its subscribers.
// It sends an input to all subscribed nodes concurrently, aggregates their outputs
// by concatenating them, and returns the combined result.
type BroadcastNode struct {
	*BaseNode
}

// NewBroadcastNode creates a new BroadcastNode with the given ID.
func NewBroadcastNode(id string) *BroadcastNode {
	b := &BroadcastNode{
		BaseNode: NewBaseNode(id),
	}
	// Replace default process function with our broadcast logic
	b.SetProcessFunc(b.broadcastData)
	return b
}

func (b *BroadcastNode) broadcastData(input []byte) ([]byte, error) {
	// Extract subscribers under read lock
	b.mutex.RLock()
	subs := make([]Node, 0, len(b.subscriptions))
	for _, sub := range b.subscriptions {
		subs = append(subs, sub)
	}
	b.mutex.RUnlock()

	if len(subs) == 0 {
		return input, nil
	}

	var wg sync.WaitGroup
	wg.Add(len(subs))

	type result struct {
		output []byte
		err    error
		index  int
	}

	resultsCh := make(chan result, len(subs))

	for i, sub := range subs {
		go func(idx int, n Node) {
			defer wg.Done()
			out, err := n.Process(input)
			resultsCh <- result{output: out, err: err, index: idx}
		}(i, sub)
	}

	wg.Wait()
	close(resultsCh)

	orderedResults := make([][]byte, len(subs))
	var combinedErr error

	for res := range resultsCh {
		if res.err != nil {
			combinedErr = res.err
			continue // We could break or join errors, here we just return the first error or keep evaluating.
		}
		orderedResults[res.index] = res.output
	}

	if combinedErr != nil {
		return nil, combinedErr
	}

	// Calculate total length for pre-allocation to prevent reallocations
	var totalLen int
	for _, out := range orderedResults {
		totalLen += len(out)
	}

	combinedOutput := make([]byte, 0, totalLen)
	for _, out := range orderedResults {
		combinedOutput = append(combinedOutput, out...)
	}

	return combinedOutput, nil
}
