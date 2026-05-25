package node

import (
	"errors"
	"sync"
	"time"
)

// BroadcastNode extends BaseNode to broadcast messages to all subscribed destinations concurrently.
type BroadcastNode struct {
	*BaseNode
	timeout time.Duration
}

// NewBroadcastNode creates a new BroadcastNode with a specific timeout for broadcasting.
func NewBroadcastNode(id string, timeout time.Duration) *BroadcastNode {
	b := &BroadcastNode{
		BaseNode: NewBaseNode(id),
		timeout:  timeout,
	}

	b.SetProcessFunc(b.broadcastData)
	return b
}

func (b *BroadcastNode) broadcastData(input []byte) ([]byte, error) {
	b.mutex.RLock()
	subs := make([]Node, 0, len(b.subscriptions))
	for _, node := range b.subscriptions {
		subs = append(subs, node)
	}
	b.mutex.RUnlock()

	if len(subs) == 0 {
		// Passthrough if no subscribers
		return input, nil
	}

	type broadcastResult struct {
		output []byte
		err    error
	}

	resultsChan := make(chan broadcastResult, len(subs))

	var wg sync.WaitGroup
	for _, sub := range subs {
		wg.Add(1)
		go func(n Node) {
			defer wg.Done()
			out, err := n.Process(input)
			resultsChan <- broadcastResult{output: out, err: err}
		}(sub)
	}

	// Wait for all to finish or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	var errs []error
	var outputs [][]byte

	select {
	case <-done:
		// All finished
	case <-time.After(b.timeout):
		errs = append(errs, errors.New("broadcast timeout"))
	}

	// Drain what we can from resultsChan
	// Do not close it, as pending goroutines may still write to it after timeout.
	drainCount := len(resultsChan)
	for i := 0; i < drainCount; i++ {
		res := <-resultsChan
		if res.err != nil {
			errs = append(errs, res.err)
		} else {
			outputs = append(outputs, res.output)
		}
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	// Combine outputs. In a simple broadcast, returning the original input or the first output might make sense.
	// We will combine all outputs simply by concatenating them, or if we want passthrough, we can just return input.
	// Concatenation is more generic for nodes that modify data.
	if len(outputs) > 0 {
		var totalLen int
		for _, o := range outputs {
			totalLen += len(o)
		}
		combined := make([]byte, 0, totalLen)
		for _, o := range outputs {
			combined = append(combined, o...)
		}
		return combined, nil
	}

	return input, nil
}
