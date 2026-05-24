package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"testing"
)

func TestBroadcastNode_Success(t *testing.T) {
	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("n1:"), input...), nil
	})

	n2 := node.NewBaseNode("n2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("n2:"), input...), nil
	})

	bn := node.NewBroadcastNode("bn-1", []node.Node{n1, n2})
	defer cleanupNodes(t, []node.Node{n1, n2, bn})

	res, err := bn.Process([]byte("test"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	strRes := string(res)
	if strRes != "n1:testn2:test" && strRes != "n2:testn1:test" {
		t.Errorf("unexpected combined result: %s", strRes)
	}
}

func TestBroadcastNode_ErrorPropagation(t *testing.T) {
	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("n1 error")
	})

	n2 := node.NewBaseNode("n2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("n2:"), input...), nil
	})

	bn := node.NewBroadcastNode("bn-2", []node.Node{n1, n2})
	defer cleanupNodes(t, []node.Node{n1, n2, bn})

	_, err := bn.Process([]byte("test"))
	if err == nil {
		t.Fatalf("expected error from broadcast node")
	}

	if err.Error() != "n1 error" {
		t.Errorf("expected joined error containing 'n1 error', got: %v", err)
	}
}

func TestBroadcastNode_PanicOnEmptyTargets(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on empty targets")
		}
	}()

	_ = node.NewBroadcastNode("bn-panic", []node.Node{})
}
