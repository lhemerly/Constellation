package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"sync/atomic"
	"testing"
	"time"
)

func TestWorkerPoolNode_BasicProcess(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp1", 2, 10)
	err := wp.Create()
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	defer cleanupNodes(t, []node.Node{wp})

	var processedCount int32
	processDone := make(chan struct{})

	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		atomic.AddInt32(&processedCount, 1)
		if atomic.LoadInt32(&processedCount) == 2 {
			close(processDone)
		}
		return nil, nil
	})

	_, err = wp.Process([]byte("task1"))
	if err != nil {
		t.Fatalf("unexpected error on process: %v", err)
	}

	_, err = wp.Process([]byte("task2"))
	if err != nil {
		t.Fatalf("unexpected error on process: %v", err)
	}

	select {
	case <-processDone:
		// success
	case <-time.After(1 * time.Second):
		t.Fatalf("timed out waiting for tasks to process")
	}

	if atomic.LoadInt32(&processedCount) != 2 {
		t.Errorf("expected 2 processed tasks, got %d", processedCount)
	}
}

func TestWorkerPoolNode_QueueFull(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-full", 1, 1) // 1 worker, queue size 1
	err := wp.Create()
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	defer cleanupNodes(t, []node.Node{wp})

	blockCh := make(chan struct{})
	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		<-blockCh // block worker so queue can fill up
		return nil, nil
	})

	// Dispatch task 1 (will be picked up by the single worker and block)
	_, err = wp.Process([]byte("task1"))
	if err != nil {
		t.Fatalf("unexpected error on process task 1: %v", err)
	}

	// Give worker time to pull task1 from queue
	time.Sleep(10 * time.Millisecond)

	// Dispatch task 2 (will fill the queue size of 1)
	_, err = wp.Process([]byte("task2"))
	if err != nil {
		t.Fatalf("unexpected error on process task 2: %v", err)
	}

	// Dispatch task 3 (should fail because queue is full)
	_, err = wp.Process([]byte("task3"))
	if !errors.Is(err, node.ErrQueueFull) {
		t.Fatalf("expected ErrQueueFull, got %v", err)
	}

	// Unblock workers before teardown
	close(blockCh)
}

func TestWorkerPoolNode_Shutdown(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-shutdown", 2, 10)
	err := wp.Create()
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	blockCh := make(chan struct{})
	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		<-blockCh
		return nil, nil
	})

	// Add some tasks
	wp.Process([]byte("task1"))
	wp.Process([]byte("task2"))

	// Delete in a goroutine
	deleteDone := make(chan struct{})
	go func() {
		err := wp.Delete()
		if err != nil {
			t.Errorf("failed to delete node: %v", err)
		}
		close(deleteDone)
	}()

	// Ensure Delete blocks waiting for workers
	select {
	case <-deleteDone:
		t.Fatalf("Delete returned before workers finished")
	case <-time.After(50 * time.Millisecond):
		// Expected, workers are blocked
	}

	// Unblock workers
	close(blockCh)

	select {
	case <-deleteDone:
		// Expected
	case <-time.After(1 * time.Second):
		t.Fatalf("Delete timed out after workers unblocked")
	}

	// Attempting to process after shutdown should return an error
	_, err = wp.Process([]byte("task3"))
	if err == nil {
		t.Fatalf("expected error processing after shutdown, got nil")
	}
}
