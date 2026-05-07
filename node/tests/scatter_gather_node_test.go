package node_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestScatterGatherNode_Process_Success(t *testing.T) {
	target1 := node.NewBaseNode("target-1")
	target1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("A"), nil
	})

	target2 := node.NewBaseNode("target-2")
	target2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("B"), nil
	})

	gatherFunc := func(results [][]byte, errs []error) ([]byte, error) {
		var b strings.Builder
		for _, res := range results {
			if res != nil {
				b.Write(res)
			}
		}
		for _, err := range errs {
			if err != nil {
				return nil, err
			}
		}
		return []byte(b.String()), nil
	}

	sgn := node.NewScatterGatherNode("sg-node", []node.Node{target1, target2}, time.Second, gatherFunc)

	err := sgn.Create()
	if err != nil {
		t.Fatalf("expected no error on create, got: %v", err)
	}

	output, err := sgn.Process([]byte("test"))
	if err != nil {
		t.Fatalf("expected no error on process, got: %v", err)
	}

	if string(output) != "AB" {
		t.Errorf("expected AB, got: %s", string(output))
	}

	cleanupNodes(t, []node.Node{sgn})
}

func TestScatterGatherNode_Process_Timeout(t *testing.T) {
	target1 := node.NewBaseNode("target-1")
	target1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("A"), nil
	})

	target2 := node.NewBaseNode("target-2")
	target2.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(200 * time.Millisecond) // This will cause timeout
		return []byte("B"), nil
	})

	gatherFunc := func(results [][]byte, errs []error) ([]byte, error) {
		for _, err := range errs {
			if err != nil && errors.Is(err, node.ErrScatterGatherTimeout) {
				return []byte("timeout-handled"), nil
			}
		}
		return nil, errors.New("expected timeout error")
	}

	sgn := node.NewScatterGatherNode("sg-node", []node.Node{target1, target2}, 50*time.Millisecond, gatherFunc)

	err := sgn.Create()
	if err != nil {
		t.Fatalf("expected no error on create, got: %v", err)
	}

	output, err := sgn.Process([]byte("test"))
	if err != nil {
		t.Fatalf("expected no error from process, got: %v", err)
	}

	if string(output) != "timeout-handled" {
		t.Errorf("expected timeout-handled, got: %s", string(output))
	}

	cleanupNodes(t, []node.Node{sgn})
}
