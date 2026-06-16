package node_test

import (
	"errors"
	"testing"

	"github.com/lhemerly/Constellation/node"
)

func TestConsensusNode_QuorumReached(t *testing.T) {
	n1 := node.NewBaseNode("node1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) { return []byte("resultA"), nil })

	n2 := node.NewBaseNode("node2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) { return []byte("resultA"), nil })

	n3 := node.NewBaseNode("node3")
	n3.SetProcessFunc(func(input []byte) ([]byte, error) { return []byte("resultB"), nil }) // Outlier

	cn := node.NewConsensusNode("consensus", []node.Node{n1, n2, n3})

	res, err := cn.Process([]byte("test"))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if string(res) != "resultA" {
		t.Fatalf("Expected 'resultA', got: %s", string(res))
	}
}

func TestConsensusNode_QuorumWithErrors(t *testing.T) {
	n1 := node.NewBaseNode("node1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) { return []byte("resultA"), nil })

	n2 := node.NewBaseNode("node2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) { return []byte("resultA"), nil })

	n3 := node.NewBaseNode("node3")
	n3.SetProcessFunc(func(input []byte) ([]byte, error) { return nil, errors.New("failed") }) // Error

	cn := node.NewConsensusNode("consensus", []node.Node{n1, n2, n3})

	res, err := cn.Process([]byte("test"))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if string(res) != "resultA" {
		t.Fatalf("Expected 'resultA', got: %s", string(res))
	}
}

func TestConsensusNode_NoQuorum(t *testing.T) {
	n1 := node.NewBaseNode("node1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) { return []byte("resultA"), nil })

	n2 := node.NewBaseNode("node2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) { return []byte("resultB"), nil })

	n3 := node.NewBaseNode("node3")
	n3.SetProcessFunc(func(input []byte) ([]byte, error) { return []byte("resultC"), nil })

	cn := node.NewConsensusNode("consensus", []node.Node{n1, n2, n3})

	_, err := cn.Process([]byte("test"))
	if err == nil {
		t.Fatal("Expected error due to no quorum, got nil")
	}

	if err != node.ErrConsensusFailed {
		t.Fatalf("Expected ErrConsensusFailed, got: %v", err)
	}
}

func TestConsensusNode_AllErrors(t *testing.T) {
	n1 := node.NewBaseNode("node1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) { return nil, errors.New("failed") })

	cn := node.NewConsensusNode("consensus", []node.Node{n1})

	_, err := cn.Process([]byte("test"))
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}

func TestConsensusNode_CreateDelete(t *testing.T) {
	n1 := node.NewBaseNode("node1")
	n2 := node.NewBaseNode("node2")

	cn := node.NewConsensusNode("consensus", []node.Node{n1, n2})
	if err := cn.Create(); err != nil {
		t.Fatalf("Expected no error on Create, got: %v", err)
	}
	if err := cn.Delete(); err != nil {
		t.Fatalf("Expected no error on Delete, got: %v", err)
	}
}
