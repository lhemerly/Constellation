package node

import (
	"errors"
	"sync"
	"time"
)

// ScatterGatherNode represents a node that broadcasts an input to multiple targets
// concurrently and aggregates the successful results. It supports a timeout for the overall process.
type ScatterGatherNode struct {
	*BaseNode
	targets []Node
	timeout time.Duration
}

// NewScatterGatherNode creates a new ScatterGatherNode.
func NewScatterGatherNode(id string, targets []Node, timeout time.Duration) *ScatterGatherNode {
	if len(targets) == 0 {
		panic("ScatterGatherNode requires at least one target")
	}

	sgNode := &ScatterGatherNode{
		BaseNode: NewBaseNode(id),
		targets:  targets,
		timeout:  timeout,
	}

	sgNode.SetProcessFunc(sgNode.scatterGatherProcess)

	return sgNode
}

// scatterGatherProcess handles the scatter-gather lifecycle.
func (sg *ScatterGatherNode) scatterGatherProcess(input []byte) ([]byte, error) {
	var wg sync.WaitGroup
	resultChan := make(chan []byte, len(sg.targets))
	errChan := make(chan error, len(sg.targets))

	// Scatter
	for _, target := range sg.targets {
		wg.Add(1)
		go func(t Node) {
			defer wg.Done()

			// Clone input to prevent data races
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			res, err := t.Process(inputCopy)
			if err != nil {
				errChan <- err
			} else {
				resultChan <- res
			}
		}(target)
	}

	// Done channel to wait for all workers
	doneChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	var timeoutOccurred bool
	var timeoutChan <-chan time.Time
	if sg.timeout > 0 {
		timeoutChan = time.After(sg.timeout)
	}

	select {
	case <-doneChan:
		// All targets finished
	case <-timeoutChan:
		timeoutOccurred = true
	}

	// Gather
	// Close result and err channels if we waited for doneChan.
	// If timeout occurred, some workers might still be running and trying to write to the channels,
	// so we don't close them to avoid "send on closed channel" panic. We let GC handle them.

	// Collect results using a non-blocking loop
	var results [][]byte
	var errs []error

	// Helper to drain channels
	drainChannels := func() {
		for {
			select {
			case res := <-resultChan:
				results = append(results, res)
			case err := <-errChan:
				errs = append(errs, err)
			default:
				return // Channels drained
			}
		}
	}
	drainChannels()

	if timeoutOccurred {
		return nil, ErrProcessTimeout
	}

	// If all targets failed and there was no timeout, return an error
	if len(results) == 0 && len(errs) == len(sg.targets) {
		return nil, errors.Join(append([]error{errors.New("all targets failed in scatter-gather")}, errs...)...)
	}

	// Flatten results
	var totalLen int
	for _, res := range results {
		totalLen += len(res)
	}
	flattened := make([]byte, 0, totalLen)
	for _, res := range results {
		flattened = append(flattened, res...)
	}

	return flattened, nil
}

// Create initializes the scatter-gather node and its dependencies.
func (sg *ScatterGatherNode) Create() error {
	if err := sg.BaseNode.Create(); err != nil {
		return err
	}
	for _, t := range sg.targets {
		if err := t.Create(); err != nil {
			return err
		}
	}
	return nil
}

// Delete cleans up the scatter-gather node and its dependencies.
func (sg *ScatterGatherNode) Delete() error {
	var errs []error
	if err := sg.BaseNode.Delete(); err != nil {
		errs = append(errs, err)
	}
	for _, t := range sg.targets {
		if err := t.Delete(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
