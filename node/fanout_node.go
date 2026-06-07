package node

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"time"
)

// FanOutNode broadcasts the input to multiple destination nodes concurrently.
// It collects the successful responses, joins them together, and returns the combined slice.
// Nodes that take longer than the specified timeout are omitted from the final result.
type FanOutNode struct {
	*BaseNode
	destinations []Node
	timeout      time.Duration
}

// NewFanOutNode creates a new FanOutNode with the specified destinations and timeout.
func NewFanOutNode(id string, destinations []Node, timeout time.Duration) *FanOutNode {
	n := &FanOutNode{
		BaseNode:     NewBaseNode(id),
		destinations: destinations,
		timeout:      timeout,
	}
	n.SetProcessFunc(n.fanOutProcess)
	return n
}

func (n *FanOutNode) fanOutProcess(input []byte) ([]byte, error) {
	n.mutex.RLock()
	dests := make([]Node, len(n.destinations))
	copy(dests, n.destinations)
	n.mutex.RUnlock()

	if len(dests) == 0 {
		return nil, errors.New("no destination nodes configured")
	}

	type result struct {
		output []byte
		err    error
	}

	resChan := make(chan result, len(dests))
	ctx, cancel := context.WithTimeout(context.Background(), n.timeout)
	defer cancel()

	var wg sync.WaitGroup
	for _, dest := range dests {
		wg.Add(1)
		go func(target Node) {
			defer wg.Done()

			// We cannot easily cancel target.Process if it doesn't take a context,
			// but we can ensure we don't block forever trying to send to resChan.
			out, err := target.Process(input)

			select {
			case resChan <- result{output: out, err: err}:
			case <-ctx.Done():
				// Timeout reached, discard the result
			}
		}(dest)
	}

	// Close the channel when all workers are done
	go func() {
		wg.Wait()
		close(resChan)
	}()

	var finalOutput [][]byte
	var joinErrs []error

	// Collect results until either all workers finish or timeout is reached
loop:
	for {
		select {
		case res, ok := <-resChan:
			if !ok {
				break loop
			}
			if res.err != nil {
				joinErrs = append(joinErrs, res.err)
			} else {
				finalOutput = append(finalOutput, res.output)
			}
		case <-ctx.Done():
			// Timeout reached. We don't close the channel here to prevent panic
			// from slow workers trying to write to a closed channel. The wg goroutine
			// will eventually close it once all workers finish their Process calls.
			joinErrs = append(joinErrs, ErrProcessTimeout)
			break loop
		}
	}

	if len(finalOutput) == 0 && len(dests) > 0 {
		return nil, errors.Join(joinErrs...)
	}

	return bytes.Join(finalOutput, []byte{}), nil
}
