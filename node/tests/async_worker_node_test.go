package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"testing"
	"time"
)

func TestAsyncWorkerNode_Success(t *testing.T) {
	n := node.NewAsyncWorkerNode("async-node", 2, 5)
	defer cleanupNodes(t, []node.Node{n})

	if err := n.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("processed "), input...), nil
	})

	res, err := n.Process([]byte("data"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(res) != "processed data" {
		t.Errorf("expected 'processed data', got '%s'", string(res))
	}
}

func TestAsyncWorkerNode_QueueFull(t *testing.T) {
	n := node.NewAsyncWorkerNode("async-node-full", 1, 1)
	defer cleanupNodes(t, []node.Node{n})

	if err := n.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	blockCh := make(chan struct{})
	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		<-blockCh
		return input, nil
	})
	defer close(blockCh) // release worker on teardown

	// 1. Dispatch first request, which will get picked up by the worker and block
	go n.Process([]byte("req1"))
	time.Sleep(10 * time.Millisecond) // Let worker pick it up

	// 2. Dispatch second request, which will fill the queue (queue size 1)
	go n.Process([]byte("req2"))
	time.Sleep(10 * time.Millisecond) // Let queue fill

	// 3. Dispatch third request, which should fail with ErrQueueFull
	_, err := n.Process([]byte("req3"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !errors.Is(err, node.ErrQueueFull) {
		t.Errorf("expected ErrQueueFull, got %v", err)
	}
}

func TestAsyncWorkerNode_Shutdown(t *testing.T) {
	n := node.NewAsyncWorkerNode("async-node-shutdown", 1, 5)
	// Don't defer cleanup here since we test Delete explicitly

	if err := n.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	blockCh := make(chan struct{})
	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		<-blockCh
		return input, nil
	})

	// Dispatch request, it gets picked up and blocks
	go n.Process([]byte("req1"))

	// Dispatch another request, sits in queue
	go n.Process([]byte("req2"))

	time.Sleep(10 * time.Millisecond) // Ensure it's queued

	// Shut down while requests are in flight/queue
	close(blockCh) // release the blocked worker
	if err := n.Delete(); err != nil {
		t.Fatalf("unexpected delete error: %v", err)
	}

	// Try processing after shutdown
	_, err := n.Process([]byte("req3"))
	if err == nil {
		t.Fatalf("expected error after shutdown, got nil")
	}
	if err.Error() != "worker pool deleted" {
		t.Errorf("expected 'worker pool deleted', got %v", err)
	}
}
