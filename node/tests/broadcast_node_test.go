package node_test

import (
	"github.com/lhemerly/Constellation/node"
	"sync/atomic"
	"testing"
	"time"
)

func TestBroadcastNode_Basic(t *testing.T) {
	bn := node.NewBroadcastNode("bn-1")
	defer cleanupNodes(t, []node.Node{bn})

	var counter1, counter2 int32

	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		atomic.AddInt32(&counter1, 1)
		return nil, nil
	})

	n2 := node.NewBaseNode("n2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		atomic.AddInt32(&counter2, 1)
		return nil, nil
	})

	bn.AddNode(n1)
	bn.AddNode(n2)

	res, err := bn.Process([]byte("input"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(res) != "input" {
		t.Errorf("expected original input 'input', got '%s'", string(res))
	}

	// wait for async broadcast to finish
	time.Sleep(50 * time.Millisecond)

	if atomic.LoadInt32(&counter1) != 1 {
		t.Errorf("expected counter1 to be 1, got %d", counter1)
	}
	if atomic.LoadInt32(&counter2) != 1 {
		t.Errorf("expected counter2 to be 1, got %d", counter2)
	}
}

func TestBroadcastNode_GetNodes(t *testing.T) {
	bn := node.NewBroadcastNode("bn-nodes")
	defer cleanupNodes(t, []node.Node{bn})

	n1 := node.NewBaseNode("n1")
	bn.AddNode(n1)

	nodes := bn.GetNodes()
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].GetID() != "n1" {
		t.Errorf("expected node 'n1', got '%s'", nodes[0].GetID())
	}
}
