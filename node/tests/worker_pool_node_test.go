package node_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestWorkerPoolNode_BasicProcess(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-node", 3)
	err := wp.Create()
	if err != nil {
		t.Fatalf("Failed to create WorkerPoolNode: %v", err)
	}
	defer cleanupNodes(t, []node.Node{wp})

	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("processed "), input...), nil
	})

	result, err := wp.Process([]byte("data"))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if string(result) != "processed data" {
		t.Errorf("Expected 'processed data', got '%s'", string(result))
	}

	if wp.GetEventCount() != 1 {
		t.Errorf("Expected event count 1, got %d", wp.GetEventCount())
	}
}

func TestWorkerPoolNode_Concurrency(t *testing.T) {
	numWorkers := 5
	wp := node.NewWorkerPoolNode("wp-node", numWorkers)
	err := wp.Create()
	if err != nil {
		t.Fatalf("Failed to create WorkerPoolNode: %v", err)
	}
	defer cleanupNodes(t, []node.Node{wp})

	// Use a WaitGroup to ensure all workers are blocked initially
	var startWg sync.WaitGroup
	startWg.Add(numWorkers)
	var processWg sync.WaitGroup
	processWg.Add(numWorkers)

	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		startWg.Done()
		processWg.Wait() // Block until all workers have picked up a job
		return input, nil
	})

	var resultWg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		resultWg.Add(1)
		go func(idx int) {
			defer resultWg.Done()
			_, err := wp.Process([]byte("test"))
			if err != nil {
				t.Errorf("Worker %d failed: %v", idx, err)
			}
		}(i)
	}

	// Wait for all workers to start processing
	startWg.Wait()
	// Unblock all workers simultaneously
	processWg.Done()
	processWg.Done()
	processWg.Done()
	processWg.Done()
	processWg.Done() // Done numWorkers times, could also use channels, but this works

	resultWg.Wait()

	if wp.GetEventCount() != uint64(numWorkers) {
		t.Errorf("Expected event count %d, got %d", numWorkers, wp.GetEventCount())
	}
}

func TestWorkerPoolNode_ProcessWithContextTimeout(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-node", 1)
	err := wp.Create()
	if err != nil {
		t.Fatalf("Failed to create WorkerPoolNode: %v", err)
	}
	defer cleanupNodes(t, []node.Node{wp})

	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(100 * time.Millisecond)
		return input, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err = wp.ProcessWithContext(ctx, []byte("data"))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected DeadlineExceeded, got %v", err)
	}
}

func TestWorkerPoolNode_Delete(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-node", 2)
	err := wp.Create()
	if err != nil {
		t.Fatalf("Failed to create WorkerPoolNode: %v", err)
	}

	// Submit a slow job that will keep the worker busy while we delete
	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(50 * time.Millisecond)
		return input, nil
	})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := wp.Process([]byte("data"))
		// It might succeed or return ErrWorkerPoolClosed depending on timing
		if err != nil && !errors.Is(err, node.ErrWorkerPoolClosed) && err.Error() != "worker pool is closed" {
			t.Errorf("Unexpected error: %v", err)
		}
	}()

	// Give the goroutine time to submit the job
	time.Sleep(10 * time.Millisecond)

	err = wp.Delete()
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	wg.Wait()

	// Try processing after close
	_, err = wp.Process([]byte("data"))
	if err == nil || err.Error() != "worker pool is closed" {
		t.Errorf("Expected worker pool is closed error, got %v", err)
	}
}
