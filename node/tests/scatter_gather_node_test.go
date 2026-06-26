package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"strings"
	"testing"
	"time"
)

func TestScatterGatherNode_Basic(t *testing.T) {
	sg := node.NewScatterGatherNode("sg1", 2*time.Second)

	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("n1:"), input...), nil
	})

	n2 := node.NewBaseNode("n2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("n2:"), input...), nil
	})

	sg.AddNode(n1)
	sg.AddNode(n2)

	defer cleanupNodes(t, []node.Node{sg, n1, n2})

	output, err := sg.Process([]byte("data"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	outStr := string(output)
	if !strings.Contains(outStr, "n1:data") || !strings.Contains(outStr, "n2:data") {
		t.Errorf("expected output to contain both n1:data and n2:data, got %s", outStr)
	}
}

func TestScatterGatherNode_Timeout(t *testing.T) {
	sg := node.NewScatterGatherNode("sg2", 100*time.Millisecond)

	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("fast"), nil
	})

	n2 := node.NewBaseNode("n2")
	blockCh := make(chan struct{})
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		<-blockCh
		return []byte("slow"), nil
	})
	defer close(blockCh)

	sg.AddNode(n1)
	sg.AddNode(n2)

	defer cleanupNodes(t, []node.Node{sg, n1, n2})

	output, err := sg.Process([]byte("data"))

	if !errors.Is(err, node.ErrScatterGatherTimeout) {
		t.Fatalf("expected ErrScatterGatherTimeout, got %v", err)
	}

	// Should have gathered partial results
	if string(output) != "fast" {
		t.Errorf("expected partial output 'fast', got %s", string(output))
	}
}

func TestScatterGatherNode_Errors(t *testing.T) {
	sg := node.NewScatterGatherNode("sg3", 2*time.Second)

	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("error from n1")
	})

	sg.AddNode(n1)
	defer cleanupNodes(t, []node.Node{sg, n1})

	_, err := sg.Process([]byte("data"))

	if err == nil || !strings.Contains(err.Error(), "error from n1") {
		t.Fatalf("expected error from n1, got %v", err)
	}
}

func TestScatterGatherNode_NoNodes(t *testing.T) {
	sg := node.NewScatterGatherNode("sg4", 2*time.Second)
	defer cleanupNodes(t, []node.Node{sg})

	_, err := sg.Process([]byte("data"))
	if err == nil || err.Error() != "no destination nodes available" {
		t.Fatalf("expected no destination nodes available error, got %v", err)
	}
}
