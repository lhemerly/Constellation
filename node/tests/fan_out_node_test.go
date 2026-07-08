package node_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestFanOutNode_Success(t *testing.T) {
	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("1:"), input...), nil
	})

	n2 := node.NewBaseNode("n2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("2:"), input...), nil
	})

	fo := node.NewFanOutNode("fo", []node.Node{n1, n2}, 2*time.Second)
	if err := fo.Create(); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	defer fo.Delete()

	out, err := fo.Process([]byte("test"))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	outStr := string(out)
	if !strings.Contains(outStr, "1:test") || !strings.Contains(outStr, "2:test") {
		t.Fatalf("Unexpected output: %s", outStr)
	}
}

func TestFanOutNode_PartialTimeout(t *testing.T) {
	n1 := node.NewBaseNode("fast")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("fast-result"), nil
	})

	n2 := node.NewBaseNode("slow")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(200 * time.Millisecond)
		return []byte("slow-result"), nil
	})

	fo := node.NewFanOutNode("fo-timeout", []node.Node{n1, n2}, 50*time.Millisecond)
	fo.Create()
	defer fo.Delete()

	out, err := fo.Process([]byte("test"))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !bytes.Equal(out, []byte("fast-result")) {
		t.Fatalf("Expected fast-result, got: %s", string(out))
	}
}

func TestFanOutNode_AllFailed(t *testing.T) {
	n1 := node.NewBaseNode("err1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("err1")
	})

	n2 := node.NewBaseNode("err2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("err2")
	})

	fo := node.NewFanOutNode("fo-err", []node.Node{n1, n2}, 1*time.Second)
	fo.Create()
	defer fo.Delete()

	_, err := fo.Process([]byte("test"))
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "all target nodes failed or timed out") {
		t.Fatalf("Unexpected error format: %v", err)
	}
}
