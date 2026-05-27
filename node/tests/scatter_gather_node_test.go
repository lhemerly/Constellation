package node_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestScatterGatherNode_Success(t *testing.T) {
	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-n1")...), nil
	})

	n2 := node.NewBaseNode("n2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-n2")...), nil
	})

	sgNode := node.NewScatterGatherNode("sg-node", []node.Node{n1, n2}, 2*time.Second)
	sgNode.Create()
	defer cleanupNodes(t, []node.Node{sgNode})

	res, err := sgNode.Process([]byte("init"))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	resStr := string(res)
	if !strings.Contains(resStr, "init-n1") || !strings.Contains(resStr, "init-n2") {
		t.Errorf("Expected result to contain both responses, got %s", resStr)
	}
}

func TestScatterGatherNode_PartialFailure(t *testing.T) {
	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-n1")...), nil
	})

	n2 := node.NewBaseNode("n2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("n2 failed")
	})

	sgNode := node.NewScatterGatherNode("sg-node", []node.Node{n1, n2}, 2*time.Second)
	sgNode.Create()
	defer cleanupNodes(t, []node.Node{sgNode})

	res, err := sgNode.Process([]byte("init"))
	if err != nil {
		t.Fatalf("Expected no error due to partial success, got %v", err)
	}

	resStr := string(res)
	if resStr != "init-n1" {
		t.Errorf("Expected result to be 'init-n1', got '%s'", resStr)
	}
}

func TestScatterGatherNode_Timeout(t *testing.T) {
	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-n1")...), nil
	})

	n2 := node.NewBaseNode("n2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(200 * time.Millisecond) // This should timeout
		return append(input, []byte("-n2")...), nil
	})

	// Timeout is very short
	sgNode := node.NewScatterGatherNode("sg-node", []node.Node{n1, n2}, 50*time.Millisecond)
	sgNode.Create()
	defer cleanupNodes(t, []node.Node{sgNode})

	res, err := sgNode.Process([]byte("init"))
	if err != nil {
		t.Fatalf("Expected no error due to partial success, got %v", err)
	}

	resStr := string(res)
	if resStr != "init-n1" {
		t.Errorf("Expected result to only contain fast node response, got '%s'", resStr)
	}
}

func TestScatterGatherNode_AllFail(t *testing.T) {
	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("n1 fail")
	})

	sgNode := node.NewScatterGatherNode("sg-node", []node.Node{n1}, 2*time.Second)
	sgNode.Create()
	defer cleanupNodes(t, []node.Node{sgNode})

	_, err := sgNode.Process([]byte("init"))
	if err == nil {
		t.Fatalf("Expected error when all nodes fail, got nil")
	}
}
