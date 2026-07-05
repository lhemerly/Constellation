package node_test

import (
	"errors"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestWorkerPoolNode_Success(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-success", 2, 5)
	if err := wp.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	defer wp.Delete()

	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("processed: "), input...), nil
	})

	res, err := wp.Process([]byte("task1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(res) != "processed: task1" {
		t.Errorf("expected 'processed: task1', got '%s'", string(res))
	}
}

func TestWorkerPoolNode_QueueFull(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-queue-full", 1, 1)
	if err := wp.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	defer wp.Delete()

	blockCh := make(chan struct{})
	defer close(blockCh) // ensure we don't deadlock on teardown

	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		<-blockCh // block the worker
		return input, nil
	})

	// 1. First task is taken by the single worker, blocking it
	go func() {
		wp.Process([]byte("task1"))
	}()

	time.Sleep(50 * time.Millisecond) // Give worker time to pick up task1

	// 2. Second task enters the queue (queue size is 1)
	go func() {
		wp.Process([]byte("task2"))
	}()

	time.Sleep(50 * time.Millisecond) // Give time for task2 to enter queue

	// 3. Third task should fail because the queue is full
	_, err := wp.Process([]byte("task3"))
	if !errors.Is(err, node.ErrQueueFull) {
		t.Fatalf("expected ErrQueueFull, got %v", err)
	}
}

func TestWorkerPoolNode_Shutdown(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-shutdown", 2, 5)
	if err := wp.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		return input, nil
	})

	// Shutdown the node
	if err := wp.Delete(); err != nil {
		t.Fatalf("failed to delete node: %v", err)
	}

	// Try to process after shutdown
	_, err := wp.Process([]byte("task"))
	if !errors.Is(err, node.ErrNodeClosed) {
		t.Fatalf("expected ErrNodeClosed, got %v", err)
	}
}
