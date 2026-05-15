package node_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/lhemerly/Constellation/node"
)

func TestFanOutNode_NoNodes(t *testing.T) {
	fn := node.NewFanOutNode("fanout-1")
	defer cleanupNodes(t, []*node.FanOutNode{fn})

	_, err := fn.Process([]byte("test"))
	if !errors.Is(err, node.ErrNoFanOutNodes) {
		t.Errorf("expected ErrNoFanOutNodes, got %v", err)
	}
}

func TestFanOutNode_Success(t *testing.T) {
	fn := node.NewFanOutNode("fanout-1")
	n1 := node.NewBaseNode("node-1")
	n2 := node.NewBaseNode("node-2")

	defer cleanupNodes(t, []node.Node{fn, n1, n2})

	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-1")...), nil
	})

	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-2")...), nil
	})

	fn.AddNode(n1)
	fn.AddNode(n2)

	res, err := fn.Process([]byte("test"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resStr := string(res)
	if !strings.Contains(resStr, "test-1") || !strings.Contains(resStr, "test-2") {
		t.Errorf("expected result to contain 'test-1' and 'test-2', got %s", resStr)
	}
}

func TestFanOutNode_PartialError(t *testing.T) {
	fn := node.NewFanOutNode("fanout-1")
	n1 := node.NewBaseNode("node-1")
	n2 := node.NewBaseNode("node-2")

	defer cleanupNodes(t, []node.Node{fn, n1, n2})

	expectedErr := errors.New("node 2 failed")

	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-1")...), nil
	})

	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, expectedErr
	})

	fn.AddNode(n1)
	fn.AddNode(n2)

	res, err := fn.Process([]byte("test"))

	if err == nil {
		t.Fatalf("expected an error, got nil")
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error to wrap '%v', got %v", expectedErr, err)
	}

	resStr := string(res)
	if !strings.Contains(resStr, "test-1") {
		t.Errorf("expected result to contain 'test-1', got %s", resStr)
	}
}
