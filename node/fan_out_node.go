package node

import (
	"context"
	"errors"
	"sync"
	"time"
)

// FanOutNode implements concurrent scatter-gather processing.
// It sends the input to all target nodes simultaneously and collects the results.
type FanOutNode struct {
	*BaseNode
	targets []Node
	timeout time.Duration
}

// NewFanOutNode creates a new FanOutNode with the specified targets and timeout.
func NewFanOutNode(id string, targets []Node, timeout time.Duration) *FanOutNode {
	if len(targets) == 0 {
		panic("FanOutNode requires at least one target node")
	}

	fanOutNode := &FanOutNode{
		BaseNode: NewBaseNode(id),
		targets:  targets,
		timeout:  timeout,
	}

	fanOutNode.SetProcessFunc(fanOutNode.fanOutProcess)
	return fanOutNode
}

func (fo *FanOutNode) fanOutProcess(input []byte) ([]byte, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error
	var results [][]byte

	ctx, cancel := context.WithTimeout(context.Background(), fo.timeout)
	defer cancel()

	for _, target := range fo.targets {
		wg.Add(1)
		go func(n Node) {
			defer wg.Done()

			// Clone input to prevent data races
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			resChan := make(chan struct {
				res []byte
				err error
			}, 1)

			go func() {
				res, err := n.Process(inputCopy)
				resChan <- struct {
					res []byte
					err error
				}{res, err}
			}()

			select {
			case <-ctx.Done():
				mu.Lock()
				errs = append(errs, errors.New("target node timed out"))
				mu.Unlock()
			case r := <-resChan:
				mu.Lock()
				if r.err != nil {
					errs = append(errs, r.err)
				} else {
					results = append(results, r.res)
				}
				mu.Unlock()
			}
		}(target)
	}

	// We must have a dedicated doneChan to wait for the WG.
	// We cannot just use time.After or blocking Wait() here directly because
	// we want to return as soon as all workers finish, without waiting for the timeout
	// if they finish early.
	doneChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	select {
	case <-doneChan:
		// All goroutines finished
	case <-ctx.Done():
		// Main timeout reached, some might still be running but we don't care
	}

	// Wait for any remaining cleanup or context cancellation completion in the worker goroutines
	// to ensure mu is fully updated. doneChan guarantees they are all past the select block.
	<-doneChan

	if len(errs) == len(fo.targets) {
		return nil, errors.Join(append([]error{errors.New("all target nodes failed or timed out")}, errs...)...)
	}

	// Aggregate successful results
	var aggregatedResult []byte
	for _, res := range results {
		aggregatedResult = append(aggregatedResult, res...)
	}

	return aggregatedResult, nil
}

// Create initializes the node and its targets.
func (fo *FanOutNode) Create() error {
	if err := fo.BaseNode.Create(); err != nil {
		return err
	}
	for _, target := range fo.targets {
		if err := target.Create(); err != nil {
			return err
		}
	}
	return nil
}

// Delete cleans up the node and its targets.
func (fo *FanOutNode) Delete() error {
	var errs []error
	if err := fo.BaseNode.Delete(); err != nil {
		errs = append(errs, err)
	}
	for _, target := range fo.targets {
		if err := target.Delete(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
