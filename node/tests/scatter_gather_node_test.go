package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"strings"
	"testing"
)

func TestScatterGatherNode_BasicProcess(t *testing.T) {
	n1 := node.NewBaseNode("child-1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("c1:" + string(input)), nil
	})

	n2 := node.NewBaseNode("child-2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("c2:" + string(input)), nil
	})

	sg, err := node.NewScatterGatherNode("sg-node", []node.Node{n1, n2})
	if err != nil {
		t.Fatalf("unexpected error creating ScatterGatherNode: %v", err)
	}
	defer cleanupNodes(t, []node.Node{sg})

	if err := sg.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	res, err := sg.Process([]byte("-data-"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resStr := string(res)
	if !strings.Contains(resStr, "c1:-data-") || !strings.Contains(resStr, "c2:-data-") {
		t.Errorf("expected result to contain data from both children, got '%s'", resStr)
	}
	if len(resStr) != len("c1:-data-")+len("c2:-data-") {
		t.Errorf("unexpected result length, got '%s'", resStr)
	}
}

func TestScatterGatherNode_ChildError(t *testing.T) {
	n1 := node.NewBaseNode("child-1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("c1"), nil
	})

	n2 := node.NewBaseNode("child-2")
	expectedErr := errors.New("child error")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, expectedErr
	})

	sg, err := node.NewScatterGatherNode("sg-error", []node.Node{n1, n2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanupNodes(t, []node.Node{sg})

	if err := sg.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	_, err = sg.Process([]byte("input"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error to wrap '%v', got '%v'", expectedErr, err)
	}
}

func TestScatterGatherNode_ZeroChildrenError(t *testing.T) {
	_, err := node.NewScatterGatherNode("sg-zero", []node.Node{})
	if err == nil {
		t.Fatal("expected error when creating ScatterGatherNode with 0 children")
	}
}
