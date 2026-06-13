package node_test

import (
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestWorkerPoolNode_Basic(t *testing.T) {
	w1 := node.NewBaseNode("w1")
	var processed int32
	w1.SetProcessFunc(func(in []byte) ([]byte, error) {
		atomic.AddInt32(&processed, 1)
		return in, nil
	})

	wp := node.NewWorkerPoolNode("wp", []node.Node{w1}, 10)
	if err := wp.Create(); err != nil {
		t.Fatalf("Failed to create worker pool: %v", err)
	}
	defer wp.Delete()

	// Give workers time to start
	time.Sleep(10 * time.Millisecond)

	out, err := wp.Process([]byte("test"))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if string(out) != "test" {
		t.Errorf("Expected output 'test', got '%s'", string(out))
	}

	if atomic.LoadInt32(&processed) != 1 {
		t.Errorf("Expected worker to process exactly 1 time")
	}
}

func TestWorkerPoolNode_QueueFull(t *testing.T) {
	w1 := node.NewBaseNode("w1")
	blockCh := make(chan struct{})
	w1.SetProcessFunc(func(in []byte) ([]byte, error) {
		// block until closed
		<-blockCh
		return in, nil
	})

	// small queue size of 1
	wp := node.NewWorkerPoolNode("wp", []node.Node{w1}, 1)
	if err := wp.Create(); err != nil {
		t.Fatalf("Failed to create worker pool: %v", err)
	}

	// 1st item will be taken by the worker immediately
	go func() { wp.Process([]byte("1")) }()
	time.Sleep(10 * time.Millisecond)

	// 2nd item will fill the queue (queue size is 1)
	go func() { wp.Process([]byte("2")) }()
	time.Sleep(10 * time.Millisecond)

	// 3rd item should hit ErrWorkerPoolFull
	_, err := wp.Process([]byte("3"))
	if err == nil || !errors.Is(err, node.ErrWorkerPoolFull) {
		t.Errorf("Expected ErrWorkerPoolFull, got %v", err)
	}

	close(blockCh)
	wp.Delete()
}

func TestWorkerPoolNode_ShutdownBehavior(t *testing.T) {
	w1 := node.NewBaseNode("w1")
	wp := node.NewWorkerPoolNode("wp", []node.Node{w1}, 10)

	if err := wp.Create(); err != nil {
		t.Fatalf("Failed to create worker pool: %v", err)
	}

	wp.Delete()

	_, err := wp.Process([]byte("test"))
	if err == nil || !errors.Is(err, node.ErrWorkerPoolClosed) {
		t.Errorf("Expected ErrWorkerPoolClosed after delete, got %v", err)
	}
}

func TestWorkerPoolNode_PanicNoWorkers(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic when creating pool with no workers")
		} else {
			if !strings.Contains(r.(string), "requires at least one worker") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}
	}()

	_ = node.NewWorkerPoolNode("wp", []node.Node{}, 10)
}
