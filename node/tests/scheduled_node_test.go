package node_test

import (
	"github.com/lhemerly/Constellation/node"
	"sync/atomic"
	"testing"
	"time"
)

func TestScheduledNode_Process(t *testing.T) {
	n := node.NewScheduledNode("sched-1", 10*time.Millisecond, []byte("tick"), false)
	defer cleanupNodes(t, []node.Node{n})

	var counter int32
	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		if string(input) == "tick" {
			atomic.AddInt32(&counter, 1)
		}
		return nil, nil
	})

	if err := n.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	// Wait for a few ticks
	time.Sleep(35 * time.Millisecond)

	count := atomic.LoadInt32(&counter)
	if count < 2 {
		t.Errorf("expected at least 2 ticks, got %d", count)
	}
}

func TestScheduledNode_Notify(t *testing.T) {
	n := node.NewScheduledNode("sched-2", 10*time.Millisecond, []byte("notify"), true)
	defer cleanupNodes(t, []node.Node{n})

	sub := node.NewBaseNode("sub-1")
	defer cleanupNodes(t, []node.Node{sub})

	var counter int32
	sub.SetProcessFunc(func(input []byte) ([]byte, error) {
		if string(input) == "notify" {
			atomic.AddInt32(&counter, 1)
		}
		return nil, nil
	})

	if err := n.Subscribe(sub); err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	if err := n.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	// Wait for a few ticks
	time.Sleep(35 * time.Millisecond)

	count := atomic.LoadInt32(&counter)
	if count < 2 {
		t.Errorf("expected at least 2 ticks, got %d", count)
	}
}

func TestScheduledNode_Delete(t *testing.T) {
	n := node.NewScheduledNode("sched-3", 10*time.Millisecond, []byte("tick"), false)

	var counter int32
	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		atomic.AddInt32(&counter, 1)
		return nil, nil
	})

	if err := n.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	time.Sleep(15 * time.Millisecond) // Let it tick once

	if err := n.Delete(); err != nil {
		t.Fatalf("failed to delete node: %v", err)
	}

	countAfterDelete := atomic.LoadInt32(&counter)

	// Wait to ensure no more ticks happen
	time.Sleep(20 * time.Millisecond)

	finalCount := atomic.LoadInt32(&counter)
	if finalCount > countAfterDelete {
		t.Errorf("expected node to stop ticking after delete, count went from %d to %d", countAfterDelete, finalCount)
	}
}
