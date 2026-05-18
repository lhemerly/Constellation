package node_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/lhemerly/Constellation/node"
)

func TestScatterGatherNode_Success(t *testing.T) {
	node1 := node.NewBaseNode("t1")
	node1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("1:"), input...), nil
	})

	node2 := node.NewBaseNode("t2")
	node2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("2:"), input...), nil
	})

	node3 := node.NewBaseNode("t3")
	node3.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("3:"), input...), nil
	})

	sgn := node.NewScatterGatherNode("sgn", []node.Node{node1, node2, node3})
	defer cleanupNodes(t, []node.Node{node1, node2, node3, sgn})

	res, err := sgn.Process([]byte("data"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resStr := string(res)

	// Since results are gathered via channels which don't guarantee order of completion,
	// we check if all expected strings are present and the total length is correct.
	if !strings.Contains(resStr, "1:data") || !strings.Contains(resStr, "2:data") || !strings.Contains(resStr, "3:data") {
		t.Errorf("missing expected parts in result: %s", resStr)
	}
	if len(resStr) != len("1:data")+len("2:data")+len("3:data") {
		t.Errorf("unexpected length of result: %d", len(resStr))
	}
}

func TestScatterGatherNode_PartialFailure(t *testing.T) {
	node1 := node.NewBaseNode("t1")
	node1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("1:"), input...), nil
	})

	nodeFail := node.NewBaseNode("fail")
	expectedErr := errors.New("target failed")
	nodeFail.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, expectedErr
	})

	sgn := node.NewScatterGatherNode("sgn", []node.Node{node1, nodeFail})
	defer cleanupNodes(t, []node.Node{node1, nodeFail, sgn})

	_, err := sgn.Process([]byte("data"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error to wrap target error. got: %v", err)
	}
}

func TestScatterGatherNode_PanicOnEmpty(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on empty targets")
		}
	}()

	node.NewScatterGatherNode("sgn", []node.Node{})
}
