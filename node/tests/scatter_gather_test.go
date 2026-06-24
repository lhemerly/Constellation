package node_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestScatterGatherNode_Success(t *testing.T) {
	sgn := node.NewScatterGatherNode("sg-success", 500*time.Millisecond)
	defer cleanupNodes(t, []node.Node{sgn})

	target1 := node.NewBaseNode("t1")
	target1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("t1-"), input...), nil
	})
	defer cleanupNodes(t, []node.Node{target1})

	target2 := node.NewBaseNode("t2")
	target2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("t2-"), input...), nil
	})
	defer cleanupNodes(t, []node.Node{target2})

	sgn.AddTarget(target1)
	sgn.AddTarget(target2)

	res, err := sgn.Process([]byte("data"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resStr := string(res)
	if !strings.Contains(resStr, "t1-data") || !strings.Contains(resStr, "t2-data") {
		t.Errorf("expected results from both targets, got %s", resStr)
	}
}

func TestScatterGatherNode_PartialTimeout(t *testing.T) {
	sgn := node.NewScatterGatherNode("sg-timeout", 100*time.Millisecond)
	defer cleanupNodes(t, []node.Node{sgn})

	target1 := node.NewBaseNode("t1")
	target1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("t1-"), input...), nil
	})
	defer cleanupNodes(t, []node.Node{target1})

	target2 := node.NewBaseNode("t2")
	target2.SetProcessFunc(func(input []byte) ([]byte, error) {
		// Use a signal channel instead of sleep to avoid blocking Delete()
		blockCh := make(chan struct{})
		// Do not wait forever, wait more than timeout but less than test total timeout to allow clean up
		select {
		case <-time.After(300 * time.Millisecond):
		case <-blockCh:
		}
		return append([]byte("t2-"), input...), nil
	})
	defer cleanupNodes(t, []node.Node{target2})

	sgn.AddTarget(target1)
	sgn.AddTarget(target2)

	res, err := sgn.Process([]byte("data"))

	if err == nil {
		t.Fatalf("expected error from timeout, got nil")
	}

	if !errors.Is(err, node.ErrScatterGatherTimeout) {
		t.Errorf("expected ErrScatterGatherTimeout, got %v", err)
	}

	resStr := string(res)
	if !strings.Contains(resStr, "t1-data") {
		t.Errorf("expected partial result from t1, got %s", resStr)
	}
}

func TestScatterGatherNode_AllErrors(t *testing.T) {
	sgn := node.NewScatterGatherNode("sg-errors", 500*time.Millisecond)
	defer cleanupNodes(t, []node.Node{sgn})

	target1 := node.NewBaseNode("t1")
	target1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("err1")
	})
	defer cleanupNodes(t, []node.Node{target1})

	target2 := node.NewBaseNode("t2")
	target2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("err2")
	})
	defer cleanupNodes(t, []node.Node{target2})

	sgn.AddTarget(target1)
	sgn.AddTarget(target2)

	_, err := sgn.Process([]byte("data"))
	if err == nil {
		t.Fatalf("expected combined errors, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "err1") || !strings.Contains(errStr, "err2") {
		t.Errorf("expected error to contain err1 and err2, got %s", errStr)
	}
}
