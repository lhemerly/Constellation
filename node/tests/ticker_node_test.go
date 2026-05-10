package node_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestTickerNode(t *testing.T) {
	tickerNode := node.NewTickerNode("ticker-1", 50*time.Millisecond, []byte("tick"))

	// Create a subscriber to receive ticks
	subscriber := node.NewBaseNode("sub-1")
	var receivedCount int32

	subscriber.SetProcessFunc(func(input []byte) ([]byte, error) {
		if string(input) == "tick" {
			atomic.AddInt32(&receivedCount, 1)
		}
		return input, nil
	})

	err := tickerNode.Subscribe(subscriber)
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	err = tickerNode.Create()
	if err != nil {
		t.Fatalf("Failed to create TickerNode: %v", err)
	}

	// Wait for a few ticks (3 ticks should take ~150ms)
	time.Sleep(200 * time.Millisecond)

	err = tickerNode.Delete()
	if err != nil {
		t.Fatalf("Failed to delete TickerNode: %v", err)
	}

	finalCount := atomic.LoadInt32(&receivedCount)
	if finalCount < 2 {
		t.Errorf("Expected at least 2 ticks, got %d", finalCount)
	}

	// Wait a bit to ensure it really stopped
	time.Sleep(100 * time.Millisecond)
	countAfterDelete := atomic.LoadInt32(&receivedCount)
	if countAfterDelete != finalCount {
		t.Errorf("TickerNode continued ticking after Delete! Final count was %d, but increased to %d", finalCount, countAfterDelete)
	}
}
