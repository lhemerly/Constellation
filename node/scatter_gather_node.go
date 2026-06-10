package node

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ScatterGatherNode broadcasts a request concurrently to a set of destination nodes
// and returns the first successful response, or an aggregate error if all fail or a timeout is reached.
type ScatterGatherNode struct {
	*BaseNode
	targets []Node
	timeout time.Duration
}

// NewScatterGatherNode creates a new ScatterGatherNode with a set of target nodes and a timeout.
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

func (sg *ScatterGatherNode) scatterGatherProcess(input []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), sg.timeout)
	defer cancel()

	resultChan := make(chan []byte, 1)
	errChan := make(chan error, len(sg.targets))
	var wg sync.WaitGroup

	for _, target := range sg.targets {
		wg.Add(1)
		go func(n Node) {
			defer wg.Done()

			// Check context before starting long work
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			default:
			}

			// Clone input to prevent data races
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			res, err := n.Process(inputCopy)
			if err != nil {
				errChan <- err
				return
			}

			// Try to send result if no one else has succeeded yet
			select {
			case resultChan <- res:
				cancel() // we have a winner, cancel others
			case <-ctx.Done():
				// already canceled, someone else won or timeout
			}
		}(target)
	}

	// Wait for all to finish to close errChan safely
	doneChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(errChan)
		close(doneChan)
	}()

	select {
	case res := <-resultChan:
		return res, nil
	case <-doneChan:
		// All targets completed but none succeeded (we would have returned otherwise).
		// Collect all errors to return.
		var errs []error
		for err := range errChan {
			errs = append(errs, err)
		}
		return nil, errors.Join(append([]error{errors.New("all targets failed")}, errs...)...)
	case <-ctx.Done():
		// We timed out before getting a successful result or all targets failing
		var errs []error
		// Non-blocking drain of currently available errors
		drain:
		for {
			select {
			case err := <-errChan:
				errs = append(errs, err)
			default:
				break drain
			}
		}
		errs = append(errs, ErrProcessTimeout)
		return nil, errors.Join(append([]error{errors.New("all targets failed or timed out")}, errs...)...)
	}
}

// Create initializes the node and its targets.
func (sg *ScatterGatherNode) Create() error {
	if err := sg.BaseNode.Create(); err != nil {
		return err
	}
	for _, n := range sg.targets {
		if err := n.Create(); err != nil {
			return err
		}
	}
	return nil
}

// Delete cleans up the node and its targets.
func (sg *ScatterGatherNode) Delete() error {
	var errs []error
	if err := sg.BaseNode.Delete(); err != nil {
		errs = append(errs, err)
	}
	for _, n := range sg.targets {
		if err := n.Delete(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
