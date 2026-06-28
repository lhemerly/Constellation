package node_test

import (
	"strings"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestFanOutNode_Success(t *testing.T) {
	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("res1"), nil
	})

	n2 := node.NewBaseNode("n2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("res2"), nil
	})

	f := node.NewFanOutNode("fanout", []node.Node{n1, n2}, 1*time.Second)

	output, err := f.Process([]byte("input"))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Because of concurrency, order of "res1" and "res2" is not guaranteed
	strOut := string(output)
	if !strings.Contains(strOut, "res1") || !strings.Contains(strOut, "res2") {
		t.Fatalf("Expected output to contain res1 and res2, got %s", strOut)
	}
	if len(strOut) != len("res1res2") {
		t.Fatalf("Expected output length %d, got %d", len("res1res2"), len(strOut))
	}
}

func TestFanOutNode_Timeout(t *testing.T) {
	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(100 * time.Millisecond)
		return []byte("res1"), nil
	})

	f := node.NewFanOutNode("fanout", []node.Node{n1}, 10*time.Millisecond)

	_, err := f.Process([]byte("input"))
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "fan-out process timed out") {
		t.Fatalf("Expected timeout error, got %v", err)
	}
}

func TestFanOutNode_Error(t *testing.T) {
	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, node.ErrProcessTimeout // Just reusing an error for test
	})

	f := node.NewFanOutNode("fanout", []node.Node{n1}, 1*time.Second)

	_, err := f.Process([]byte("input"))
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "fan-out encountered errors") {
		t.Fatalf("Expected aggregated error, got %v", err)
	}
}
