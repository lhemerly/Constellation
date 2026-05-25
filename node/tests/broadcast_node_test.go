package node_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestBroadcastNode_Basic(t *testing.T) {
	bn := node.NewBroadcastNode("bn-1", 1*time.Second)

	sub1 := node.NewBaseNode("sub-1")
	sub1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("sub1_" + string(input)), nil
	})

	sub2 := node.NewBaseNode("sub-2")
	sub2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("sub2_" + string(input)), nil
	})

	bn.Subscribe(sub1)
	bn.Subscribe(sub2)

	out, err := bn.Process([]byte("test"))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	strOut := string(out)
	if !strings.Contains(strOut, "sub1_test") || !strings.Contains(strOut, "sub2_test") {
		t.Fatalf("Expected output to contain 'sub1_test' and 'sub2_test', got '%s'", strOut)
	}
}

func TestBroadcastNode_NoSubscribers(t *testing.T) {
	bn := node.NewBroadcastNode("bn-no-subs", 1*time.Second)

	out, err := bn.Process([]byte("test"))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if string(out) != "test" {
		t.Fatalf("Expected passthrough 'test', got '%s'", string(out))
	}
}

func TestBroadcastNode_Timeout(t *testing.T) {
	bn := node.NewBroadcastNode("bn-timeout", 50*time.Millisecond)

	sub1 := node.NewBaseNode("sub-1")
	sub1.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(100 * time.Millisecond)
		return []byte("sub1_" + string(input)), nil
	})

	bn.Subscribe(sub1)

	_, err := bn.Process([]byte("test"))
	if err == nil {
		t.Fatalf("Expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "broadcast timeout") {
		t.Fatalf("Expected 'broadcast timeout' error, got '%v'", err)
	}
}

func TestBroadcastNode_PartialErrors(t *testing.T) {
	bn := node.NewBroadcastNode("bn-errs", 1*time.Second)

	sub1 := node.NewBaseNode("sub-1")
	sub1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("sub1 error")
	})

	sub2 := node.NewBaseNode("sub-2")
	sub2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("sub2_" + string(input)), nil
	})

	bn.Subscribe(sub1)
	bn.Subscribe(sub2)

	_, err := bn.Process([]byte("test"))
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "sub1 error") {
		t.Fatalf("Expected 'sub1 error', got '%v'", err)
	}
}
