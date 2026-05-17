package node

import (
	"context"
	"time"
)

// ScheduledNode executes a periodic task using a payload on a fixed interval.
// It leverages BaseNode's Notify to broadcast results if needed,
// or just processes data internally.
type ScheduledNode struct {
	*BaseNode
	interval time.Duration
	payload  []byte
	cancel   context.CancelFunc
	done     chan struct{}
}

// NewScheduledNode creates a new ScheduledNode.
func NewScheduledNode(id string, interval time.Duration, payload []byte) *ScheduledNode {
	return &ScheduledNode{
		BaseNode: NewBaseNode(id),
		interval: interval,
		payload:  payload,
		done:     make(chan struct{}),
	}
}

// Create overrides BaseNode's Create to start the background ticker.
func (n *ScheduledNode) Create() error {
	if err := n.BaseNode.Create(); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	n.cancel = cancel

	go n.runLoop(ctx)

	return nil
}

// Delete overrides BaseNode's Delete to stop the ticker and wait for the loop to exit.
func (n *ScheduledNode) Delete() error {
	if n.cancel != nil {
		n.cancel()
		<-n.done // wait for goroutine to exit
	}
	return n.BaseNode.Delete()
}

func (n *ScheduledNode) runLoop(ctx context.Context) {
	defer close(n.done)
	ticker := time.NewTicker(n.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Process the payload when ticker fires
			out, err := n.Process(n.payload)
			if err == nil && len(out) > 0 {
				// We can optionally Notify subscribers with the result
				_ = n.Notify(out)
			}
		}
	}
}
