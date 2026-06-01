package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"testing"
	"time"
)

func TestWorkerPoolNode_Basic(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-1", 2, 10)
	if err := wp.Create(); err != nil {
		t.Fatalf("Failed to create WorkerPoolNode: %v", err)
	}
	defer cleanupNodes(t, []node.Node{wp})

	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("processed: "), input...), nil
	})

	res, err := wp.Process([]byte("data"))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if string(res) != "processed: data" {
		t.Errorf("Expected 'processed: data', got '%s'", string(res))
	}
}

func TestWorkerPoolNode_Concurrency(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-concurrency", 3, 10)
	if err := wp.Create(); err != nil {
		t.Fatalf("Failed to create WorkerPoolNode: %v", err)
	}
	defer cleanupNodes(t, []node.Node{wp})

	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(50 * time.Millisecond) // Simulate work
		return input, nil
	})

	start := time.Now()
	done := make(chan struct{})

	// Send 3 jobs simultaneously (should take ~50ms total since there are 3 workers)
	for i := 0; i < 3; i++ {
		go func() {
			_, err := wp.Process([]byte("data"))
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			done <- struct{}{}
		}()
	}

	for i := 0; i < 3; i++ {
		<-done
	}

	duration := time.Since(start)
	if duration >= 100*time.Millisecond {
		t.Errorf("Expected duration < 100ms, got %v", duration)
	}
}

func TestWorkerPoolNode_Shutdown(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-shutdown", 2, 2)
	if err := wp.Create(); err != nil {
		t.Fatalf("Failed to create WorkerPoolNode: %v", err)
	}

	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(20 * time.Millisecond)
		return input, nil
	})

	// Fill queue
	go wp.Process([]byte("data1"))
	go wp.Process([]byte("data2"))
	time.Sleep(5 * time.Millisecond)

	// Delete while jobs are processing/queued
	err := wp.Delete()
	if err != nil {
		t.Fatalf("Unexpected error during delete: %v", err)
	}

	// Submitting after delete should return closed error
	_, err = wp.Process([]byte("data3"))
	if !errors.Is(err, node.ErrWorkerPoolClosed) {
		t.Errorf("Expected ErrWorkerPoolClosed, got %v", err)
	}
}
