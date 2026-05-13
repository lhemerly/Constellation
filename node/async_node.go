package node

import (
	"context"
	"errors"
	"sync"
)

// AsyncNode queues incoming processing requests and returns immediately
// without waiting for the output. A background worker processes the queue
// and forwards the items to a destination node.
type AsyncNode struct {
	*BaseNode
	destination Node
	queue       chan []byte
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// NewAsyncNode creates a new AsyncNode. The bufferSize determines
// how many requests can be queued before the Process method blocks
// or returns an error (depending on implementation, here it blocks until there is space).
func NewAsyncNode(id string, destination Node, bufferSize int) *AsyncNode {
	if destination == nil {
		panic("AsyncNode requires a destination node")
	}

	ctx, cancel := context.WithCancel(context.Background())

	an := &AsyncNode{
		BaseNode:    NewBaseNode(id),
		destination: destination,
		queue:       make(chan []byte, bufferSize),
		ctx:         ctx,
		cancel:      cancel,
	}

	an.SetProcessFunc(an.asyncProcess)
	return an
}

// asyncProcess queues the input for processing by the background worker.
func (an *AsyncNode) asyncProcess(input []byte) ([]byte, error) {
	// Clone input to prevent data races
	inputCopy := make([]byte, len(input))
	copy(inputCopy, input)

	select {
	case <-an.ctx.Done():
		return nil, errors.New("node is shutting down")
	default:
		select {
		case <-an.ctx.Done():
			return nil, errors.New("node is shutting down")
		case an.queue <- inputCopy:
			// Queued successfully
			return nil, nil
		}
	}
}

// worker processes items from the queue.
func (an *AsyncNode) worker() {
	defer an.wg.Done()
	for {
		select {
		case <-an.ctx.Done():
			// Process remaining items in the queue (best effort)
			for {
				select {
				case input := <-an.queue:
					_, _ = an.destination.Process(input)
				default:
					return
				}
			}
		case input := <-an.queue:
			_, _ = an.destination.Process(input)
		}
	}
}

// Create initializes the AsyncNode and starts its background worker.
func (an *AsyncNode) Create() error {
	if err := an.BaseNode.Create(); err != nil {
		return err
	}
	if err := an.destination.Create(); err != nil {
		return err
	}

	an.wg.Add(1)
	go an.worker()

	return nil
}

// Delete stops the background worker and cleans up the node.
func (an *AsyncNode) Delete() error {
	// Stop the worker
	an.cancel()
	an.wg.Wait()

	var errs []error
	if err := an.destination.Delete(); err != nil {
		errs = append(errs, err)
	}
	if err := an.BaseNode.Delete(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
