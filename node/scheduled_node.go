package node

import (
	"context"
	"sync"
	"time"
)

// ScheduledNode represents a node that acts as a periodic event source.
// It executes its process function on a defined interval, notifying its subscribers
// with the output.
type ScheduledNode struct {
	*BaseNode
	interval time.Duration
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	mu       sync.Mutex
	running  bool
}

// NewScheduledNode creates a new ScheduledNode with the given ID and interval.
func NewScheduledNode(id string, interval time.Duration) *ScheduledNode {
	ctx, cancel := context.WithCancel(context.Background())
	return &ScheduledNode{
		BaseNode: NewBaseNode(id),
		interval: interval,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Create starts the internal timer that triggers the process function periodically.
func (sn *ScheduledNode) Create() error {
	if err := sn.BaseNode.Create(); err != nil {
		return err
	}

	sn.mu.Lock()
	defer sn.mu.Unlock()

	if sn.running {
		return nil
	}
	sn.running = true

	// In case Create is called again after Delete
	if sn.ctx.Err() != nil {
		sn.ctx, sn.cancel = context.WithCancel(context.Background())
	}

	sn.wg.Add(1)
	go sn.run()

	return nil
}

// Delete stops the internal timer and waits for the goroutine to finish.
func (sn *ScheduledNode) Delete() error {
	sn.mu.Lock()
	if !sn.running {
		sn.mu.Unlock()
		return sn.BaseNode.Delete()
	}
	sn.running = false
	sn.cancel()
	sn.mu.Unlock()

	sn.wg.Wait()
	return sn.BaseNode.Delete()
}

// run is the background loop that executes on the configured interval.
func (sn *ScheduledNode) run() {
	defer sn.wg.Done()
	ticker := time.NewTicker(sn.interval)
	defer ticker.Stop()

	for {
		select {
		case <-sn.ctx.Done():
			return
		case <-ticker.C:
			// Process an empty payload or a timestamp. For flexibility, pass empty.
			output, err := sn.Process(nil)
			if err == nil && output != nil {
				// Broadcast the result to subscribers
				_ = sn.Notify(output)
			}
		}
	}
}
