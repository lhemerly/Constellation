package node_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

type MockTargetNode struct {
	node.Node
	received atomic.Int32
	lastData []byte
	mu       sync.Mutex
}

func (m *MockTargetNode) Process(input []byte) ([]byte, error) {
	m.received.Add(1)
	m.mu.Lock()
	m.lastData = make([]byte, len(input))
	copy(m.lastData, input)
	m.mu.Unlock()
	return nil, nil
}

func (m *MockTargetNode) Create() error { return nil }
func (m *MockTargetNode) Delete() error { return nil }

func TestBatchingNode_SizeLimit(t *testing.T) {
	target := &MockTargetNode{}
	bn := node.NewBatchingNode("batcher", 3, 1*time.Minute, target)

	err := bn.Create()
	if err != nil {
		t.Fatalf("unexpected error creating node: %v", err)
	}

	_, _ = bn.Process([]byte("a"))
	_, _ = bn.Process([]byte("b"))

	if target.received.Load() != 0 {
		t.Errorf("expected 0 received batches, got %d", target.received.Load())
	}

	_, _ = bn.Process([]byte("c")) // This should trigger the flush

	// Wait a tiny bit for async flush
	time.Sleep(50 * time.Millisecond)

	if target.received.Load() != 1 {
		t.Errorf("expected 1 received batches, got %d", target.received.Load())
	}

	target.mu.Lock()
	if string(target.lastData) != "abc" {
		t.Errorf("expected target to receive 'abc', got '%s'", string(target.lastData))
	}
	target.mu.Unlock()

	cleanupNodes(t, []node.Node{bn})
}

func TestBatchingNode_Timeout(t *testing.T) {
	target := &MockTargetNode{}
	bn := node.NewBatchingNode("batcher", 10, 50*time.Millisecond, target)

	err := bn.Create()
	if err != nil {
		t.Fatalf("unexpected error creating node: %v", err)
	}

	_, _ = bn.Process([]byte("x"))
	_, _ = bn.Process([]byte("y"))

	if target.received.Load() != 0 {
		t.Errorf("expected 0 received batches initially, got %d", target.received.Load())
	}

	// Wait for the timeout to hit
	time.Sleep(100 * time.Millisecond)

	if target.received.Load() != 1 {
		t.Errorf("expected 1 received batches after timeout, got %d", target.received.Load())
	}

	target.mu.Lock()
	if string(target.lastData) != "xy" {
		t.Errorf("expected target to receive 'xy', got '%s'", string(target.lastData))
	}
	target.mu.Unlock()

	cleanupNodes(t, []node.Node{bn})
}
