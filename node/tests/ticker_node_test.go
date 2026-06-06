package node_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestTickerNode(t *testing.T) {
	interval := 50 * time.Millisecond
	payload := []byte("tick")

	ticker := node.NewTickerNode("ticker-1", interval, payload)
	err := ticker.Create()
	if err != nil {
		t.Fatalf("Failed to create TickerNode: %v", err)
	}

	subscriber := node.NewBaseNode("sub-1")
	err = subscriber.Create()
	if err != nil {
		t.Fatalf("Failed to create subscriber: %v", err)
	}
	defer func() {
		_ = subscriber.Delete()
	}()

	var count int32
	subscriber.SetProcessFunc(func(input []byte) ([]byte, error) {
		atomic.AddInt32(&count, 1)
		return input, nil
	})

	err = ticker.Subscribe(subscriber)
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Wait for roughly 3 ticks
	time.Sleep(160 * time.Millisecond)

	err = ticker.Delete()
	if err != nil {
		t.Fatalf("Failed to delete TickerNode: %v", err)
	}

	finalCount := atomic.LoadInt32(&count)

	if finalCount < 2 || finalCount > 4 {
		t.Fatalf("Expected between 2 and 4 ticks, got %d", finalCount)
	}
}
