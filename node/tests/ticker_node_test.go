package node_test

import (
	"sync"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestTickerNode(t *testing.T) {
	interval := 50 * time.Millisecond
	payload := []byte("ping")

	tNode := node.NewTickerNode("ticker-node", interval, payload)

	subNode := node.NewBaseNode("sub-node")
	var mu sync.Mutex
	var receivedCount int

	subNode.SetProcessFunc(func(input []byte) ([]byte, error) {
		mu.Lock()
		defer mu.Unlock()
		if string(input) == string(payload) {
			receivedCount++
		}
		return input, nil
	})

	tNode.Subscribe(subNode)

	err := tNode.Create()
	if err != nil {
		t.Fatalf("Expected no error creating TickerNode, got %v", err)
	}

	// Wait enough time for a few ticks
	time.Sleep(170 * time.Millisecond)

	// Clean up which should stop the ticker
	err = tNode.Delete()
	if err != nil {
		t.Fatalf("Expected no error deleting TickerNode, got %v", err)
	}

	mu.Lock()
	count := receivedCount
	mu.Unlock()

	// 170ms / 50ms interval should result in ~3 ticks.
	if count < 2 || count > 4 {
		t.Errorf("Expected between 2 and 4 ticks, got %d", count)
	}

	// Verify no more ticks are sent after delete
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	countAfterDelete := receivedCount
	mu.Unlock()

	if countAfterDelete != count {
		t.Errorf("Expected no more ticks after delete, got %d, previously had %d", countAfterDelete, count)
	}
}
