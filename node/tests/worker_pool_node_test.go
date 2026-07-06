package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWorkerPoolNode_Basic(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-1", 2, 10)
	defer cleanupNodes(t, []node.Node{wp})

	if err := wp.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

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

func TestWorkerPoolNode_Concurrency(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-2", 5, 20)
	defer cleanupNodes(t, []node.Node{wp})

	if err := wp.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	var processedCount int32
	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		atomic.AddInt32(&processedCount, 1)
		time.Sleep(10 * time.Millisecond) // Simulate work
		return input, nil
	})

	numTasks := 15
	var wg sync.WaitGroup
	for i := 0; i < numTasks; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := wp.Process([]byte("task"))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}

	wg.Wait()

	if atomic.LoadInt32(&processedCount) != int32(numTasks) {
		t.Errorf("expected %d tasks to be processed, got %d", numTasks, processedCount)
	}
}

func TestWorkerPoolNode_QueueFull(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-3", 1, 1)
	defer cleanupNodes(t, []node.Node{wp})

	if err := wp.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	blockCh := make(chan struct{})
	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		<-blockCh // Block worker
		return input, nil
	})

	// Dispatch first task, will be picked up by the single worker and block
	go wp.Process([]byte("task1"))

	// Give worker time to pick up the task
	time.Sleep(10 * time.Millisecond)

	// Dispatch second task, will fill the queue (size 1)
	go wp.Process([]byte("task2"))

	// Give time for queue to fill
	time.Sleep(10 * time.Millisecond)

	// Dispatch third task, should fail with ErrQueueFull
	_, err := wp.Process([]byte("task3"))
	if !errors.Is(err, node.ErrQueueFull) {
		t.Errorf("expected ErrQueueFull, got %v", err)
	}

	close(blockCh) // unblock to clean up
}

func TestWorkerPoolNode_Shutdown(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-4", 2, 5)

	if err := wp.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		return input, nil
	})

	// Process a task normally
	_, err := wp.Process([]byte("task"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Shutdown
	if err := wp.Delete(); err != nil {
		t.Fatalf("failed to delete node: %v", err)
	}

	// Processing after shutdown should fail
	_, err = wp.Process([]byte("task"))
	if err == nil {
		t.Fatalf("expected error after shutdown, got nil")
	}
}
