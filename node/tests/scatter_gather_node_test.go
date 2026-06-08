package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"strings"
	"testing"
	"time"
)

func TestScatterGatherNode_Success(t *testing.T) {
	sg := node.NewScatterGatherNode("sg-1", 100*time.Millisecond)
	defer cleanupNodes(t, []node.Node{sg})

	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("A"), nil
	})

	n2 := node.NewBaseNode("n2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("B"), nil
	})

	sg.AddDestination(n1)
	sg.AddDestination(n2)

	res, err := sg.Process([]byte("input"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultStr := string(res)
	if !strings.Contains(resultStr, "A") || !strings.Contains(resultStr, "B") {
		t.Errorf("expected result to contain A and B, got %s", resultStr)
	}
	if len(resultStr) != 2 {
		t.Errorf("expected length 2, got %d", len(resultStr))
	}
}

func TestScatterGatherNode_PartialTimeout(t *testing.T) {
	sg := node.NewScatterGatherNode("sg-2", 50*time.Millisecond)
	defer cleanupNodes(t, []node.Node{sg})

	n1 := node.NewBaseNode("fast")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("F"), nil
	})

	n2 := node.NewBaseNode("slow")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(100 * time.Millisecond)
		return []byte("S"), nil
	})

	sg.AddDestination(n1)
	sg.AddDestination(n2)

	res, err := sg.Process([]byte("input"))
	if err != nil {
		t.Fatalf("unexpected error on partial success: %v", err)
	}

	if string(res) != "F" {
		t.Errorf("expected result to be F, got %s", string(res))
	}
}

func TestScatterGatherNode_CompleteTimeout(t *testing.T) {
	sg := node.NewScatterGatherNode("sg-3", 50*time.Millisecond)
	defer cleanupNodes(t, []node.Node{sg})

	n1 := node.NewBaseNode("slow1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(100 * time.Millisecond)
		return []byte("S1"), nil
	})

	n2 := node.NewBaseNode("slow2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(100 * time.Millisecond)
		return []byte("S2"), nil
	})

	sg.AddDestination(n1)
	sg.AddDestination(n2)

	_, err := sg.Process([]byte("input"))
	if err == nil {
		t.Fatalf("expected error on complete failure, got nil")
	}

	if !strings.Contains(err.Error(), "scatter-gather failed completely") {
		t.Errorf("expected error to mention total failure, got %v", err)
	}
}

func TestScatterGatherNode_EmptyDestinations(t *testing.T) {
	sg := node.NewScatterGatherNode("sg-empty", 100*time.Millisecond)
	defer cleanupNodes(t, []node.Node{sg})

	res, err := sg.Process([]byte("input"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != nil {
		t.Errorf("expected nil result for empty destinations, got %v", res)
	}
}

func TestScatterGatherNode_NodeError(t *testing.T) {
	sg := node.NewScatterGatherNode("sg-err", 100*time.Millisecond)
	defer cleanupNodes(t, []node.Node{sg})

	n1 := node.NewBaseNode("success")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("OK"), nil
	})

	n2 := node.NewBaseNode("fail")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("expected failure")
	})

	sg.AddDestination(n1)
	sg.AddDestination(n2)

	res, err := sg.Process([]byte("input"))
	if err != nil {
		t.Fatalf("unexpected error on partial success: %v", err)
	}

	if string(res) != "OK" {
		t.Errorf("expected OK, got %s", string(res))
	}
}
