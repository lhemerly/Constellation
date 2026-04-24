package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"testing"
)

func TestPipelineNode_SequentialProcessing(t *testing.T) {
	p := node.NewPipelineNode("pipe-1")
	node1 := node.NewBaseNode("stage-1")
	node2 := node.NewBaseNode("stage-2")
	node3 := node.NewBaseNode("stage-3")

	var nodes []node.Node
	nodes = append(nodes, p, node1, node2, node3)
	defer cleanupNodes(t, nodes)

	node1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-s1")...), nil
	})
	node2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-s2")...), nil
	})
	node3.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-s3")...), nil
	})

	p.AddStage(node1)
	p.AddStage(node2)
	p.AddStage(node3)

	res, err := p.Process([]byte("data"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "data-s1-s2-s3"
	if string(res) != expected {
		t.Errorf("expected %s, got %s", expected, string(res))
	}
}

func TestPipelineNode_Empty(t *testing.T) {
	p := node.NewPipelineNode("pipe-empty")
	defer cleanupNodes(t, []node.Node{p})

	_, err := p.Process([]byte("data"))
	if !errors.Is(err, node.ErrEmptyPipeline) {
		t.Errorf("expected ErrEmptyPipeline, got %v", err)
	}
}

func TestPipelineNode_ErrorPropagation(t *testing.T) {
	p := node.NewPipelineNode("pipe-err")
	node1 := node.NewBaseNode("stage-1")
	node2 := node.NewBaseNode("stage-err")
	node3 := node.NewBaseNode("stage-3")

	var nodes []node.Node
	nodes = append(nodes, p, node1, node2, node3)
	defer cleanupNodes(t, nodes)

	expectedErr := errors.New("stage 2 error")

	node1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-s1")...), nil
	})
	node2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, expectedErr
	})
	node3.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-s3")...), nil
	})

	p.AddStage(node1)
	p.AddStage(node2)
	p.AddStage(node3)

	_, err := p.Process([]byte("data"))
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected %v, got %v", expectedErr, err)
	}
}

func TestPipelineNode_EmptyInput(t *testing.T) {
	p := node.NewPipelineNode("pipe-empty-input")
	node1 := node.NewBaseNode("stage-1")
	node2 := node.NewBaseNode("stage-2")

	var nodes []node.Node
	nodes = append(nodes, p, node1, node2)
	defer cleanupNodes(t, nodes)

	node1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("stage1")...), nil
	})
	node2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-stage2")...), nil
	})

	p.AddStage(node1)
	p.AddStage(node2)

	res, err := p.Process([]byte{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "stage1-stage2"
	if string(res) != expected {
		t.Errorf("expected %s, got %s", expected, string(res))
	}
}
