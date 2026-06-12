package node

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ScatterGatherNode implements the scatter-gather pattern. It scatters a single
// input to multiple targets concurrently and gathers successful results until
// a specified quorum is reached, or a timeout occurs.
type ScatterGatherNode struct {
	*BaseNode
	targets []Node
	quorum  int
	timeout time.Duration
}

// NewScatterGatherNode creates a new ScatterGatherNode.
func NewScatterGatherNode(id string, targets []Node, quorum int, timeout time.Duration) *ScatterGatherNode {
	if len(targets) == 0 {
		panic("ScatterGatherNode requires at least one target")
	}
	if quorum <= 0 || quorum > len(targets) {
		panic("ScatterGatherNode requires quorum > 0 and <= len(targets)")
	}

	sgn := &ScatterGatherNode{
		BaseNode: NewBaseNode(id),
		targets:  targets,
		quorum:   quorum,
		timeout:  timeout,
	}

	sgn.SetProcessFunc(sgn.scatterGatherProcess)
	return sgn
}

// scatterGatherProcess handles the scatter-gather logic.
func (sgn *ScatterGatherNode) scatterGatherProcess(input []byte) ([]byte, error) {
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		errs    []error
		results [][]byte
	)

	// Context for cancellation
	ctx, cancel := context.WithTimeout(context.Background(), sgn.timeout)
	defer cancel()

	resultChan := make(chan []byte, len(sgn.targets))
	errChan := make(chan error, len(sgn.targets))

	// Scatter
	for _, target := range sgn.targets {
		wg.Add(1)
		go func(t Node) {
			defer wg.Done()

			// Check for cancellation before processing
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			default:
			}

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

	// Wait for all workers in a background goroutine to close channels
	doneChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneChan)
		close(resultChan)
		close(errChan)
	}()

	// Gather
	for {
		select {
		case res, ok := <-resultChan:
			if !ok {
				resultChan = nil
				break
			}
			mu.Lock()
			results = append(results, res)
			if len(results) >= sgn.quorum {
				cancel() // Stop remaining workers
				mu.Unlock()
				return sgn.combineResults(results), nil
			}
			mu.Unlock()
		case err, ok := <-errChan:
			if !ok {
				errChan = nil
				break
			}
			mu.Lock()
			errs = append(errs, err)
			mu.Unlock()
		case <-doneChan:
			mu.Lock()
			defer mu.Unlock()
			if len(results) >= sgn.quorum {
				return sgn.combineResults(results), nil
			}
			return nil, errors.Join(append([]error{errors.New("scatter-gather failed to reach quorum")}, errs...)...)
		case <-ctx.Done():
			mu.Lock()
			defer mu.Unlock()
			if len(results) >= sgn.quorum {
				return sgn.combineResults(results), nil
			}
			return nil, errors.Join(append([]error{errors.New("scatter-gather timeout")}, errs...)...)
		}
	}
}

func (sgn *ScatterGatherNode) combineResults(results [][]byte) []byte {
	// Simple concatenation for now, similar to MapReduceNode
	var combined []byte
	for _, res := range results {
		combined = append(combined, res...)
	}
	return combined
}

// Create initializes the scatter-gather node and its dependencies.
func (sgn *ScatterGatherNode) Create() error {
	if err := sgn.BaseNode.Create(); err != nil {
		return err
	}
	for _, t := range sgn.targets {
		if err := t.Create(); err != nil {
			return err
		}
	}
	return nil
}

// Delete cleans up the scatter-gather node and its dependencies.
func (sgn *ScatterGatherNode) Delete() error {
	var errs []error
	if err := sgn.BaseNode.Delete(); err != nil {
		errs = append(errs, err)
	}
	for _, t := range sgn.targets {
		if err := t.Delete(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
