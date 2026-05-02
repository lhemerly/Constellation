package node_test

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestBatchingNode_BatchSize(t *testing.T) {
	bn := node.NewBatchingNode("batch-node", 3, 1*time.Minute) // Long flush interval so it only triggers by size
	if err := bn.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	defer cleanupNodes(t, []node.Node{bn})

	// Setup a subscriber to receive the flush
	sub := node.NewBaseNode("sub")
	var receivedData []byte
	var wg sync.WaitGroup

	wg.Add(1)
	sub.SetProcessFunc(func(input []byte) ([]byte, error) {
		receivedData = input
		wg.Done()
		return nil, nil
	})

	bn.Subscribe(sub)

	// Send 3 requests to trigger a batch size flush
	bn.Process([]byte("a"))
	bn.Process([]byte("b"))
	bn.Process([]byte("c"))

	// Wait for the subscriber to get the flush
	wg.Wait()

	expected := []byte("abc")
	if !bytes.Equal(receivedData, expected) {
		t.Fatalf("expected %s, got %s", expected, receivedData)
	}
}

func TestBatchingNode_FlushInterval(t *testing.T) {
	bn := node.NewBatchingNode("batch-node", 100, 100*time.Millisecond) // Short flush interval
	if err := bn.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	defer cleanupNodes(t, []node.Node{bn})

	// Setup a subscriber
	sub := node.NewBaseNode("sub")
	var receivedData []byte
	var wg sync.WaitGroup

	wg.Add(1)
	sub.SetProcessFunc(func(input []byte) ([]byte, error) {
		receivedData = input
		wg.Done()
		return nil, nil
	})

	bn.Subscribe(sub)

	// Send requests that don't fill the batch size
	bn.Process([]byte("1"))
	bn.Process([]byte("2"))

	// Wait for the timer to trigger the flush
	wg.Wait()

	expected := []byte("12")
	if !bytes.Equal(receivedData, expected) {
		t.Fatalf("expected %s, got %s", expected, receivedData)
	}
}

func TestBatchingNode_DeleteFlushesRemaining(t *testing.T) {
	bn := node.NewBatchingNode("batch-node", 100, 1*time.Minute)
	if err := bn.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	sub := node.NewBaseNode("sub")
	var receivedData []byte
	var wg sync.WaitGroup

	wg.Add(1)
	sub.SetProcessFunc(func(input []byte) ([]byte, error) {
		receivedData = input
		wg.Done()
		return nil, nil
	})

	bn.Subscribe(sub)

	bn.Process([]byte("shutdown"))

	// Deleting should flush remaining items
	err := bn.Delete()
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	wg.Wait()

	expected := []byte("shutdown")
	if !bytes.Equal(receivedData, expected) {
		t.Fatalf("expected %s, got %s", expected, receivedData)
	}
}
