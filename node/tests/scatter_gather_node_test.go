package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"strings"
	"testing"
	"time"
)

func TestScatterGatherNode_Success(t *testing.T) {
	sg := node.NewScatterGatherNode("sg-success", 1*time.Second)

	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("res1"), nil
	})

	n2 := node.NewBaseNode("n2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("res2"), nil
	})

	sg.AddTarget(n1)
	sg.AddTarget(n2)

	defer cleanupNodes(t, []node.Node{sg, n1, n2})

	res, err := sg.Process([]byte("input"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resStr := string(res)
	if !strings.Contains(resStr, "res1") || !strings.Contains(resStr, "res2") {
		t.Errorf("expected result to contain 'res1' and 'res2', got %s", resStr)
	}
}

func TestScatterGatherNode_PartialFailure(t *testing.T) {
	sg := node.NewScatterGatherNode("sg-fail", 1*time.Second)

	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("error1")
	})

	n2 := node.NewBaseNode("n2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("res2"), nil
	})

	sg.AddTarget(n1)
	sg.AddTarget(n2)

	defer cleanupNodes(t, []node.Node{sg, n1, n2})

	_, err := sg.Process([]byte("input"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "error1") {
		t.Errorf("expected error to contain 'error1', got %v", err)
	}
}

func TestScatterGatherNode_Timeout(t *testing.T) {
	sg := node.NewScatterGatherNode("sg-timeout", 50*time.Millisecond)

	n1 := node.NewBaseNode("n1")
	blockCh := make(chan struct{})
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		<-blockCh
		return []byte("res1"), nil
	})

	n2 := node.NewBaseNode("n2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("res2"), nil
	})

	sg.AddTarget(n1)
	sg.AddTarget(n2)

	defer cleanupNodes(t, []node.Node{sg, n1, n2})

	_, err := sg.Process([]byte("input"))
	if err == nil {
		t.Fatalf("expected timeout error, got nil")
	}

	if !errors.Is(err, node.ErrScatterGatherTimeout) {
		t.Errorf("expected ErrScatterGatherTimeout, got %v", err)
	}

	close(blockCh) // unblock n1 so it can be cleaned up
}
