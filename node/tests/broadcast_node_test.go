package node_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/lhemerly/Constellation/node"
)

func TestBroadcastNode_Success(t *testing.T) {
	bn := node.NewBroadcastNode("broadcast-node")
	if err := bn.Create(); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-n1")...), nil
	})

	n2 := node.NewBaseNode("n2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-n2")...), nil
	})

	bn.AddNode(n1)
	bn.AddNode(n2)

	defer cleanupNodes(t, []node.Node{bn, n1, n2})

	input := []byte("data")
	output, err := bn.Process(input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	// Output is concatenated. Since order is deterministic via indexing, it should be n1 then n2.
	expected := []byte("data-n1data-n2")
	if !bytes.Equal(output, expected) {
		t.Errorf("Process() output = %s, want %s", string(output), string(expected))
	}
}

func TestBroadcastNode_PartialFailure(t *testing.T) {
	bn := node.NewBroadcastNode("broadcast-node")
	if err := bn.Create(); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-n1")...), nil
	})

	n2 := node.NewBaseNode("n2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("n2 failed")
	})

	bn.AddNode(n1)
	bn.AddNode(n2)

	defer cleanupNodes(t, []node.Node{bn, n1, n2})

	input := []byte("data")
	output, err := bn.Process(input)
	if err == nil {
		t.Fatalf("Process() expected error but got none")
	}

	if !strings.Contains(err.Error(), "broadcast partial failure") {
		t.Errorf("Process() error = %v, expected partial failure", err)
	}

	if !strings.Contains(err.Error(), "n2 failed") {
		t.Errorf("Process() error = %v, expected n2 failed", err)
	}

	// It should still return the successful result from n1
	expected := []byte("data-n1")
	if !bytes.Equal(output, expected) {
		t.Errorf("Process() output = %s, want %s", string(output), string(expected))
	}
}

func TestBroadcastNode_NoNodes(t *testing.T) {
	bn := node.NewBroadcastNode("broadcast-node")
	defer bn.Delete()

	_, err := bn.Process([]byte("data"))
	if err != node.ErrNoBroadcastNodes {
		t.Errorf("Expected ErrNoBroadcastNodes, got %v", err)
	}
}
