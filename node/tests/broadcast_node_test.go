package node_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestBroadcastNode_Process(t *testing.T) {
	var count atomic.Int32

	target1 := node.NewBaseNode("target-1")
	target1.SetProcessFunc(func(input []byte) ([]byte, error) {
		count.Add(1)
		return nil, nil
	})

	target2 := node.NewBaseNode("target-2")
	target2.SetProcessFunc(func(input []byte) ([]byte, error) {
		count.Add(1)
		return nil, nil
	})

	bn := node.NewBroadcastNode("broadcast", []node.Node{target1, target2})

	err := bn.Create()
	if err != nil {
		t.Fatalf("expected no error on create, got: %v", err)
	}

	input := []byte("broadcast message")
	output, err := bn.Process(input)
	if err != nil {
		t.Fatalf("expected no error on process, got: %v", err)
	}

	if string(output) != "broadcast message" {
		t.Errorf("expected output to match input, got: %s", string(output))
	}

	// Wait briefly for goroutines to execute
	time.Sleep(50 * time.Millisecond)

	if count.Load() != 2 {
		t.Errorf("expected 2 targets to process, got: %d", count.Load())
	}

	cleanupNodes(t, []node.Node{bn})
}

func TestBroadcastNode_Delete(t *testing.T) {
	target1 := node.NewBaseNode("target-1")
	bn := node.NewBroadcastNode("broadcast", []node.Node{target1})
	err := bn.Create()
	if err != nil {
		t.Fatalf("expected no error on create, got: %v", err)
	}

	err = bn.Delete()
	if err != nil {
		t.Fatalf("expected no error on delete, got: %v", err)
	}
}
