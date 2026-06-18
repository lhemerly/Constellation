package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"strings"
	"testing"
	"time"
)

func TestFanOutNode_Success(t *testing.T) {
	fanOut := node.NewFanOutNode("fanout-1", 500*time.Millisecond)

	node1 := node.NewBaseNode("n1")
	node1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("A"), nil
	})

	node2 := node.NewBaseNode("n2")
	node2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("B"), nil
	})

	fanOut.AddNode(node1)
	fanOut.AddNode(node2)

	defer cleanupNodes(t, []node.Node{fanOut, node1, node2})

	res, err := fanOut.Process([]byte("input"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resStr := string(res)
	if len(resStr) != 2 || (!strings.Contains(resStr, "A") || !strings.Contains(resStr, "B")) {
		t.Errorf("expected result to contain 'A' and 'B' in any order, got '%s'", resStr)
	}
}

func TestFanOutNode_Timeout(t *testing.T) {
	fanOut := node.NewFanOutNode("fanout-timeout", 50*time.Millisecond)

	slowNode := node.NewBaseNode("slow")
	blockCh := make(chan struct{})
	slowNode.SetProcessFunc(func(input []byte) ([]byte, error) {
		<-blockCh
		return []byte("slow"), nil
	})

	fastNode := node.NewBaseNode("fast")
	fastNode.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("fast"), nil
	})

	fanOut.AddNode(slowNode)
	fanOut.AddNode(fastNode)

	defer func() {
		close(blockCh) // ensure slowNode unblocks for cleanup
		cleanupNodes(t, []node.Node{fanOut, slowNode, fastNode})
	}()

	_, err := fanOut.Process([]byte("input"))
	if !errors.Is(err, node.ErrFanOutTimeout) {
		t.Errorf("expected ErrFanOutTimeout, got %v", err)
	}
}

func TestFanOutNode_Errors(t *testing.T) {
	fanOut := node.NewFanOutNode("fanout-errs", 500*time.Millisecond)

	errNode := node.NewBaseNode("errNode")
	expectedErr := errors.New("simulated failure")
	errNode.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, expectedErr
	})

	fanOut.AddNode(errNode)
	defer cleanupNodes(t, []node.Node{fanOut, errNode})

	_, err := fanOut.Process([]byte("input"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "simulated failure") {
		t.Errorf("expected error to contain 'simulated failure', got %v", err)
	}
}

func TestFanOutNode_NoNodes(t *testing.T) {
	fanOut := node.NewFanOutNode("fanout-empty", 500*time.Millisecond)
	defer cleanupNodes(t, []node.Node{fanOut})

	_, err := fanOut.Process([]byte("input"))
	if !errors.Is(err, node.ErrNoFanOutNodes) {
		t.Errorf("expected ErrNoFanOutNodes, got %v", err)
	}
}
