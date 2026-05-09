package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"strings"
	"testing"
)

func TestFilterNode_SuccessPasses(t *testing.T) {
	target := node.NewBaseNode("target")
	target.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte(strings.ToUpper(string(input))), nil
	})

	filter := node.NewFilterNode("filter", target, func(input []byte) bool {
		return strings.Contains(string(input), "allow")
	})
	defer cleanupNodes(t, []node.Node{filter})

	if err := filter.Create(); err != nil {
		t.Fatalf("unexpected error on create: %v", err)
	}

	res, err := filter.Process([]byte("please allow this"))
	if err != nil {
		t.Fatalf("unexpected error on process: %v", err)
	}

	if string(res) != "PLEASE ALLOW THIS" {
		t.Errorf("expected 'PLEASE ALLOW THIS', got '%s'", string(res))
	}
}

func TestFilterNode_Drops(t *testing.T) {
	target := node.NewBaseNode("target")
	target.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("should not reach here"), nil
	})

	filter := node.NewFilterNode("filter", target, func(input []byte) bool {
		return strings.Contains(string(input), "allow")
	})
	defer cleanupNodes(t, []node.Node{filter})

	_, err := filter.Process([]byte("please deny this"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, node.ErrFilterDropped) {
		t.Errorf("expected ErrFilterDropped, got %v", err)
	}
}

func TestFilterNode_PanicsOnNilTarget(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on nil target")
		} else if err, ok := r.(string); ok && err != node.ErrNoTargetNode.Error() {
			t.Errorf("expected ErrNoTargetNode panic, got %v", r)
		}
	}()

	node.NewFilterNode("filter", nil, func(input []byte) bool { return true })
}

func TestFilterNode_PanicsOnNilPredicate(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on nil predicate")
		} else if err, ok := r.(string); ok && err != "FilterNode requires a predicate function" {
			t.Errorf("expected missing predicate panic, got %v", r)
		}
	}()

	target := node.NewBaseNode("target")
	node.NewFilterNode("filter", target, nil)
}
