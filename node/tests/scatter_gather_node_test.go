package node_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestScatterGatherNode_Success(t *testing.T) {
	sg := node.NewScatterGatherNode("sg-node", 500*time.Millisecond)
	if err := sg.Create(); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer sg.Delete()

	t1 := node.NewBaseNode("target1")
	t1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("res1"), nil
	})
	sg.AddTarget(t1)

	t2 := node.NewBaseNode("target2")
	t2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("res2"), nil
	})
	sg.AddTarget(t2)

	res, err := sg.Process([]byte("input"))
	if err != nil {
		t.Fatalf("Process() unexpected error: %v", err)
	}

	resultStr := string(res)
	// Because go routines order is not guaranteed, check if both results are present
	if !strings.Contains(resultStr, "res1") || !strings.Contains(resultStr, "res2") || len(resultStr) != 8 {
		t.Errorf("Process() unexpected result: %v", resultStr)
	}
}

func TestScatterGatherNode_Timeout(t *testing.T) {
	sg := node.NewScatterGatherNode("sg-node-timeout", 100*time.Millisecond)
	if err := sg.Create(); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer sg.Delete()

	// Target 1: fast, succeeds
	t1 := node.NewBaseNode("target1")
	t1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("res1"), nil
	})
	sg.AddTarget(t1)

	// Target 2: slow, triggers timeout
	t2 := node.NewBaseNode("target2")

	blockCh := make(chan struct{})
	t2.SetProcessFunc(func(input []byte) ([]byte, error) {
		<-blockCh
		return []byte("res2"), nil
	})
	sg.AddTarget(t2)

	// Execute
	res, err := sg.Process([]byte("input"))

	// We expect partial success: res1 + timeout error
	if !errors.Is(err, node.ErrScatterGatherTimeout) {
		t.Fatalf("Process() expected timeout error, got %v", err)
	}

	if string(res) != "res1" {
		t.Errorf("Process() expected partial result 'res1', got '%v'", string(res))
	}

	// Clean up background goroutine by closing channel
	close(blockCh)
}

func TestScatterGatherNode_AllFail(t *testing.T) {
	sg := node.NewScatterGatherNode("sg-node-fail", 100*time.Millisecond)
	if err := sg.Create(); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer sg.Delete()

	expectedErr := errors.New("simulated failure")

	t1 := node.NewBaseNode("target1")
	t1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, expectedErr
	})
	sg.AddTarget(t1)

	res, err := sg.Process([]byte("input"))

	if !errors.Is(err, node.ErrScatterGatherFailed) {
		t.Fatalf("Process() expected ErrScatterGatherFailed, got %v", err)
	}

	if !errors.Is(err, expectedErr) {
		t.Fatalf("Process() expected wrapped simulated failure error, got %v", err)
	}

	if res != nil {
		t.Errorf("Process() expected nil result on complete failure, got '%v'", string(res))
	}
}
