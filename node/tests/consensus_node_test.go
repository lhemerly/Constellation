package node_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestConsensusNode_Process_Success(t *testing.T) {
	cn := node.NewConsensusNode("consensus1")
	defer cleanupNodes(t, []node.Node{cn})

	node1 := node.NewBaseNode("node1")
	node1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("A"), nil
	})
	node2 := node.NewBaseNode("node2")
	node2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("A"), nil
	})
	node3 := node.NewBaseNode("node3")
	node3.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("B"), nil
	})

	cn.AddNode(node1)
	cn.AddNode(node2)
	cn.AddNode(node3)

	result, err := cn.Process([]byte("test"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !reflect.DeepEqual(result, []byte("A")) {
		t.Fatalf("expected result 'A', got %s", result)
	}
}

func TestConsensusNode_Process_NoConsensus(t *testing.T) {
	cn := node.NewConsensusNode("consensus1")
	defer cleanupNodes(t, []node.Node{cn})

	node1 := node.NewBaseNode("node1")
	node1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("A"), nil
	})
	node2 := node.NewBaseNode("node2")
	node2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("B"), nil
	})
	node3 := node.NewBaseNode("node3")
	node3.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("C"), nil
	})

	cn.AddNode(node1)
	cn.AddNode(node2)
	cn.AddNode(node3)

	_, err := cn.Process([]byte("test"))
	if !errors.Is(err, node.ErrNoConsensus) {
		t.Fatalf("expected ErrNoConsensus, got %v", err)
	}
}

func TestConsensusNode_Process_AllNodesFailed(t *testing.T) {
	cn := node.NewConsensusNode("consensus1")
	defer cleanupNodes(t, []node.Node{cn})

	node1 := node.NewBaseNode("node1")
	node1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("error A")
	})
	node2 := node.NewBaseNode("node2")
	node2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("error B")
	})

	cn.AddNode(node1)
	cn.AddNode(node2)

	_, err := cn.Process([]byte("test"))
	if !errors.Is(err, node.ErrAllNodesFailed) {
		t.Fatalf("expected ErrAllNodesFailed, got %v", err)
	}
}

func TestConsensusNode_Process_NoNodes(t *testing.T) {
	cn := node.NewConsensusNode("consensus1")
	defer cleanupNodes(t, []node.Node{cn})

	_, err := cn.Process([]byte("test"))
	if !errors.Is(err, node.ErrNoNodes) {
		t.Fatalf("expected ErrNoNodes, got %v", err)
	}
}

func TestConsensusNode_RemoveNode(t *testing.T) {
	cn := node.NewConsensusNode("consensus1")
	defer cleanupNodes(t, []node.Node{cn})

	node1 := node.NewBaseNode("node1")
	cn.AddNode(node1)

	if len(cn.GetNodes()) != 1 {
		t.Fatalf("expected 1 node, got %d", len(cn.GetNodes()))
	}

	cn.RemoveNode("node1")

	if len(cn.GetNodes()) != 0 {
		t.Fatalf("expected 0 nodes after removal, got %d", len(cn.GetNodes()))
	}
}

func TestConsensusNode_TimeoutGracefully(t *testing.T) {
	cn := node.NewConsensusNode("consensus1")
	defer cleanupNodes(t, []node.Node{cn})

	node1 := node.NewBaseNode("node1")
	node1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("A"), nil
	})
	node2 := node.NewBaseNode("node2")
	node2.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(50 * time.Millisecond) // Simulating slow processing
		return []byte("B"), nil
	})
	node3 := node.NewBaseNode("node3")
	node3.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("A"), nil
	})

	cn.AddNode(node1)
	cn.AddNode(node2)
	cn.AddNode(node3)

	result, err := cn.Process([]byte("test"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !reflect.DeepEqual(result, []byte("A")) {
		t.Fatalf("expected result 'A', got %s", result)
	}
}
