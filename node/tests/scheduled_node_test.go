package node_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestScheduledNode_RunLoop(t *testing.T) {
	// A very short interval to trigger the process fast
	sn := node.NewScheduledNode("scheduler", 10*time.Millisecond, []byte("tick"))

	var callCount int32
	sn.SetProcessFunc(func(input []byte) ([]byte, error) {
		atomic.AddInt32(&callCount, 1)
		return input, nil
	})

	if err := sn.Create(); err != nil {
		t.Fatalf("Failed to create ScheduledNode: %v", err)
	}

	// Wait for a few ticks
	time.Sleep(35 * time.Millisecond)

	// Clean up / stop loop
	if err := sn.Delete(); err != nil {
		t.Fatalf("Failed to delete ScheduledNode: %v", err)
	}

	// Expected calls: ~3 (depending on scheduling, at least 1)
	count := atomic.LoadInt32(&callCount)
	if count < 1 {
		t.Errorf("Expected at least 1 tick execution, got %d", count)
	}

	// Ensure that after Delete(), no more ticks are processed
	time.Sleep(30 * time.Millisecond)
	countAfterDelete := atomic.LoadInt32(&callCount)
	if countAfterDelete > count {
		t.Errorf("ScheduledNode executed after Delete! before=%d, after=%d", count, countAfterDelete)
	}
}
