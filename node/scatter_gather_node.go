package node

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ScatterGatherNode broadcasts an input to multiple nodes concurrently
// and gathers their results. It can optionally enforce a timeout.
type ScatterGatherNode struct {
	*BaseNode
	targets []Node
	timeout time.Duration
}

// NewScatterGatherNode creates a new ScatterGatherNode.
// If timeout is > 0, the node will wait up to that duration for all results.
func NewScatterGatherNode(id string, targets []Node, timeout time.Duration) *ScatterGatherNode {
	if len(targets) == 0 {
		panic("ScatterGatherNode requires at least one target node")
	}

	sgNode := &ScatterGatherNode{
		BaseNode: NewBaseNode(id),
		targets:  targets,
		timeout:  timeout,
	}

	sgNode.SetProcessFunc(sgNode.scatterGatherProcess)

	return sgNode
}

// scatterGatherProcess handles the broadcast and aggregation of results.
func (sg *ScatterGatherNode) scatterGatherProcess(input []byte) ([]byte, error) {
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		errs    []error
		results [][]byte
	)

	// Context for optional timeout
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if sg.timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), sg.timeout)
		defer cancel()
	}

	doneChan := make(chan struct{})

	// Scatter phase
	for _, target := range sg.targets {
		wg.Add(1)
		go func(n Node) {
			defer wg.Done()

			// Fast-fail if context is already done
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Clone input to prevent data races
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			resChan := make(chan struct {
				res []byte
				err error
			}, 1)

			go func() {
				r, e := n.Process(inputCopy)
				resChan <- struct {
					res []byte
					err error
				}{r, e}
			}()

			select {
			case r := <-resChan:
				mu.Lock()
				if r.err != nil {
					errs = append(errs, r.err)
				} else {
					results = append(results, r.res)
				}
				mu.Unlock()
			case <-ctx.Done():
				// Context cancelled or timed out
				mu.Lock()
				errs = append(errs, ctx.Err())
				mu.Unlock()
			}

		}(target)
	}

	go func() {
		wg.Wait()
		close(doneChan)
	}()

	// Wait for completion or fast-fail entirely if all workers complete
	select {
	case <-doneChan:
		// All completed
	case <-ctx.Done():
		// Wait for remaining workers to acknowledge cancellation
		<-doneChan
	}

	if len(errs) > 0 {
		return nil, errors.Join(append([]error{errors.New("scatter-gather failed")}, errs...)...)
	}

	// Gather phase: simple concatenation for this basic implementation.
	var gatheredInput []byte
	for _, res := range results {
		gatheredInput = append(gatheredInput, res...)
	}

	return gatheredInput, nil
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
