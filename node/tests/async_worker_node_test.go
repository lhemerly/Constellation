package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"sync/atomic"
	"testing"
	"time"
)

func TestAsyncWorkerNode_Processing(t *testing.T) {
	n := node.NewAsyncWorkerNode("async-node", 2, 10)
	defer cleanupNodes(t, []node.Node{n})

	if err := n.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	var processedCount int32
	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		atomic.AddInt32(&processedCount, 1)
		return input, nil
	})

	// Dispatch tasks
	for i := 0; i < 5; i++ {
		_, err := n.Process([]byte("task"))
		if err != nil {
			t.Fatalf("unexpected error during process: %v", err)
		}
	}

	// Give workers time to process
	time.Sleep(50 * time.Millisecond)

	count := atomic.LoadInt32(&processedCount)
	if count != 5 {
		t.Errorf("expected 5 tasks to be processed, got %d", count)
	}
}

func TestAsyncWorkerNode_QueueFull(t *testing.T) {
	// 1 worker, queue capacity of 1
	n := node.NewAsyncWorkerNode("async-node-full", 1, 1)
	defer cleanupNodes(t, []node.Node{n})

	if err := n.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	blockCh := make(chan struct{})
	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		<-blockCh // block the worker to simulate slow processing
		return input, nil
	})

	// 1st task: worker picks it up and blocks
	_, err := n.Process([]byte("task 1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Small sleep to ensure the worker pulled task 1 off the queue
	time.Sleep(10 * time.Millisecond)

	// 2nd task: fills the queue capacity of 1
	_, err = n.Process([]byte("task 2"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 3rd task: queue is full, should fail immediately
	_, err = n.Process([]byte("task 3"))
	if !errors.Is(err, node.ErrQueueFull) {
		t.Errorf("expected ErrQueueFull, got %v", err)
	}

	close(blockCh) // unblock the worker to allow graceful teardown
}

func TestAsyncWorkerNode_Deleted(t *testing.T) {
	n := node.NewAsyncWorkerNode("async-node-deleted", 1, 10)
	// No defer cleanup needed because we are explicitly testing Delete

	if err := n.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	if err := n.Delete(); err != nil {
		t.Fatalf("failed to delete node: %v", err)
	}

	// Process after deletion
	_, err := n.Process([]byte("task"))
	if !errors.Is(err, node.ErrNodeDeleted) {
		t.Errorf("expected ErrNodeDeleted, got %v", err)
	}
}
