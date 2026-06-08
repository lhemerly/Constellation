package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"sync/atomic"
	"testing"
	"time"
)

func TestWorkerPoolNode_Success(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-1", 2, 5)

	var processedCount int32
	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		atomic.AddInt32(&processedCount, 1)
		return nil, nil
	})

	err := wp.Create()
	if err != nil {
		t.Fatalf("failed to create worker pool: %v", err)
	}

	for i := 0; i < 5; i++ {
		res, err := wp.Process([]byte("task"))
		if err != nil {
			t.Fatalf("unexpected error queueing task: %v", err)
		}
		if string(res) != "accepted" {
			t.Errorf("expected accepted, got %s", string(res))
		}
	}

	// Wait for workers to process tasks
	time.Sleep(100 * time.Millisecond)

	count := atomic.LoadInt32(&processedCount)
	if count != 5 {
		t.Errorf("expected 5 processed tasks, got %d", count)
	}

	err = wp.Delete()
	if err != nil {
		t.Fatalf("failed to delete worker pool: %v", err)
	}
}

func TestWorkerPoolNode_QueueFull(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-full", 1, 1)

	// Worker sleeps to block the queue
	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(100 * time.Millisecond)
		return nil, nil
	})

	err := wp.Create()
	if err != nil {
		t.Fatalf("failed to create worker pool: %v", err)
	}
	defer wp.Delete()

	// Fill the worker and the queue
	_, err = wp.Process([]byte("task1")) // Worker takes it
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wait a moment for the worker to pull task1 from the queue
	time.Sleep(10 * time.Millisecond)

	_, err = wp.Process([]byte("task2")) // Queue takes it
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// This should fail because queue is full
	_, err = wp.Process([]byte("task3"))
	if !errors.Is(err, node.ErrQueueFull) {
		t.Fatalf("expected ErrQueueFull, got %v", err)
	}
}

func TestWorkerPoolNode_ClosedPool(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-closed", 1, 1)
	err := wp.Create()
	if err != nil {
		t.Fatalf("failed to create worker pool: %v", err)
	}

	err = wp.Delete()
	if err != nil {
		t.Fatalf("failed to delete worker pool: %v", err)
	}

	_, err = wp.Process([]byte("task"))
	if !errors.Is(err, node.ErrWorkerPoolClosed) {
		t.Fatalf("expected ErrWorkerPoolClosed, got %v", err)
	}
}
