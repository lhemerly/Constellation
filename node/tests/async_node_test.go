package node_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestAsyncNode(t *testing.T) {
	dest := node.NewBaseNode("dest")
	var count int32

	dest.SetProcessFunc(func(input []byte) ([]byte, error) {
		atomic.AddInt32(&count, 1)
		return nil, nil
	})

	an := node.NewAsyncNode("async-1", dest, 10)

	err := an.Create()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	defer func() {
		_ = an.Delete()
	}()

	for i := 0; i < 5; i++ {
		_, err := an.Process([]byte("test"))
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	}

	// Give the worker some time to process
	time.Sleep(50 * time.Millisecond)

	if atomic.LoadInt32(&count) != 5 {
		t.Errorf("expected 5 processes, got %d", count)
	}
}

func TestAsyncNode_Shutdown(t *testing.T) {
	dest := node.NewBaseNode("dest")
	an := node.NewAsyncNode("async-2", dest, 10)

	err := an.Create()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	err = an.Delete()
	if err != nil {
		t.Fatalf("expected nil error on Delete, got %v", err)
	}

	_, err = an.Process([]byte("test"))
	if err == nil || err.Error() != "node is shutting down" {
		t.Fatalf("expected 'node is shutting down' error, got %v", err)
	}
}
