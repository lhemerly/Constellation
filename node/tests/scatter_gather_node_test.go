package node_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/lhemerly/Constellation/node"
)

func TestScatterGatherNode_Success(t *testing.T) {
	n1 := node.NewBaseNode("scatter-1")
	n1.SetProcessFunc(func(b []byte) ([]byte, error) {
		return append(b, []byte("-1")...), nil
	})

	n2 := node.NewBaseNode("scatter-2")
	n2.SetProcessFunc(func(b []byte) ([]byte, error) {
		return append(b, []byte("-2")...), nil
	})

	gatherFunc := func(results [][]byte) ([]byte, error) {
		var out []byte
		for _, r := range results {
			out = append(out, r...)
		}
		return out, nil
	}

	sgn, err := node.NewScatterGatherNode("sg-node", []node.Node{n1, n2}, gatherFunc)
	if err != nil {
		t.Fatalf("Failed to create ScatterGatherNode: %v", err)
	}

	result, err := sgn.Process([]byte("test"))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Results order is deterministic because we iterate over the slice
	expected := []byte("test-1test-2")
	if !bytes.Equal(result, expected) {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestScatterGatherNode_EmptyScatterNodes(t *testing.T) {
	_, err := node.NewScatterGatherNode("sg-node", []node.Node{}, func([][]byte) ([]byte, error) { return nil, nil })
	if err == nil {
		t.Fatal("Expected error when providing empty scatter nodes")
	}
}

func TestScatterGatherNode_NilGatherFunc(t *testing.T) {
	n1 := node.NewBaseNode("scatter-1")
	_, err := node.NewScatterGatherNode("sg-node", []node.Node{n1}, nil)
	if err == nil {
		t.Fatal("Expected error when providing nil gather func")
	}
}

func TestScatterGatherNode_ScatterError(t *testing.T) {
	n1 := node.NewBaseNode("scatter-1")
	n1.SetProcessFunc(func(b []byte) ([]byte, error) {
		return nil, errors.New("scatter error")
	})

	n2 := node.NewBaseNode("scatter-2")

	sgn, err := node.NewScatterGatherNode("sg-node", []node.Node{n1, n2}, func([][]byte) ([]byte, error) { return nil, nil })
	if err != nil {
		t.Fatalf("Failed to create ScatterGatherNode: %v", err)
	}

	_, err = sgn.Process([]byte("test"))
	if err == nil {
		t.Fatal("Expected process to fail because of scatter node error")
	}
}
