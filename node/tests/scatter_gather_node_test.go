package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"strings"
	"testing"
	"time"
)

func TestScatterGatherNode_Success(t *testing.T) {
	sg := node.NewScatterGatherNode("sg-node", 500*time.Millisecond)
	sg.Create()
	defer sg.Delete()

	dest1 := node.NewBaseNode("dest1")
	dest1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("A"), nil
	})
	dest1.Create()
	defer dest1.Delete()

	dest2 := node.NewBaseNode("dest2")
	dest2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("B"), nil
	})
	dest2.Create()
	defer dest2.Delete()

	sg.AddDestination(dest1)
	sg.AddDestination(dest2)

	res, err := sg.Process([]byte("input"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resStr := string(res)
	if !strings.Contains(resStr, "A") || !strings.Contains(resStr, "B") {
		t.Errorf("expected output to contain both 'A' and 'B', got %s", resStr)
	}
	if len(resStr) != 2 {
		t.Errorf("expected length 2, got %d", len(resStr))
	}
}

func TestScatterGatherNode_PartialFailure(t *testing.T) {
	sg := node.NewScatterGatherNode("sg-node", 500*time.Millisecond)
	sg.Create()
	defer sg.Delete()

	dest1 := node.NewBaseNode("dest1")
	dest1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("A"), nil
	})
	dest1.Create()
	defer dest1.Delete()

	dest2 := node.NewBaseNode("dest2")
	dest2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("some error")
	})
	dest2.Create()
	defer dest2.Delete()

	sg.AddDestination(dest1)
	sg.AddDestination(dest2)

	res, err := sg.Process([]byte("input"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(res) != "A" {
		t.Errorf("expected output 'A', got %s", string(res))
	}
}

func TestScatterGatherNode_CompleteFailure(t *testing.T) {
	sg := node.NewScatterGatherNode("sg-node", 500*time.Millisecond)
	sg.Create()
	defer sg.Delete()

	expectedErr := errors.New("some error")
	dest1 := node.NewBaseNode("dest1")
	dest1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, expectedErr
	})
	dest1.Create()
	defer dest1.Delete()

	sg.AddDestination(dest1)

	_, err := sg.Process([]byte("input"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != expectedErr.Error() {
		t.Fatalf("expected expectedErr, got %v", err)
	}
}

func TestScatterGatherNode_Timeout(t *testing.T) {
	sg := node.NewScatterGatherNode("sg-node", 100*time.Millisecond)
	sg.Create()
	defer sg.Delete()

	blockCh := make(chan struct{})

	dest1 := node.NewBaseNode("dest1")
	dest1.SetProcessFunc(func(input []byte) ([]byte, error) {
		<-blockCh // block until unblocked or timeout
		return []byte("slow"), nil
	})
	dest1.Create()
	defer func() {
		close(blockCh) // unblock to allow Delete() to finish
		dest1.Delete()
	}()

	sg.AddDestination(dest1)

	_, err := sg.Process([]byte("input"))
	if !errors.Is(err, node.ErrProcessTimeout) {
		t.Fatalf("expected ErrProcessTimeout, got %v", err)
	}
}

func TestScatterGatherNode_EmptyDestinations(t *testing.T) {
	sg := node.NewScatterGatherNode("sg-node", 500*time.Millisecond)
	sg.Create()
	defer sg.Delete()

	res, err := sg.Process([]byte("input"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("expected empty output, got %s", string(res))
	}
}
