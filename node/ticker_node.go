package node

import (
	"sync"
	"time"
)

// TickerNode is a specialized node that autonomously generates events
// on a regular schedule and broadcasts them to all subscribed nodes.
type TickerNode struct {
	*BaseNode
	ticker    *time.Ticker
	stopChan  chan struct{}
	payload   []byte
	wg        sync.WaitGroup
	mu        sync.Mutex
	isRunning bool
}

// NewTickerNode creates a new TickerNode that generates the given payload
// at the specified interval.
func NewTickerNode(id string, interval time.Duration, payload []byte) *TickerNode {
	t := &TickerNode{
		BaseNode: NewBaseNode(id),
		ticker:   time.NewTicker(interval),
		stopChan: make(chan struct{}),
		payload:  payload,
	}

	return t
}

// Create initializes the TickerNode and starts the background ticking goroutine.
func (t *TickerNode) Create() error {
	if err := t.BaseNode.Create(); err != nil {
		return err
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.isRunning {
		return nil
	}

	t.isRunning = true
	t.wg.Add(1)

	go func() {
		defer t.wg.Done()
		for {
			select {
			case <-t.ticker.C:
				// Ensure payload is copied if it's mutable, though it's typically read-only.
				payloadCopy := make([]byte, len(t.payload))
				copy(payloadCopy, t.payload)
				// Notify all subscribers asynchronously
				_ = t.Notify(payloadCopy)
			case <-t.stopChan:
				return
			}
		}
	}()

	return nil
}

// Delete stops the ticker, waits for the background goroutine to exit cleanly,
// and delegates to BaseNode.Delete().
func (t *TickerNode) Delete() error {
	t.mu.Lock()
	if t.isRunning {
		t.ticker.Stop()
		close(t.stopChan)
		t.isRunning = false
	}
	t.mu.Unlock()

	// Wait for the background goroutine to finish before deleting the base node
	t.wg.Wait()

	return t.BaseNode.Delete()
}
