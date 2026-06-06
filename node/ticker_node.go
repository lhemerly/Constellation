package node

import (
	"context"
	"sync"
	"time"
)

// TickerNode generates periodic events and calls Notify on its subscribers.
type TickerNode struct {
	*BaseNode
	interval time.Duration
	payload  []byte
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	mu       sync.Mutex
	started  bool
}

// NewTickerNode creates a new TickerNode that ticks at the given interval.
func NewTickerNode(id string, interval time.Duration, payload []byte) *TickerNode {
	ctx, cancel := context.WithCancel(context.Background())
	return &TickerNode{
		BaseNode: NewBaseNode(id),
		interval: interval,
		payload:  payload,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Create initializes the node and starts the ticker loop.
func (tn *TickerNode) Create() error {
	if err := tn.BaseNode.Create(); err != nil {
		return err
	}

	tn.mu.Lock()
	defer tn.mu.Unlock()

	if tn.started {
		return nil
	}

	tn.started = true
	tn.wg.Add(1)
	go tn.tickerLoop()

	return nil
}

// tickerLoop continuously waits for the ticker interval or context cancellation.
func (tn *TickerNode) tickerLoop() {
	defer tn.wg.Done()

	ticker := time.NewTicker(tn.interval)
	defer ticker.Stop()

	for {
		select {
		case <-tn.ctx.Done():
			return
		case <-ticker.C:
			// Ensure we notify subscribers on each tick
			// We handle errors gracefully to avoid stopping the ticker
			_ = tn.Notify(tn.payload)
		}
	}
}

// Delete gracefully stops the ticker and waits for the goroutine to finish.
func (tn *TickerNode) Delete() error {
	tn.cancel()
	tn.wg.Wait()
	return tn.BaseNode.Delete()
}
