package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"testing"
	"time"
)

func TestScatterGatherNode_Basic(t *testing.T) {
	sg := node.NewScatterGatherNode("sg-1", 1*time.Second)
	defer cleanupNodes(t, []node.Node{sg})

	if err := sg.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("A"), nil
	})

	n2 := node.NewBaseNode("n2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("B"), nil
	})

	sg.AddNode(n1)
	sg.AddNode(n2)

	res, err := sg.Process([]byte("data"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := string(res)
	if s != "AB" && s != "BA" {
		t.Errorf("expected 'AB' or 'BA', got %s", s)
	}
}

func TestScatterGatherNode_Timeout(t *testing.T) {
	sg := node.NewScatterGatherNode("sg-2", 50*time.Millisecond)
	defer cleanupNodes(t, []node.Node{sg})

	if err := sg.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(100 * time.Millisecond)
		return []byte("A"), nil
	})

	sg.AddNode(n1)

	_, err := sg.Process([]byte("data"))
	if !errors.Is(err, node.ErrScatterGatherTimeout) {
		t.Fatalf("expected ErrScatterGatherTimeout, got %v", err)
	}
}

func TestScatterGatherNode_Failed(t *testing.T) {
	sg := node.NewScatterGatherNode("sg-3", 1*time.Second)
	defer cleanupNodes(t, []node.Node{sg})

	if err := sg.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("failed")
	})

	sg.AddNode(n1)

	_, err := sg.Process([]byte("data"))
	if !errors.Is(err, node.ErrScatterGatherFailed) {
		t.Fatalf("expected ErrScatterGatherFailed, got %v", err)
	}
}
