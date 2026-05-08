package node_test

import (
	"github.com/lhemerly/Constellation/node"
	"sync/atomic"
	"testing"
	"time"
)

func TestTeeNode(t *testing.T) {
	observerCalled := atomic.Int32{}
	observerNode := node.NewBaseNode("observer-node")
	defer cleanupNodes(t, []node.Node{observerNode})

	observerNode.SetProcessFunc(func(input []byte) ([]byte, error) {
		if string(input) != "test-data" {
			t.Errorf("observer expected 'test-data', got %s", string(input))
		}
		observerCalled.Add(1)
		return nil, nil
	})

	primaryNode := node.NewBaseNode("primary-node")
	defer cleanupNodes(t, []node.Node{primaryNode})

	primaryNode.SetProcessFunc(func(input []byte) ([]byte, error) {
		if string(input) != "test-data" {
			t.Errorf("primary expected 'test-data', got %s", string(input))
		}
		return append([]byte("processed-"), input...), nil
	})

	teeNode := node.NewTeeNode("tee-node", observerNode, primaryNode)
	defer cleanupNodes(t, []node.Node{teeNode})

	// Process
	res, err := teeNode.Process([]byte("test-data"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify primary behavior
	if string(res) != "processed-test-data" {
		t.Errorf("expected 'processed-test-data', got %s", string(res))
	}

	// Wait for observer to process asynchronously
	time.Sleep(50 * time.Millisecond)

	// Verify observer behavior
	if observerCalled.Load() != 1 {
		t.Errorf("expected observer to be called 1 time, got %d", observerCalled.Load())
	}
}

func TestTeeNode_NilPrimary(t *testing.T) {
	observerCalled := atomic.Int32{}
	observerNode := node.NewBaseNode("observer-node")
	defer cleanupNodes(t, []node.Node{observerNode})

	observerNode.SetProcessFunc(func(input []byte) ([]byte, error) {
		observerCalled.Add(1)
		return nil, nil
	})

	teeNode := node.NewTeeNode("tee-node", observerNode, nil)
	defer cleanupNodes(t, []node.Node{teeNode})

	res, err := teeNode.Process([]byte("test-data"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(res) != "test-data" {
		t.Errorf("expected input to be returned untouched, got %s", string(res))
	}

	time.Sleep(50 * time.Millisecond)

	if observerCalled.Load() != 1 {
		t.Errorf("expected observer to be called 1 time, got %d", observerCalled.Load())
	}
}

func TestTeeNode_NilObserver(t *testing.T) {
	primaryNode := node.NewBaseNode("primary-node")
	defer cleanupNodes(t, []node.Node{primaryNode})

	primaryNode.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("primary-"), input...), nil
	})

	teeNode := node.NewTeeNode("tee-node", nil, primaryNode)
	defer cleanupNodes(t, []node.Node{teeNode})

	res, err := teeNode.Process([]byte("test-data"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(res) != "primary-test-data" {
		t.Errorf("expected 'primary-test-data', got %s", string(res))
	}
}
