package node

import (
	"context"
	"time"
)

// TickerNode is a specialized node that generates an event at regular intervals
// and broadcasts it to all its subscribers via Notify(). It does not process
// incoming data (acts as a passthrough).
type TickerNode struct {
	*BaseNode
	interval time.Duration
	payload  []byte
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
}

// NewTickerNode creates a new TickerNode that fires at the specified interval
// and sends the specified payload.
func NewTickerNode(id string, interval time.Duration, payload []byte) *TickerNode {
	if interval <= 0 {
		panic("TickerNode requires interval > 0")
	}

	tn := &TickerNode{
		BaseNode: NewBaseNode(id),
		interval: interval,
		payload:  payload,
		done:     make(chan struct{}),
	}

	// Default process function acts as a passthrough
	tn.SetProcessFunc(func(input []byte) ([]byte, error) {
		return input, nil
	})

	return tn
}

// Create initializes the node and starts the ticker goroutine.
func (tn *TickerNode) Create() error {
	if err := tn.BaseNode.Create(); err != nil {
		return err
	}

	tn.ctx, tn.cancel = context.WithCancel(context.Background())

	go tn.tickerLoop()

	return nil
}

// Delete stops the ticker goroutine and cleans up resources.
func (tn *TickerNode) Delete() error {
	if tn.cancel != nil {
		tn.cancel()
		<-tn.done // wait for goroutine to exit
	}

	return tn.BaseNode.Delete()
}

func (tn *TickerNode) tickerLoop() {
	defer close(tn.done)

	ticker := time.NewTicker(tn.interval)
	defer ticker.Stop()

	for {
		select {
		case <-tn.ctx.Done():
			return
		case <-ticker.C:
			// Clone payload to prevent data races if subscribers mutate it
			payloadCopy := make([]byte, len(tn.payload))
			copy(payloadCopy, tn.payload)

			// Ignore notification errors in background
			_ = tn.Notify(payloadCopy)
		}
	}
}
