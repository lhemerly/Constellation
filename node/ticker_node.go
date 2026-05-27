package node

import (
	"context"
	"time"
)

// TickerNode is a node that emits a specific payload to its subscribers
// on a regular schedule defined by a duration.
type TickerNode struct {
	*BaseNode
	interval time.Duration
	payload  []byte
	cancel   context.CancelFunc
	done     chan struct{}
}

// NewTickerNode creates a new TickerNode with the given ID, interval, and payload.
func NewTickerNode(id string, interval time.Duration, payload []byte) *TickerNode {
	return &TickerNode{
		BaseNode: NewBaseNode(id),
		interval: interval,
		payload:  payload,
	}
}

// Create starts the ticker goroutine to emit the payload at intervals.
func (t *TickerNode) Create() error {
	if err := t.BaseNode.Create(); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel
	t.done = make(chan struct{})

	go t.runTicker(ctx)

	return nil
}

// Delete stops the ticker goroutine and cleans up resources.
func (t *TickerNode) Delete() error {
	if t.cancel != nil {
		t.cancel()
		<-t.done
	}
	return t.BaseNode.Delete()
}

func (t *TickerNode) runTicker(ctx context.Context) {
	defer close(t.done)
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Emit the payload to all subscribers
			// Clone payload to prevent external modification
			payloadCopy := make([]byte, len(t.payload))
			copy(payloadCopy, t.payload)
			_ = t.Notify(payloadCopy)
		}
	}
}
