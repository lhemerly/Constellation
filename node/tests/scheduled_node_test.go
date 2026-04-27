package node_test

import (
	"github.com/lhemerly/Constellation/node"
	"sync/atomic"
	"testing"
	"time"
)

func TestScheduledNode(t *testing.T) {
	interval := 50 * time.Millisecond
	sn := node.NewScheduledNode("scheduled-node", interval)
	defer cleanupNodes(t, []node.Node{sn})

	var executionCount int32
	sn.SetProcessFunc(func(input []byte) ([]byte, error) {
		atomic.AddInt32(&executionCount, 1)
		return []byte("tick"), nil
	})

	// Add a subscriber to receive the output
	subscriber := node.NewBaseNode("subscriber")
	defer cleanupNodes(t, []node.Node{subscriber})

	var notificationCount int32
	subscriber.SetProcessFunc(func(input []byte) ([]byte, error) {
		if string(input) == "tick" {
			atomic.AddInt32(&notificationCount, 1)
		}
		return input, nil
	})

	err := sn.Subscribe(subscriber)
	if err != nil {
		t.Fatalf("unexpected error subscribing: %v", err)
	}

	err = sn.Create()
	if err != nil {
		t.Fatalf("unexpected error creating scheduled node: %v", err)
	}

	// Wait long enough for a few ticks to happen
	time.Sleep(175 * time.Millisecond)

	// Clean up which stops the scheduler
	err = sn.Delete()
	if err != nil {
		t.Fatalf("unexpected error deleting scheduled node: %v", err)
	}

	count := atomic.LoadInt32(&executionCount)
	notifs := atomic.LoadInt32(&notificationCount)

	// In 175ms with 50ms interval, we expect roughly 3 ticks (50, 100, 150)
	if count < 2 || count > 4 {
		t.Errorf("expected between 2 and 4 executions, got %d", count)
	}

	if notifs < 2 || notifs > 4 {
		t.Errorf("expected between 2 and 4 notifications, got %d", notifs)
	}

	// Ensure it stopped
	time.Sleep(100 * time.Millisecond)

	newCount := atomic.LoadInt32(&executionCount)
	if newCount != count {
		t.Errorf("expected execution count to remain %d after delete, but got %d", count, newCount)
	}
}
