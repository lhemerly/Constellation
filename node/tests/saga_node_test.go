package node_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/lhemerly/Constellation/node"
)

func TestSagaNode_Success(t *testing.T) {
	sNode := node.NewSagaNode("saga-node")

	action1 := node.NewBaseNode("action1")
	action1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-a1")...), nil
	})

	action2 := node.NewBaseNode("action2")
	action2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-a2")...), nil
	})

	comp1 := node.NewBaseNode("comp1") // Should not be called
	comp1.SetProcessFunc(func(input []byte) ([]byte, error) {
		t.Error("Compensation 1 should not be called on success")
		return input, nil
	})

	sNode.AddStep(action1, comp1)
	sNode.AddStep(action2, nil)

	sNode.Create()
	defer cleanupNodes(t, []node.Node{sNode})

	res, err := sNode.Process([]byte("init"))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := "init-a1-a2"
	if string(res) != expected {
		t.Errorf("Expected result %s, got %s", expected, res)
	}
}

func TestSagaNode_FailureAndCompensation(t *testing.T) {
	sNode := node.NewSagaNode("saga-node")

	comp1Called := false
	comp2Called := false

	action1 := node.NewBaseNode("action1")
	action1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-a1")...), nil
	})

	comp1 := node.NewBaseNode("comp1")
	comp1.SetProcessFunc(func(input []byte) ([]byte, error) {
		comp1Called = true
		if string(input) != "init" {
			t.Errorf("Expected comp1 input to be 'init', got '%s'", input)
		}
		return input, nil
	})

	action2 := node.NewBaseNode("action2")
	action2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-a2")...), nil
	})

	comp2 := node.NewBaseNode("comp2")
	comp2.SetProcessFunc(func(input []byte) ([]byte, error) {
		comp2Called = true
		if string(input) != "init-a1" {
			t.Errorf("Expected comp2 input to be 'init-a1', got '%s'", input)
		}
		return input, nil
	})

	action3 := node.NewBaseNode("action3")
	action3.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("action3 failed")
	})

	sNode.AddStep(action1, comp1)
	sNode.AddStep(action2, comp2)
	sNode.AddStep(action3, nil)

	sNode.Create()
	defer cleanupNodes(t, []node.Node{sNode})

	res, err := sNode.Process([]byte("init"))

	if err == nil {
		t.Fatalf("Expected error from saga, got nil")
	}

	if res != nil {
		t.Errorf("Expected nil result on failure, got %s", res)
	}

	if !strings.Contains(err.Error(), "action3 failed") {
		t.Errorf("Expected error to contain 'action3 failed', got %v", err)
	}

	if !comp1Called {
		t.Errorf("Expected comp1 to be called")
	}

	if !comp2Called {
		t.Errorf("Expected comp2 to be called")
	}
}
