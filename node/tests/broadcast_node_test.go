package node_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/lhemerly/Constellation/node"
)

func TestBroadcastNode_Success(t *testing.T) {
	bn := node.NewBroadcastNode("broadcast-1")

	n1 := node.NewBaseNode("node-1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-1")...), nil
	})

	n2 := node.NewBaseNode("node-2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-2")...), nil
	})

	bn.AddNode(n1)
	bn.AddNode(n2)

	err := bn.Create()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	defer func() {
		_ = bn.Delete()
	}()

	res, err := bn.Process([]byte("test"))
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	resStr := string(res)
	if !strings.Contains(resStr, "test-1") || !strings.Contains(resStr, "test-2") {
		t.Errorf("expected result to contain 'test-1' and 'test-2', got %s", resStr)
	}
}

func TestBroadcastNode_Error(t *testing.T) {
	bn := node.NewBroadcastNode("broadcast-2")

	n1 := node.NewBaseNode("node-1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-1")...), nil
	})

	n2 := node.NewBaseNode("node-2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("node-2 error")
	})

	bn.AddNode(n1)
	bn.AddNode(n2)

	err := bn.Create()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	defer func() {
		_ = bn.Delete()
	}()

	_, err = bn.Process([]byte("test"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "broadcast failed") || !strings.Contains(err.Error(), "node-2 error") {
		t.Errorf("expected error to contain 'broadcast failed' and 'node-2 error', got %v", err)
	}
}

func TestBroadcastNode_NoNodes(t *testing.T) {
	bn := node.NewBroadcastNode("broadcast-3")

	err := bn.Create()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	defer func() {
		_ = bn.Delete()
	}()

	_, err = bn.Process([]byte("test"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if err.Error() != "no destination nodes available" {
		t.Errorf("expected 'no destination nodes available' error, got %v", err)
	}
}

func TestBroadcastNode_AddRemoveNodes(t *testing.T) {
	bn := node.NewBroadcastNode("broadcast-4")

	n1 := node.NewBaseNode("node-1")
	n2 := node.NewBaseNode("node-2")

	bn.AddNode(n1)
	bn.AddNode(n2)

	nodes := bn.GetNodes()
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}

	bn.RemoveNode("node-1")

	nodes = bn.GetNodes()
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}

	if nodes[0].GetID() != "node-2" {
		t.Errorf("expected node-2 to be the remaining node, got %s", nodes[0].GetID())
	}
}
