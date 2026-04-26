package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"strings"
	"testing"
)

func TestDAGNode(t *testing.T) {
	dag := node.NewDAGNode("dag")
	defer cleanupNodes(t, []node.Node{dag})

	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(in []byte) ([]byte, error) {
		return []byte("A"), nil
	})

	n2 := node.NewBaseNode("n2")
	n2.SetProcessFunc(func(in []byte) ([]byte, error) {
		return []byte("B"), nil
	})

	n3 := node.NewBaseNode("n3")
	n3.SetProcessFunc(func(in []byte) ([]byte, error) {
		return append(in, []byte("C")...), nil
	})

	dag.AddNode(n1)
	dag.AddNode(n2)
	dag.AddNode(n3, "n1", "n2") // n3 depends on n1 and n2

	if err := dag.Create(); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	res, err := dag.Process([]byte("initial"))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	outStr := string(res)
	// n3 receives n1 output ("A") and n2 output ("B") concatenated.
	// The order of concatenation in map iteration is non-deterministic in Go,
	// so it could be "ABC" or "BAC".
	if outStr != "ABC" && outStr != "BAC" {
		t.Errorf("expected ABC or BAC, got %s", outStr)
	}
}

func TestDAGNode_Failure(t *testing.T) {
	dag := node.NewDAGNode("dag")
	defer cleanupNodes(t, []node.Node{dag})

	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(in []byte) ([]byte, error) {
		return nil, errors.New("n1 failed")
	})

	n2 := node.NewBaseNode("n2")
	n2.SetProcessFunc(func(in []byte) ([]byte, error) {
		return []byte("B"), nil
	})

	dag.AddNode(n1)
	dag.AddNode(n2, "n1")

	if err := dag.Create(); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	_, err := dag.Process([]byte("initial"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "DAG execution failed") || !strings.Contains(err.Error(), "n1 failed") {
		t.Errorf("unexpected error message: %v", err)
	}
}
