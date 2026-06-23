package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"testing"
	"time"
)

func TestScatterGatherNode_Success(t *testing.T) {
	n1 := node.NewBaseNode("t1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("A"), nil
	})

	n2 := node.NewBaseNode("t2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("B"), nil
	})

	sgn := node.NewScatterGatherNode("sg", 100*time.Millisecond)
	sgn.AddTarget(n1)
	sgn.AddTarget(n2)

	defer cleanupNodes(t, []node.Node{n1, n2, sgn})

	res, err := sgn.Process([]byte("input"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resStr := string(res)
	if resStr != "AB" && resStr != "BA" {
		t.Errorf("expected combined results, got %s", resStr)
	}
}

func TestScatterGatherNode_Timeout(t *testing.T) {
	n1 := node.NewBaseNode("t1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("A"), nil
	})

	n2 := node.NewBaseNode("t2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(100 * time.Millisecond) // Slow node
		return []byte("B"), nil
	})

	// Timeout shorter than the slow node's sleep
	sgn := node.NewScatterGatherNode("sg", 50*time.Millisecond)
	sgn.AddTarget(n1)
	sgn.AddTarget(n2)

	defer cleanupNodes(t, []node.Node{n1, n2, sgn})

	res, err := sgn.Process([]byte("input"))
	if err != nil {
		t.Fatalf("unexpected error on timeout partial success: %v", err)
	}

	if string(res) != "A" {
		t.Errorf("expected only fast node's result, got %s", string(res))
	}
}

func TestScatterGatherNode_AllFail(t *testing.T) {
	n1 := node.NewBaseNode("t1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("err1")
	})

	n2 := node.NewBaseNode("t2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("err2")
	})

	sgn := node.NewScatterGatherNode("sg", 100*time.Millisecond)
	sgn.AddTarget(n1)
	sgn.AddTarget(n2)

	defer cleanupNodes(t, []node.Node{n1, n2, sgn})

	_, err := sgn.Process([]byte("input"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestScatterGatherNode_NoTargets(t *testing.T) {
	sgn := node.NewScatterGatherNode("sg", 100*time.Millisecond)
	defer cleanupNodes(t, []node.Node{sgn})

	_, err := sgn.Process([]byte("input"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestScatterGatherNode_RemoveTarget(t *testing.T) {
	n1 := node.NewBaseNode("t1")
	sgn := node.NewScatterGatherNode("sg", 100*time.Millisecond)
	sgn.AddTarget(n1)
	sgn.RemoveTarget("t1")
	defer cleanupNodes(t, []node.Node{n1, sgn})

	_, err := sgn.Process([]byte("input"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
