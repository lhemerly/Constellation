package node_test

import (
	"bytes"
	"github.com/lhemerly/Constellation/node"
	"testing"
)

func TestConditionNode_Success(t *testing.T) {
	trueNode := node.NewBaseNode("true-node")
	trueNode.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("true-branch"), nil
	})

	falseNode := node.NewBaseNode("false-node")
	falseNode.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("false-branch"), nil
	})

	condFunc := func(input []byte) bool {
		return bytes.Contains(input, []byte("true"))
	}

	condNode := node.NewConditionNode("cond-node", condFunc, trueNode, falseNode)

	// Ensure we cleanup nodes
	defer cleanupNodes(t, []node.Node{condNode, trueNode, falseNode})

	if err := condNode.Create(); err != nil {
		t.Fatalf("expected no error creating condition node, got %v", err)
	}

	// Test True branch
	res, err := condNode.Process([]byte("this should be true"))
	if err != nil {
		t.Fatalf("unexpected error processing true branch: %v", err)
	}
	if string(res) != "true-branch" {
		t.Errorf("expected true-branch, got %s", string(res))
	}

	// Test False branch
	res, err = condNode.Process([]byte("this is false"))
	if err != nil {
		t.Fatalf("unexpected error processing false branch: %v", err)
	}
	if string(res) != "false-branch" {
		t.Errorf("expected false-branch, got %s", string(res))
	}
}

func TestConditionNode_Panics(t *testing.T) {
	trueNode := node.NewBaseNode("true-node")
	falseNode := node.NewBaseNode("false-node")
	condFunc := func(input []byte) bool { return true }

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected panic when condFunc is nil")
			}
		}()
		node.NewConditionNode("cond-node", nil, trueNode, falseNode)
	}()

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected panic when trueNode is nil")
			}
		}()
		node.NewConditionNode("cond-node", condFunc, nil, falseNode)
	}()

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected panic when falseNode is nil")
			}
		}()
		node.NewConditionNode("cond-node", condFunc, trueNode, nil)
	}()
}
