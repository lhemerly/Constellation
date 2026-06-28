package node_test

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestWorkerPoolNode_QueueFull(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp", 1, 1)
	err := wp.Create()
	if err != nil {
		t.Fatalf("Failed to create worker pool: %v", err)
	}
	defer wp.Delete()

	// Block the worker so the queue fills up
	blockCh := make(chan struct{})
	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		<-blockCh
		return input, nil
	})

	// Fill the worker
	_, err = wp.Process([]byte("task1"))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	time.Sleep(10 * time.Millisecond) // Let the worker pick up the task

	// Fill the queue
	_, err = wp.Process([]byte("task2"))
	if err != nil {
		t.Fatalf("Expected no error on filling queue, got %v", err)
	}

	// Queue should now be full
	_, err = wp.Process([]byte("task3"))
	if err != node.ErrQueueFull {
		t.Fatalf("Expected ErrQueueFull, got %v", err)
	}

	close(blockCh) // Unblock worker so Delete can finish
}

func TestWorkerPoolNode_Processing(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp", 10, 2)
	err := wp.Create()
	if err != nil {
		t.Fatalf("Failed to create worker pool: %v", err)
	}
	defer wp.Delete()

	var mu sync.Mutex
	var processed [][]byte

	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		mu.Lock()
		processed = append(processed, input)
		mu.Unlock()
		return input, nil
	})

	tasks := [][]byte{[]byte("task1"), []byte("task2"), []byte("task3")}
	for _, task := range tasks {
		_, err := wp.Process(task)
		if err != nil {
			t.Fatalf("Failed to process task: %v", err)
		}
	}

	time.Sleep(50 * time.Millisecond) // Wait for workers to finish

	mu.Lock()
	defer mu.Unlock()
	if len(processed) != len(tasks) {
		t.Fatalf("Expected %d tasks processed, got %d", len(tasks), len(processed))
	}

	// Verify all tasks were processed
	for _, task := range tasks {
		found := false
		for _, p := range processed {
			if bytes.Equal(task, p) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Task %s not found in processed tasks", task)
		}
	}
}

func TestWorkerPoolNode_Teardown(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp", 10, 1)
	err := wp.Create()
	if err != nil {
		t.Fatalf("Failed to create worker pool: %v", err)
	}

	blockCh := make(chan struct{})
	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		<-blockCh
		return input, nil
	})

	_, err = wp.Process([]byte("task1"))
	if err != nil {
		t.Fatalf("Failed to process task: %v", err)
	}

	time.Sleep(10 * time.Millisecond) // Let the worker pick up the task

	// Delete in a goroutine because it waits for workers to finish
	done := make(chan struct{})
	go func() {
		err := wp.Delete()
		if err != nil {
			t.Errorf("Delete failed: %v", err)
		}
		close(done)
	}()

	time.Sleep(10 * time.Millisecond) // Let Delete block

	// Now try to add a task while shutting down
	_, err = wp.Process([]byte("task2"))
	if err == nil {
		t.Errorf("Expected error processing during shutdown, got nil")
	}

	close(blockCh) // Unblock the worker

	select {
	case <-done:
		// Delete finished successfully
	case <-time.After(1 * time.Second):
		t.Fatal("Delete timed out waiting for worker")
	}
}
