package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"sync/atomic"
	"testing"
)

func TestBroadcastNode_Success(t *testing.T) {
	b := node.NewBroadcastNode("broadcaster")
	defer cleanupNodes(t, []node.Node{b})

	n1 := node.NewBaseNode("dest1")
	n2 := node.NewBaseNode("dest2")
	defer cleanupNodes(t, []node.Node{n1, n2})

	var counter1, counter2 int32

	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		atomic.AddInt32(&counter1, 1)
		return nil, nil
	})

	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		atomic.AddInt32(&counter2, 1)
		return nil, nil
	})

	b.AddDestination(n1)
	b.AddDestination(n2)

	res, err := b.Process([]byte("broadcast-msg"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(res) != "broadcast-msg" {
		t.Errorf("expected original input 'broadcast-msg', got %s", string(res))
	}

	if atomic.LoadInt32(&counter1) != 1 {
		t.Errorf("expected dest1 to be called 1 time, got %d", counter1)
	}
	if atomic.LoadInt32(&counter2) != 1 {
		t.Errorf("expected dest2 to be called 1 time, got %d", counter2)
	}
}

func TestBroadcastNode_PartialFailure(t *testing.T) {
	b := node.NewBroadcastNode("broadcaster-err")
	defer cleanupNodes(t, []node.Node{b})

	n1 := node.NewBaseNode("dest1")
	n2 := node.NewBaseNode("dest2")
	defer cleanupNodes(t, []node.Node{n1, n2})

	expectedErr := errors.New("simulated error")

	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, expectedErr
	})

	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, nil
	})

	b.AddDestination(n1)
	b.AddDestination(n2)

	_, err := b.Process([]byte("broadcast-msg"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf("expected err to contain expectedErr, got: %v", err)
	}
}

func TestBroadcastNode_NoDestinations(t *testing.T) {
	b := node.NewBroadcastNode("broadcaster-empty")
	defer cleanupNodes(t, []node.Node{b})

	res, err := b.Process([]byte("input"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(res) != "input" {
		t.Errorf("expected 'input', got %s", string(res))
	}
}
