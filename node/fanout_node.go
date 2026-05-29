package node

import (
	"context"
	"errors"
	"sync"
	"time"
)

// FanOutNode broadcasts the same input to multiple destination nodes concurrently
// and aggregates their results. It can optionally enforce a timeout.
type FanOutNode struct {
	*BaseNode
	destinations []Node
	timeout      time.Duration
}

// NewFanOutNode creates a new FanOutNode with the specified destinations and timeout.
// If timeout is 0, no timeout is applied.
func NewFanOutNode(id string, destinations []Node, timeout time.Duration) *FanOutNode {
	f := &FanOutNode{
		BaseNode:     NewBaseNode(id),
		destinations: destinations,
		timeout:      timeout,
	}

	f.SetProcessFunc(f.fanOutProcess)
	return f
}

func (f *FanOutNode) fanOutProcess(input []byte) ([]byte, error) {
	if len(f.destinations) == 0 {
		return nil, errors.New("no destination nodes")
	}

	var ctx context.Context
	var cancel context.CancelFunc

	if f.timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), f.timeout)
		defer cancel()
	} else {
		ctx, cancel = context.WithCancel(context.Background())
		defer cancel()
	}

	results := make([][]byte, len(f.destinations))
	errs := make([]error, len(f.destinations))

	var wg sync.WaitGroup

	for i, dest := range f.destinations {
		wg.Add(1)
		go func(index int, n Node) {
			defer wg.Done()

			// Clone input to prevent data races
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			resCh := make(chan struct {
				output []byte
				err    error
			}, 1)

			go func() {
				res, err := n.Process(inputCopy)
				resCh <- struct {
					output []byte
					err    error
				}{res, err}
			}()

			select {
			case <-ctx.Done():
				errs[index] = ErrProcessTimeout
			case res := <-resCh:
				results[index] = res.output
				errs[index] = res.err
			}
		}(i, dest)
	}

	wg.Wait()

	var finalErrs []error
	var aggregatedOutput []byte
	for i := 0; i < len(f.destinations); i++ {
		if errs[i] != nil {
			finalErrs = append(finalErrs, errs[i])
		} else {
			aggregatedOutput = append(aggregatedOutput, results[i]...)
		}
	}

	if len(finalErrs) > 0 {
		return nil, errors.Join(append([]error{errors.New("fan-out processing failed")}, finalErrs...)...)
	}

	return aggregatedOutput, nil
}
