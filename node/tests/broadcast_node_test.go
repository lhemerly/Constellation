package node_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestBroadcastNode_Success(t *testing.T) {
	bn := node.NewBroadcastNode("broadcast-1", 1*time.Second)
	defer cleanupNodes(t, []node.Node{bn})

	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("A"), nil
	})
	n2 := node.NewBaseNode("n2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("B"), nil
	})
	defer cleanupNodes(t, []node.Node{n1, n2})

	bn.AddNode(n1)
	bn.AddNode(n2)

	output, err := bn.Process([]byte("input"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	outStr := string(output)
	// Order is not guaranteed, but it should contain both A and B
	if len(outStr) != 2 || !strings.Contains(outStr, "A") || !strings.Contains(outStr, "B") {
		t.Errorf("expected combined output of A and B, got %s", outStr)
	}
}

func TestBroadcastNode_Error(t *testing.T) {
	bn := node.NewBroadcastNode("broadcast-1", 1*time.Second)
	defer cleanupNodes(t, []node.Node{bn})

	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("error from n1")
	})
	n2 := node.NewBaseNode("n2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("B"), nil
	})
	defer cleanupNodes(t, []node.Node{n1, n2})

	bn.AddNode(n1)
	bn.AddNode(n2)

	_, err := bn.Process([]byte("input"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "error from n1") {
		t.Errorf("expected error to contain 'error from n1', got %v", err)
	}
}

func TestBroadcastNode_Timeout(t *testing.T) {
	bn := node.NewBroadcastNode("broadcast-1", 50*time.Millisecond)
	defer cleanupNodes(t, []node.Node{bn})

	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(100 * time.Millisecond)
		return []byte("A"), nil
	})
	defer cleanupNodes(t, []node.Node{n1})

	bn.AddNode(n1)

	_, err := bn.Process([]byte("input"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != "broadcast timed out" {
		t.Errorf("expected timeout error, got %v", err)
	}
}

func TestBroadcastNode_Empty(t *testing.T) {
	bn := node.NewBroadcastNode("broadcast-1", 1*time.Second)
	defer cleanupNodes(t, []node.Node{bn})

	output, err := bn.Process([]byte("input"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != nil {
		t.Errorf("expected nil output, got %s", string(output))
	}
}
