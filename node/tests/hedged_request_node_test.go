package node_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestHedgedRequestNode_SuccessFastest(t *testing.T) {
	fastNode := node.NewBaseNode("fast-node")
	fastNode.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("fast response"), nil
	})

	slowNode := node.NewBaseNode("slow-node")
	slowNode.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(100 * time.Millisecond)
		return []byte("slow response"), nil
	})

	hNode := node.NewHedgedRequestNode("hedged-node", []node.Node{slowNode, fastNode})

	if err := hNode.Create(); err != nil {
		t.Fatalf("failed to create HedgedRequestNode: %v", err)
	}
	defer cleanupNodes(t, []node.Node{hNode})

	res, err := hNode.Process([]byte("input"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(res) != "fast response" {
		t.Errorf("expected 'fast response', got '%s'", res)
	}
}

func TestHedgedRequestNode_AllFail(t *testing.T) {
	node1 := node.NewBaseNode("fail-node-1")
	node1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("error 1")
	})

	node2 := node.NewBaseNode("fail-node-2")
	node2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("error 2")
	})

	hNode := node.NewHedgedRequestNode("hedged-node", []node.Node{node1, node2})

	if err := hNode.Create(); err != nil {
		t.Fatalf("failed to create HedgedRequestNode: %v", err)
	}
	defer cleanupNodes(t, []node.Node{hNode})

	_, err := hNode.Process([]byte("input"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "all hedged requests failed") {
		t.Errorf("expected error to contain 'all hedged requests failed', got %v", err)
	}
}
