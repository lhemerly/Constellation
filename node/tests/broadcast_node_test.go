package node_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/lhemerly/Constellation/node"
)

func TestBroadcastNode_Process(t *testing.T) {
	bn := node.NewBroadcastNode("broadcaster")
	if err := bn.Create(); err != nil {
		t.Fatalf("Failed to create BroadcastNode: %v", err)
	}
	defer bn.Delete()

	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-1")...), nil
	})

	n2 := node.NewBaseNode("n2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-2")...), nil
	})

	bn.Subscribe(n1)
	bn.Subscribe(n2)

	// In Go map iteration is random, but since we collect to a slice in BroadcastNode it might be random.
	// Actually, wait, our implementation returns deterministic order?
	// Our map iteration in BroadcastNode logic:
	// subs := make([]Node, 0, len(b.subscriptions))
	// for _, sub := range b.subscriptions { subs = append(subs, sub) }
	// This will result in random order of nodes!
	// Which means the index is random!
	// Wait, is it random? Let's check our BroadcastNode implementation.
	// "orderedResults[res.index] = res.output"
	// `res.index` is based on the iteration order of the map. So it will append correctly relative to the slice...
	// but the slice itself has randomly ordered subscribers.

	out, err := bn.Process([]byte("test"))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// We expect "test-1" and "test-2" in some order because of map iteration
	expected1 := []byte("test-1test-2")
	expected2 := []byte("test-2test-1")

	if !bytes.Equal(out, expected1) && !bytes.Equal(out, expected2) {
		t.Errorf("Expected %s or %s, got %s", expected1, expected2, string(out))
	}
}

func TestBroadcastNode_ProcessError(t *testing.T) {
	bn := node.NewBroadcastNode("broadcaster")
	defer bn.Delete()

	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("sub node error")
	})

	bn.Subscribe(n1)

	_, err := bn.Process([]byte("test"))
	if err == nil || err.Error() != "sub node error" {
		t.Errorf("Expected 'sub node error', got %v", err)
	}
}
