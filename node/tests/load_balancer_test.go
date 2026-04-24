package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"testing"
)

func TestLoadBalancerNode_RoundRobin(t *testing.T) {
	lb := node.NewLoadBalancerNode("lb-1")
	node1 := node.NewBaseNode("dest-1")
	node2 := node.NewBaseNode("dest-2")
	node3 := node.NewBaseNode("dest-3")

	var nodes []node.Node
	nodes = append(nodes, lb, node1, node2, node3)
	defer cleanupNodes(t, nodes)

	// Custom process func to identify which node processed the request
	node1.SetProcessFunc(func(input []byte) ([]byte, error) { return []byte("node1"), nil })
	node2.SetProcessFunc(func(input []byte) ([]byte, error) { return []byte("node2"), nil })
	node3.SetProcessFunc(func(input []byte) ([]byte, error) { return []byte("node3"), nil })

	lb.AddNode(node1)
	lb.AddNode(node2)
	lb.AddNode(node3)

	expectedOrder := []string{"node1", "node2", "node3", "node1", "node2"}

	for i, expected := range expectedOrder {
		res, err := lb.Process([]byte("input"))
		if err != nil {
			t.Fatalf("Step %d: unexpected error %v", i, err)
		}
		if string(res) != expected {
			t.Errorf("Step %d: expected %s, got %s", i, expected, string(res))
		}
	}
}

func TestLoadBalancerNode_Empty(t *testing.T) {
	lb := node.NewLoadBalancerNode("lb-empty")
	defer cleanupNodes(t, []node.Node{lb})

	_, err := lb.Process([]byte("input"))
	if !errors.Is(err, node.ErrNoNodes) {
		t.Errorf("expected ErrNoNodes, got %v", err)
	}
}

func TestLoadBalancerNode_RemoveNode(t *testing.T) {
	lb := node.NewLoadBalancerNode("lb-rm")
	node1 := node.NewBaseNode("dest-1")
	node2 := node.NewBaseNode("dest-2")

	var nodes []node.Node
	nodes = append(nodes, lb, node1, node2)
	defer cleanupNodes(t, nodes)

	lb.AddNode(node1)
	lb.AddNode(node2)

	lb.RemoveNode("dest-1")

	if len(lb.GetNodes()) != 1 {
		t.Fatalf("expected 1 node, got %d", len(lb.GetNodes()))
	}
	if lb.GetNodes()[0].GetID() != "dest-2" {
		t.Errorf("expected remaining node to be dest-2, got %s", lb.GetNodes()[0].GetID())
	}
}

func TestLoadBalancerNode_RemoveNonExistentNode(t *testing.T) {
	lb := node.NewLoadBalancerNode("lb-rm-nonexistent")
	node1 := node.NewBaseNode("dest-1")

	var nodes []node.Node
	nodes = append(nodes, lb, node1)
	defer cleanupNodes(t, nodes)

	lb.AddNode(node1)

	lb.RemoveNode("non-existent-dest")

	if len(lb.GetNodes()) != 1 {
		t.Fatalf("expected 1 node, got %d", len(lb.GetNodes()))
	}
	if lb.GetNodes()[0].GetID() != "dest-1" {
		t.Errorf("expected remaining node to be dest-1, got %s", lb.GetNodes()[0].GetID())
	}
}
