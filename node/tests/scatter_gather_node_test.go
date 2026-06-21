package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"strings"
	"testing"
	"time"
)

func TestScatterGatherNode_Success(t *testing.T) {
	t1 := node.NewBaseNode("target1")
	t1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("t1:"), input...), nil
	})

	t2 := node.NewBaseNode("target2")
	t2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("t2:"), input...), nil
	})

	sgNode := node.NewScatterGatherNode("sg1", []node.Node{t1, t2}, 0)
	defer cleanupNodes(t, []node.Node{t1, t2, sgNode})

	if err := sgNode.Create(); err != nil {
		t.Fatalf("unexpected error during Create: %v", err)
	}

	res, err := sgNode.Process([]byte("data"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resStr := string(res)
	if !strings.Contains(resStr, "t1:data") || !strings.Contains(resStr, "t2:data") {
		t.Errorf("expected result to contain 't1:data' and 't2:data', got '%s'", resStr)
	}
}

func TestScatterGatherNode_PartialFailure(t *testing.T) {
	t1 := node.NewBaseNode("target1")
	t1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("t1:"), input...), nil
	})

	t2 := node.NewBaseNode("target2")
	t2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("simulated error")
	})

	sgNode := node.NewScatterGatherNode("sg1", []node.Node{t1, t2}, 0)
	defer cleanupNodes(t, []node.Node{t1, t2, sgNode})

	if err := sgNode.Create(); err != nil {
		t.Fatalf("unexpected error during Create: %v", err)
	}

	res, err := sgNode.Process([]byte("data"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resStr := string(res)
	if resStr != "t1:data" {
		t.Errorf("expected result to contain only 't1:data', got '%s'", resStr)
	}
}

func TestScatterGatherNode_AllFailure(t *testing.T) {
	t1 := node.NewBaseNode("target1")
	t1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("simulated error 1")
	})

	t2 := node.NewBaseNode("target2")
	t2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("simulated error 2")
	})

	sgNode := node.NewScatterGatherNode("sg1", []node.Node{t1, t2}, 0)
	defer cleanupNodes(t, []node.Node{t1, t2, sgNode})

	if err := sgNode.Create(); err != nil {
		t.Fatalf("unexpected error during Create: %v", err)
	}

	_, err := sgNode.Process([]byte("data"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "all targets failed") {
		t.Errorf("expected error to mention 'all targets failed', got '%v'", err)
	}
}

func TestScatterGatherNode_Timeout(t *testing.T) {
	t1 := node.NewBaseNode("target1")
	t1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("t1:"), input...), nil
	})

	t2 := node.NewBaseNode("target2")
	t2.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(100 * time.Millisecond) // slow target
		return append([]byte("t2:"), input...), nil
	})

	// Timeout faster than t2, slower than t1
	sgNode := node.NewScatterGatherNode("sg1", []node.Node{t1, t2}, 50*time.Millisecond)
	defer cleanupNodes(t, []node.Node{t1, t2, sgNode})

	if err := sgNode.Create(); err != nil {
		t.Fatalf("unexpected error during Create: %v", err)
	}

	_, err := sgNode.Process([]byte("data"))
	if !errors.Is(err, node.ErrProcessTimeout) {
		t.Fatalf("expected ErrProcessTimeout, got %v", err)
	}
}
