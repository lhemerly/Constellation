package node_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestScatterGatherNode_Success(t *testing.T) {
	node1 := node.NewBaseNode("node1")
	node1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-1")...), nil
	})

	node2 := node.NewBaseNode("node2")
	node2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-2")...), nil
	})

	sgNode := node.NewScatterGatherNode("sg", []node.Node{node1, node2}, 500*time.Millisecond)

	input := []byte("data")
	output, err := sgNode.Process(input)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	outStr := string(output)
	// Because of concurrency, order of appending isn't strictly guaranteed,
	// but it should contain both.
	if !strings.Contains(outStr, "data-1") || !strings.Contains(outStr, "data-2") {
		t.Errorf("Expected output to contain responses from both nodes, got: %s", outStr)
	}
}

func TestScatterGatherNode_Timeout(t *testing.T) {
	node1 := node.NewBaseNode("node1")
	node1.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(200 * time.Millisecond)
		return []byte("success"), nil
	})

	sgNode := node.NewScatterGatherNode("sg", []node.Node{node1}, 50*time.Millisecond)

	_, err := sgNode.Process([]byte("data"))
	if err == nil {
		t.Fatal("Expected error due to timeout, got nil")
	}

	if !errors.Is(err, node.ErrScatterGatherTimeout) {
		t.Errorf("Expected ErrScatterGatherTimeout, got: %v", err)
	}
}

func TestScatterGatherNode_PartialFailure(t *testing.T) {
	node1 := node.NewBaseNode("node1")
	node1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("success"), nil
	})

	node2 := node.NewBaseNode("node2")
	node2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("simulated error")
	})

	sgNode := node.NewScatterGatherNode("sg", []node.Node{node1, node2}, 500*time.Millisecond)

	_, err := sgNode.Process([]byte("data"))
	if err == nil {
		t.Fatal("Expected error due to partial failure, got nil")
	}

	if !strings.Contains(err.Error(), "simulated error") {
		t.Errorf("Expected error to contain 'simulated error', got: %v", err)
	}
}

func TestScatterGatherNode_CustomGather(t *testing.T) {
	node1 := node.NewBaseNode("node1")
	node1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("A"), nil
	})

	node2 := node.NewBaseNode("node2")
	node2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("B"), nil
	})

	sgNode := node.NewScatterGatherNode("sg", []node.Node{node1, node2}, 500*time.Millisecond)
	sgNode.SetGatherFunc(func(results [][]byte) ([]byte, error) {
		return []byte("custom-aggregation"), nil
	})

	output, err := sgNode.Process([]byte("data"))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if string(output) != "custom-aggregation" {
		t.Errorf("Expected 'custom-aggregation', got: %s", string(output))
	}
}
