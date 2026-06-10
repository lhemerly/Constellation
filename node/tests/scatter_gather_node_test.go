package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"strings"
	"testing"
	"time"
)

func TestScatterGatherNode_FirstSuccess(t *testing.T) {
	n1 := node.NewBaseNode("t1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(50 * time.Millisecond)
		return []byte("slow"), nil
	})

	n2 := node.NewBaseNode("t2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(10 * time.Millisecond)
		return []byte("fast"), nil
	})

	n3 := node.NewBaseNode("t3")
	n3.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("fail")
	})

	sg := node.NewScatterGatherNode("sg", []node.Node{n1, n2, n3}, 100*time.Millisecond)
	if err := sg.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	defer sg.Delete()

	res, err := sg.Process([]byte("input"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(res) != "fast" {
		t.Errorf("expected 'fast', got '%s'", string(res))
	}
}

func TestScatterGatherNode_AllFail(t *testing.T) {
	n1 := node.NewBaseNode("t1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("fail1")
	})

	n2 := node.NewBaseNode("t2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("fail2")
	})

	sg := node.NewScatterGatherNode("sg", []node.Node{n1, n2}, 100*time.Millisecond)
	if err := sg.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	defer sg.Delete()

	_, err := sg.Process([]byte("input"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "all targets failed") {
		t.Errorf("unexpected error message: %v", err)
	}
	if !strings.Contains(errStr, "fail1") || !strings.Contains(errStr, "fail2") {
		t.Errorf("expected error to contain target errors, got: %v", err)
	}
}

func TestScatterGatherNode_Timeout(t *testing.T) {
	n1 := node.NewBaseNode("t1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(100 * time.Millisecond)
		return []byte("slow"), nil
	})

	sg := node.NewScatterGatherNode("sg", []node.Node{n1}, 20*time.Millisecond)
	if err := sg.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	defer sg.Delete()

	_, err := sg.Process([]byte("input"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !errors.Is(err, node.ErrProcessTimeout) {
		t.Errorf("expected ErrProcessTimeout, got %v", err)
	}
}
