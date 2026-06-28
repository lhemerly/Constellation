package node

import (
	"errors"
	"sync"
	"time"
)

// FanOutNode broadcasts a message to multiple destination nodes concurrently and
// aggregates the successful results. It supports an explicit timeout.
type FanOutNode struct {
	*BaseNode
	destinations []Node
	timeout      time.Duration
}

// NewFanOutNode creates a new FanOutNode with the specified destinations and timeout.
func NewFanOutNode(id string, destinations []Node, timeout time.Duration) *FanOutNode {
	f := &FanOutNode{
		BaseNode:     NewBaseNode(id),
		destinations: destinations,
		timeout:      timeout,
	}
	f.SetProcessFunc(f.fanOutProcess)
	return f
}

type workerResult struct {
	output []byte
	err    error
}

// fanOutProcess handles broadcasting the input and aggregating results.
func (f *FanOutNode) fanOutProcess(input []byte) ([]byte, error) {
	// Copy destinations under lock if dynamic addition is supported later
	// For now, it assumes destinations are fixed or safe to read
	dests := f.destinations

	if len(dests) == 0 {
		return nil, nil // Or an error depending on requirements
	}

	resultChan := make(chan workerResult, len(dests))
	var wg sync.WaitGroup

	for _, dest := range dests {
		wg.Add(1)
		go func(n Node) {
			defer wg.Done()

			// Clone input to prevent data races
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			res, err := n.Process(inputCopy)
			resultChan <- workerResult{output: res, err: err}
		}(dest)
	}

	// Goroutine to wait for all workers to finish and close the channel
	doneChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	var (
		results         [][]byte
		errs            []error
		timeoutOccurred bool
	)

	// Set up the timeout channel explicitly outside the loop
	var timeoutChan <-chan time.Time
	if f.timeout > 0 {
		timeoutChan = time.After(f.timeout)
	}

	// Wait for completion or timeout
	select {
	case <-doneChan:
		// All workers finished
	case <-timeoutChan:
		timeoutOccurred = true
	}

	// Drain the result channel using a non-blocking loop
	// We do this because if we hit a timeout, doneChan isn't closed yet,
	// and resultChan might still have results. And we don't want to use
	// range resultChan because it blocks until closed (which happens in doneChan).
	for {
		select {
		case res := <-resultChan:
			if res.err != nil {
				errs = append(errs, res.err)
			} else if len(res.output) > 0 {
				results = append(results, res.output)
			}
		default:
			goto CollectResults
		}
	}

CollectResults:
	if timeoutOccurred {
		errs = append(errs, errors.New("fan-out process timed out"))
	}

	if len(errs) > 0 {
		// Even if some failed, return what we have? Typically fan-out aggregates what succeeded.
		// If all failed or timeout occurred, return errors.
		// For simplicity, let's return errors if *any* failed, or just append them to the combined error.
		return nil, errors.Join(append([]error{errors.New("fan-out encountered errors")}, errs...)...)
	}

	var combined []byte
	for _, res := range results {
		combined = append(combined, res...)
	}

	return combined, nil
}
