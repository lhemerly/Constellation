package node_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestScatterGatherNode_Success(t *testing.T) {
	sg := node.NewScatterGatherNode("sg-1")
	defer cleanupNodes(t, []node.Node{sg})

	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(10 * time.Millisecond)
		return []byte("A"), nil
	})

	n2 := node.NewBaseNode("n2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(20 * time.Millisecond)
		return []byte("B"), nil
	})

	sg.AddTarget(n1)
	sg.AddTarget(n2)

	out, err := sg.Process([]byte("input"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultStr := string(out)
	if !strings.Contains(resultStr, "A") || !strings.Contains(resultStr, "B") {
		t.Errorf("expected combined output to contain A and B, got %s", resultStr)
	}
	if len(resultStr) != 2 {
		t.Errorf("expected length 2, got %d", len(resultStr))
	}
}

func TestScatterGatherNode_Failure(t *testing.T) {
	sg := node.NewScatterGatherNode("sg-1")
	defer cleanupNodes(t, []node.Node{sg})

	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("A"), nil
	})

	n2 := node.NewBaseNode("n2")
	expectedErr := errors.New("worker error")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, expectedErr
	})

	sg.AddTarget(n1)
	sg.AddTarget(n2)

	_, err := sg.Process([]byte("input"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), expectedErr.Error()) {
		t.Errorf("expected error to contain %q, got %v", expectedErr.Error(), err)
	}
}

func TestScatterGatherNode_NoTargets(t *testing.T) {
	sg := node.NewScatterGatherNode("sg-1")
	defer cleanupNodes(t, []node.Node{sg})

	_, err := sg.Process([]byte("input"))
	if !errors.Is(err, node.ErrNoTargets) {
		t.Fatalf("expected ErrNoTargets, got %v", err)
	}
}

func TestScatterGatherNode_RemoveTarget(t *testing.T) {
	sg := node.NewScatterGatherNode("sg-1")
	defer cleanupNodes(t, []node.Node{sg})

	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) { return []byte("A"), nil })

	sg.AddTarget(n1)
	sg.RemoveTarget("n1")

	_, err := sg.Process([]byte("input"))
	if !errors.Is(err, node.ErrNoTargets) {
		t.Fatalf("expected ErrNoTargets, got %v", err)
	}
}
