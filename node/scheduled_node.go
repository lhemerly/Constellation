package node

import (
	"context"
	"sync"
	"time"
)

// ScheduledNode fires a predefined payload periodically to its Process method or Notify.
type ScheduledNode struct {
	*BaseNode
	interval time.Duration
	payload  []byte
	ticker   *time.Ticker
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	notify   bool // if true, calls Notify instead of Process
}

// NewScheduledNode creates a new ScheduledNode.
// If notify is true, it calls Notify(payload) on each tick. Otherwise, it calls Process(payload).
func NewScheduledNode(id string, interval time.Duration, payload []byte, notify bool) *ScheduledNode {
	ctx, cancel := context.WithCancel(context.Background())
	return &ScheduledNode{
		BaseNode: NewBaseNode(id),
		interval: interval,
		payload:  payload,
		ctx:      ctx,
		cancel:   cancel,
		notify:   notify,
	}
}

// Create initializes the node and starts the scheduled loop.
func (n *ScheduledNode) Create() error {
	if err := n.BaseNode.Create(); err != nil {
		return err
	}

	n.ticker = time.NewTicker(n.interval)
	n.wg.Add(1)
	go n.loop()

	return nil
}

func (n *ScheduledNode) loop() {
	defer n.wg.Done()
	for {
		select {
		case <-n.ctx.Done():
			if n.ticker != nil {
				n.ticker.Stop()
			}
			return
		case <-n.ticker.C:
			if n.notify {
				_ = n.Notify(n.payload)
			} else {
				_, _ = n.Process(n.payload)
			}
		}
	}
}

// Delete stops the ticker and cleans up resources.
func (n *ScheduledNode) Delete() error {
	n.cancel()
	n.wg.Wait()
	return n.BaseNode.Delete()
}
