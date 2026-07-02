package node_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestScatterGatherNode_Success(t *testing.T) {
	target1 := node.NewBaseNode("target1")
	target1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("T1:"), input...), nil
	})

	target2 := node.NewBaseNode("target2")
	target2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("T2:"), input...), nil
	})

	targets := []node.Node{target1, target2}
	sg := node.NewScatterGatherNode("sg", targets, 0)
	defer cleanupNodes(t, append(targets, sg))

	if err := sg.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	res, err := sg.Process([]byte("data"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resStr := string(res)
	if !strings.Contains(resStr, "T1:data") || !strings.Contains(resStr, "T2:data") {
		t.Errorf("unexpected result: %s", resStr)
	}
}

func TestScatterGatherNode_PartialFailure(t *testing.T) {
	target1 := node.NewBaseNode("target1")
	target1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("T1:"), input...), nil
	})

	target2 := node.NewBaseNode("target2")
	expectedErr := errors.New("target2 failed")
	target2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, expectedErr
	})

	targets := []node.Node{target1, target2}
	sg := node.NewScatterGatherNode("sg", targets, 0)
	defer cleanupNodes(t, append(targets, sg))

	if err := sg.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	_, err := sg.Process([]byte("data"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "scatter-gather failed") || !strings.Contains(err.Error(), "target2 failed") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestScatterGatherNode_Timeout(t *testing.T) {
	target1 := node.NewBaseNode("target1")
	target1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("T1:done"), nil
	})

	target2 := node.NewBaseNode("target2")
	target2.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(100 * time.Millisecond) // slow process
		return []byte("T2:done"), nil
	})

	targets := []node.Node{target1, target2}
	// 50ms timeout
	sg := node.NewScatterGatherNode("sg", targets, 50*time.Millisecond)
	defer cleanupNodes(t, append(targets, sg))

	if err := sg.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	_, err := sg.Process([]byte("data"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("expected timeout error, got %v", err)
	}
}

func TestScatterGatherNode_PanicOnEmptyTargets(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on empty targets")
		}
	}()
	node.NewScatterGatherNode("sg", []node.Node{}, 0)
}
