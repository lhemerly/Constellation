package node_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/lhemerly/Constellation/node"
)

func TestFanOutNode_Creation(t *testing.T) {
	// Empty children should fail
	_, err := node.NewFanOutNode("empty-fanout", nil)
	if !errors.Is(err, node.ErrNoNodesFanOut) {
		t.Fatalf("expected ErrNoNodesFanOut, got %v", err)
	}

	// Valid creation
	child := node.NewBaseNode("child1")
	fNode, err := node.NewFanOutNode("valid-fanout", []node.Node{child})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if fNode.GetID() != "valid-fanout" {
		t.Errorf("expected ID 'valid-fanout', got %s", fNode.GetID())
	}
}

func TestFanOutNode_Success(t *testing.T) {
	child1 := node.NewBaseNode("child1")
	child1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("A:"), input...), nil
	})

	child2 := node.NewBaseNode("child2")
	child2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("B:"), input...), nil
	})

	fNode, err := node.NewFanOutNode("fanout-node", []node.Node{child1, child2})
	if err != nil {
		t.Fatalf("expected no error creating FanOutNode: %v", err)
	}

	defer cleanupNodes(t, []node.Node{fNode, child1, child2})

	res, err := fNode.Process([]byte("test"))
	if err != nil {
		t.Fatalf("expected no error processing, got %v", err)
	}

	resStr := string(res)
	if resStr != "A:testB:test" {
		t.Errorf("expected 'A:testB:test', got '%s'", resStr)
	}
}

func TestFanOutNode_Failure(t *testing.T) {
	child1 := node.NewBaseNode("child1")
	child1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("A:"), input...), nil
	})

	child2 := node.NewBaseNode("child2")
	child2Err := errors.New("child2 failed")
	child2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, child2Err
	})

	fNode, err := node.NewFanOutNode("fanout-node-fail", []node.Node{child1, child2})
	if err != nil {
		t.Fatalf("expected no error creating FanOutNode: %v", err)
	}

	defer cleanupNodes(t, []node.Node{fNode, child1, child2})

	_, err = fNode.Process([]byte("test"))
	if err == nil {
		t.Fatalf("expected error from FanOutNode")
	}

	if !errors.Is(err, node.ErrFanOutFailed) {
		t.Errorf("expected error to wrap ErrFanOutFailed, got %v", err)
	}

	if !strings.Contains(err.Error(), "child2 failed") {
		t.Errorf("expected error message to contain 'child2 failed', got %v", err.Error())
	}
}
