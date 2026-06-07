package node_test

import (
	"bytes"
	"errors"
	"github.com/lhemerly/Constellation/node"
	"testing"
	"time"
)

func TestFanOutNode_Success(t *testing.T) {
	n1 := node.NewBaseNode("dest1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("1:"), input...), nil
	})

	n2 := node.NewBaseNode("dest2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("2:"), input...), nil
	})

	fanout := node.NewFanOutNode("fanout1", []node.Node{n1, n2}, 2*time.Second)
	defer cleanupNodes(t, []node.Node{n1, n2, fanout})

	res, err := fanout.Process([]byte("data"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Because map/iteration order over slice might be deterministic here,
	// but goroutine completion order is not, we just check if both substrings exist.
	if !bytes.Contains(res, []byte("1:data")) || !bytes.Contains(res, []byte("2:data")) {
		t.Errorf("expected result to contain outputs from both destinations, got: %s", string(res))
	}
}

func TestFanOutNode_Timeout(t *testing.T) {
	n1 := node.NewBaseNode("dest1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("fast"), nil
	})

	n2 := node.NewBaseNode("dest2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(100 * time.Millisecond)
		return []byte("slow"), nil
	})

	fanout := node.NewFanOutNode("fanout2", []node.Node{n1, n2}, 50*time.Millisecond)
	defer cleanupNodes(t, []node.Node{n1, n2, fanout})

	res, err := fanout.Process([]byte("data"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(res) != "fast" {
		t.Errorf("expected only 'fast' to be returned due to timeout, got: %s", string(res))
	}
}

func TestFanOutNode_NoDestinations(t *testing.T) {
	fanout := node.NewFanOutNode("fanout3", []node.Node{}, 1*time.Second)
	defer cleanupNodes(t, []node.Node{fanout})

	_, err := fanout.Process([]byte("data"))
	if err == nil {
		t.Fatalf("expected error due to no destinations, got nil")
	}

	if err.Error() != "no destination nodes configured" {
		t.Errorf("expected 'no destination nodes configured', got: %v", err)
	}
}

func TestFanOutNode_AllFail(t *testing.T) {
	n1 := node.NewBaseNode("dest1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("err1")
	})

	n2 := node.NewBaseNode("dest2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("err2")
	})

	fanout := node.NewFanOutNode("fanout4", []node.Node{n1, n2}, 1*time.Second)
	defer cleanupNodes(t, []node.Node{n1, n2, fanout})

	_, err := fanout.Process([]byte("data"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	// Should join errors
	if err.Error() != "err1\nerr2" && err.Error() != "err2\nerr1" {
		t.Errorf("expected joined errors, got: %v", err)
	}
}
