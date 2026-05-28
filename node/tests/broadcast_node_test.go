package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"strings"
	"testing"
)

func TestBroadcastNode_Success(t *testing.T) {
	bn := node.NewBroadcastNode("broadcast-node")
	defer cleanupNodes(t, []node.Node{bn})

	n1 := node.NewBaseNode("node1")
	n2 := node.NewBaseNode("node2")
	n3 := node.NewBaseNode("node3")
	defer cleanupNodes(t, []node.Node{n1, n2, n3})

	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("A"), nil
	})
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("B"), nil
	})
	n3.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("C"), nil
	})

	bn.AddNode(n1)
	bn.AddNode(n2)
	bn.AddNode(n3)

	output, err := bn.Process([]byte("input"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	strOutput := string(output)
	// Because go routines order is undefined, we expect output to be length 3 and contain A, B, and C
	if len(strOutput) != 3 {
		t.Fatalf("expected output length 3, got %d", len(strOutput))
	}
	if !strings.Contains(strOutput, "A") || !strings.Contains(strOutput, "B") || !strings.Contains(strOutput, "C") {
		t.Errorf("expected output to contain A, B, and C. got %s", strOutput)
	}
}

func TestBroadcastNode_Errors(t *testing.T) {
	bn := node.NewBroadcastNode("broadcast-err-node")
	defer cleanupNodes(t, []node.Node{bn})

	n1 := node.NewBaseNode("node1")
	n2 := node.NewBaseNode("node2")
	defer cleanupNodes(t, []node.Node{n1, n2})

	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("err1")
	})
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("err2")
	})

	bn.AddNode(n1)
	bn.AddNode(n2)

	_, err := bn.Process([]byte("input"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "err1") || !strings.Contains(errStr, "err2") {
		t.Errorf("expected error to contain err1 and err2. got %s", errStr)
	}
}

func TestBroadcastNode_Empty(t *testing.T) {
	bn := node.NewBroadcastNode("broadcast-empty-node")
	defer cleanupNodes(t, []node.Node{bn})

	_, err := bn.Process([]byte("input"))
	if !errors.Is(err, node.ErrNoNodes) {
		t.Fatalf("expected ErrNoNodes, got %v", err)
	}
}

func TestBroadcastNode_AddRemove(t *testing.T) {
	bn := node.NewBroadcastNode("broadcast-add-remove-node")
	defer cleanupNodes(t, []node.Node{bn})

	n1 := node.NewBaseNode("node1")
	n2 := node.NewBaseNode("node2")
	defer cleanupNodes(t, []node.Node{n1, n2})

	bn.AddNode(n1)
	bn.AddNode(n2)

	nodes := bn.GetNodes()
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}

	bn.RemoveNode("node1")
	nodes = bn.GetNodes()
	if len(nodes) != 1 || nodes[0].GetID() != "node2" {
		t.Fatalf("expected 1 node (node2), got %v", nodes)
	}
}
