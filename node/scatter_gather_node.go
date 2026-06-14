package node

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrScatterGatherTimeout = errors.New("scatter-gather timeout reached before quorum")
	ErrScatterGatherFailed  = errors.New("scatter-gather total failure")
)

// ScatterGatherNode sends an input request to multiple target nodes concurrently
// and waits for responses until a specific timeout is reached or a required number
// of successful responses (quorum) are collected.
type ScatterGatherNode struct {
	*BaseNode
	targets []Node
	timeout time.Duration
	quorum  int
}

// NewScatterGatherNode creates a new ScatterGatherNode.
func NewScatterGatherNode(id string, targets []Node, timeout time.Duration, quorum int) *ScatterGatherNode {
	if quorum > len(targets) {
		quorum = len(targets)
	}

	sg := &ScatterGatherNode{
		BaseNode: NewBaseNode(id),
		targets:  targets,
		timeout:  timeout,
		quorum:   quorum,
	}

	sg.SetProcessFunc(sg.scatterGatherProcess)
	return sg
}

// Create initializes the scatter-gather node and its targets.
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

// Delete cleans up the scatter-gather node and its targets.
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

// scatterGatherProcess handles the dispatching and gathering.
func (sg *ScatterGatherNode) scatterGatherProcess(input []byte) ([]byte, error) {
	if len(sg.targets) == 0 {
		return nil, errors.New("no targets configured for scatter-gather")
	}

	ctx, cancel := context.WithTimeout(context.Background(), sg.timeout)
	defer cancel()

	resultChan := make(chan []byte, len(sg.targets))
	errChan := make(chan error, len(sg.targets))
	var wg sync.WaitGroup

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

	// Channel to signal when all workers are done (so we don't block on timeout
	// if everything finishes or fails early).
	doneChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	var finalOutput []byte
	var collectedErrs []error
	var successfulResponses int

	// Wait loop
	for {
		// Stop if we hit quorum
		if successfulResponses >= sg.quorum && sg.quorum > 0 {
			return finalOutput, nil
		}

		select {
		case res := <-resultChan:
			successfulResponses++
			// Concatenate successful responses
			finalOutput = append(finalOutput, res...)
		case <-ctx.Done():
			// Timeout reached.
			// Drain any remaining successful responses before declaring complete failure
			// just in case they arrived right as context was cancelled.
		drainLoop:
			for {
				select {
				case res := <-resultChan:
					successfulResponses++
					finalOutput = append(finalOutput, res...)
				default:
					break drainLoop
				}
			}

			if successfulResponses > 0 {
				return finalOutput, nil
			}

			// If no successes, gather errors without blocking
		errDrainLoop:
			for {
				select {
				case e := <-errChan:
					collectedErrs = append(collectedErrs, e)
				default:
					break errDrainLoop
				}
			}
			return nil, errors.Join(append([]error{ErrScatterGatherTimeout}, collectedErrs...)...)

		case <-doneChan:
			// All workers finished. Let's drain remaining results/errors.
		finishLoop:
			for {
				select {
				case res := <-resultChan:
					successfulResponses++
					finalOutput = append(finalOutput, res...)
				case e := <-errChan:
					collectedErrs = append(collectedErrs, e)
				default:
					break finishLoop
				}
			}

			if successfulResponses > 0 {
				return finalOutput, nil
			}
			return nil, errors.Join(append([]error{ErrScatterGatherFailed}, collectedErrs...)...)
		}
	}
}
