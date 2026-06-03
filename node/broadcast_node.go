package node

import (
	"errors"
	"sync"
)

// BroadcastNode extends BaseNode to broadcast inputs to multiple destination nodes concurrently.
// It differs from standard Pub/Sub (Notify) by being a structural processing node
// where the result of the broadcast is awaited during the `Process` phase.
// It returns the original input and aggregates any errors from destinations.
type BroadcastNode struct {
	*BaseNode
	destMutex    sync.RWMutex
	destinations []Node
}

// NewBroadcastNode creates a new BroadcastNode with the given ID.
func NewBroadcastNode(id string) *BroadcastNode {
	b := &BroadcastNode{
		BaseNode:     NewBaseNode(id),
		destinations: make([]Node, 0),
	}
	b.SetProcessFunc(b.broadcastData)
	return b
}

// AddDestination adds a node to the broadcast list.
func (b *BroadcastNode) AddDestination(node Node) {
	b.destMutex.Lock()
	defer b.destMutex.Unlock()
	b.destinations = append(b.destinations, node)
}

// broadcastData sends the input to all destinations concurrently and aggregates errors.
func (b *BroadcastNode) broadcastData(input []byte) ([]byte, error) {
	b.destMutex.RLock()
	dests := make([]Node, len(b.destinations))
	copy(dests, b.destinations)
	b.destMutex.RUnlock()

	if len(dests) == 0 {
		return input, nil
	}

	var (
		wg      sync.WaitGroup
		errs    []error
		errChan = make(chan error, len(dests))
	)

	for _, dest := range dests {
		wg.Add(1)
		go func(n Node) {
			defer wg.Done()
			_, err := n.Process(input)
			if err != nil {
				errChan <- err
			}
		}(dest)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return input, nil
}
