package node

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrFanOutTimeout = errors.New("fan-out process timed out")
)

// FanOutNode extends BaseNode to broadcast an input message to multiple destination nodes concurrently.
// It waits for all nodes to process the message or until a timeout is reached.
type FanOutNode struct {
	*BaseNode
	destinationsMutex sync.RWMutex
	destinations      []Node
	timeout           time.Duration
}

// NewFanOutNode creates a new FanOutNode with the given ID and timeout.
func NewFanOutNode(id string, timeout time.Duration) *FanOutNode {
	f := &FanOutNode{
		BaseNode:     NewBaseNode(id),
		destinations: make([]Node, 0),
		timeout:      timeout,
	}
	f.SetProcessFunc(f.fanOutData)
	return f
}

// AddDestination adds a node to the broadcast list.
func (f *FanOutNode) AddDestination(node Node) {
	f.destinationsMutex.Lock()
	defer f.destinationsMutex.Unlock()
	f.destinations = append(f.destinations, node)
}

type fanOutResult struct {
	output []byte
	err    error
}

// fanOutData broadcasts the input to all destinations concurrently and gathers the results.
// It implements a scatter-gather pattern.
func (f *FanOutNode) fanOutData(input []byte) ([]byte, error) {
	f.destinationsMutex.RLock()
	destinationsCopy := make([]Node, len(f.destinations))
	copy(destinationsCopy, f.destinations)
	f.destinationsMutex.RUnlock()

	if len(destinationsCopy) == 0 {
		return input, nil
	}

	resultChan := make(chan fanOutResult, len(destinationsCopy))
	ctx, cancel := context.WithTimeout(context.Background(), f.timeout)
	defer cancel()

	var wg sync.WaitGroup
	for _, dest := range destinationsCopy {
		wg.Add(1)
		go func(target Node) {
			defer wg.Done()

			// We simulate context cancellation on our own process logic
			// since Process interface doesn't take context
			done := make(chan fanOutResult, 1)
			go func() {
				out, err := target.Process(input)
				done <- fanOutResult{output: out, err: err}
			}()

			select {
			case res := <-done:
				resultChan <- res
			case <-ctx.Done():
				// Target process may continue, but we time out locally
				resultChan <- fanOutResult{output: nil, err: ErrFanOutTimeout}
			}
		}(dest)
	}

	// Wait for all workers in a separate goroutine
	doneChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	var errs []error
	var results [][]byte
	var timeoutOccurred bool

	select {
	case <-doneChan:
		// All finished
	case <-ctx.Done():
		timeoutOccurred = true
	}

	// Drain result channel. Note we don't close resultChan here because
	// slow goroutines might still be trying to write to it if we timed out.
	// Instead, we drain it with a non-blocking loop.
	for {
		select {
		case res := <-resultChan:
			if res.err != nil {
				if errors.Is(res.err, ErrFanOutTimeout) {
					timeoutOccurred = true
				} else {
					errs = append(errs, res.err)
				}
			} else {
				results = append(results, res.output)
			}
		default:
			goto EndDrain
		}
	}
EndDrain:

	if timeoutOccurred {
		return nil, ErrFanOutTimeout
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	// Just return the input or aggregate? Usually scatter gather returns something.
	// For simplicity, we just return the input since FanOut is typically a broadcast.
	return input, nil
}
