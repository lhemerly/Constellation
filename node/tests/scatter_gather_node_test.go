package node_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/lhemerly/Constellation/node"
)

func TestScatterGatherNode_Success(t *testing.T) {
	target1 := node.NewBaseNode("target1")
	target1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-t1")...), nil
	})

	target2 := node.NewBaseNode("target2")
	target2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-t2")...), nil
	})

	gatherFunc := func(results [][]byte, errs []error) ([]byte, error) {
		var resStrs []string
		for i, err := range errs {
			if err != nil {
				return nil, err
			}
			resStrs = append(resStrs, string(results[i]))
		}
		return []byte(strings.Join(resStrs, "|")), nil
	}

	targets := []node.Node{target1, target2}
	sgNode := node.NewScatterGatherNode("sg1", targets, gatherFunc)

	defer cleanupNodes(t, append(targets, sgNode))

	res, err := sgNode.Process([]byte("req"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "req-t1|req-t2"
	if string(res) != expected {
		t.Errorf("expected %s, got %s", expected, string(res))
	}
}

func TestScatterGatherNode_Error(t *testing.T) {
	target1 := node.NewBaseNode("target1")
	target1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-t1")...), nil
	})

	target2 := node.NewBaseNode("target2")
	target2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("target2 failed")
	})

	gatherFunc := func(results [][]byte, errs []error) ([]byte, error) {
		var errList []error
		for _, err := range errs {
			if err != nil {
				errList = append(errList, err)
			}
		}
		if len(errList) > 0 {
			return nil, errors.Join(errList...)
		}
		return []byte("success"), nil
	}

	targets := []node.Node{target1, target2}
	sgNode := node.NewScatterGatherNode("sg2", targets, gatherFunc)

	defer cleanupNodes(t, append(targets, sgNode))

	_, err := sgNode.Process([]byte("req"))
	if err == nil {
		t.Fatalf("expected error from scatter-gather, got nil")
	}

	if !strings.Contains(err.Error(), "target2 failed") {
		t.Errorf("expected error to contain 'target2 failed', got %v", err)
	}
}

func TestScatterGatherNode_EmptyTargets(t *testing.T) {
	gatherFunc := func(results [][]byte, errs []error) ([]byte, error) {
		return []byte("empty"), nil
	}

	sgNode := node.NewScatterGatherNode("sg3", nil, gatherFunc)
	defer cleanupNodes(t, []node.Node{sgNode})

	res, err := sgNode.Process([]byte("req"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(res) != "empty" {
		t.Errorf("expected 'empty', got %s", string(res))
	}
}
