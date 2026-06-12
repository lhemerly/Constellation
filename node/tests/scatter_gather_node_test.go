package node_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestScatterGatherNode_Success(t *testing.T) {
	n1 := node.NewBaseNode("target1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("res1"), nil
	})

	n2 := node.NewBaseNode("target2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("res2"), nil
	})

	n3 := node.NewBaseNode("target3")
	n3.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(100 * time.Millisecond) // slow target
		return []byte("res3"), nil
	})

	sgn := node.NewScatterGatherNode("sg", []node.Node{n1, n2, n3}, 2, 500*time.Millisecond)

	err := sgn.Create()
	if err != nil {
		t.Fatalf("Failed to create node: %v", err)
	}

	output, err := sgn.Process([]byte("input"))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	outStr := string(output)
	// Quorum is 2, so it should return res1+res2 or res2+res1
	if !strings.Contains(outStr, "res1") || !strings.Contains(outStr, "res2") {
		t.Errorf("Unexpected output: %s", outStr)
	}
	if strings.Contains(outStr, "res3") {
		t.Errorf("Output should not contain res3 since quorum of 2 was reached early")
	}

	err = sgn.Delete()
	if err != nil {
		t.Fatalf("Failed to delete node: %v", err)
	}
}

func TestScatterGatherNode_FailureNoQuorum(t *testing.T) {
	n1 := node.NewBaseNode("target1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("fail1")
	})

	n2 := node.NewBaseNode("target2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("fail2")
	})

	sgn := node.NewScatterGatherNode("sg", []node.Node{n1, n2}, 1, 500*time.Millisecond)

	_, err := sgn.Process([]byte("input"))
	if err == nil {
		t.Fatal("Expected error due to failure to reach quorum")
	}
	if !strings.Contains(err.Error(), "scatter-gather failed to reach quorum") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestScatterGatherNode_Timeout(t *testing.T) {
	n1 := node.NewBaseNode("target1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(200 * time.Millisecond)
		return []byte("res1"), nil
	})

	n2 := node.NewBaseNode("target2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(200 * time.Millisecond)
		return []byte("res2"), nil
	})

	sgn := node.NewScatterGatherNode("sg", []node.Node{n1, n2}, 2, 50*time.Millisecond)

	_, err := sgn.Process([]byte("input"))
	if err == nil {
		t.Fatal("Expected timeout error")
	}
	if !strings.Contains(err.Error(), "scatter-gather timeout") {
		t.Errorf("Unexpected error message: %v", err)
	}
}
