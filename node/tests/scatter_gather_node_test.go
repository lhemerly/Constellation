package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"strings"
	"testing"
	"time"
)

func TestScatterGatherNode_Basic(t *testing.T) {
	sg := node.NewScatterGatherNode("sg-1", 0)
	defer cleanupNodes(t, []node.Node{sg})

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

	res, err := sg.Process([]byte("input"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	strRes := string(res)
	if strRes != "AB" {
		t.Errorf("expected 'AB', got '%s'", strRes)
	}
}

func TestScatterGatherNode_Error(t *testing.T) {
	sg := node.NewScatterGatherNode("sg-err", 0)
	defer cleanupNodes(t, []node.Node{sg})

	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("error A")
	})

	n2 := node.NewBaseNode("n2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("error B")
	})

	sg.AddNode(n1)
	sg.AddNode(n2)

	_, err := sg.Process([]byte("input"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "error A") || !strings.Contains(errStr, "error B") {
		t.Errorf("expected joined errors, got: %v", err)
	}
}

func TestScatterGatherNode_Timeout(t *testing.T) {
	sg := node.NewScatterGatherNode("sg-timeout", 50*time.Millisecond)
	defer cleanupNodes(t, []node.Node{sg})

	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(100 * time.Millisecond)
		return []byte("A"), nil
	})

	sg.AddNode(n1)

	_, err := sg.Process([]byte("input"))
	if !errors.Is(err, node.ErrScatterGatherTimeout) {
		t.Errorf("expected ErrScatterGatherTimeout, got %v", err)
	}
}

func TestScatterGatherNode_Empty(t *testing.T) {
	sg := node.NewScatterGatherNode("sg-empty", 0)
	defer cleanupNodes(t, []node.Node{sg})

	_, err := sg.Process([]byte("input"))
	if !errors.Is(err, node.ErrEmptyScatterGather) {
		t.Errorf("expected ErrEmptyScatterGather, got %v", err)
	}
}
