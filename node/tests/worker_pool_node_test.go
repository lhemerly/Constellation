package node_test

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestWorkerPoolNode_ConcurrentProcessing(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-1", 5, 10)
	err := wp.Create()
	if err != nil {
		t.Fatalf("Failed to create WorkerPoolNode: %v", err)
	}
	defer func() {
		_ = wp.Delete()
	}()

	var mu sync.Mutex
	processedCount := 0

	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(10 * time.Millisecond) // Simulate work
		mu.Lock()
		processedCount++
		mu.Unlock()
		return append(input, []byte("-processed")...), nil
	})

	var wg sync.WaitGroup
	numRequests := 10

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			res, err := wp.Process([]byte("test"))
			if err != nil {
				t.Errorf("Process failed: %v", err)
				return
			}
			expected := []byte("test-processed")
			if !bytes.Equal(res, expected) {
				t.Errorf("Expected %s, got %s", expected, res)
			}
		}(i)
	}

	wg.Wait()

	if processedCount != numRequests {
		t.Fatalf("Expected %d requests to be processed, got %d", numRequests, processedCount)
	}
}

func TestWorkerPoolNode_GracefulShutdown(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-shutdown", 2, 5)
	err := wp.Create()
	if err != nil {
		t.Fatalf("Failed to create WorkerPoolNode: %v", err)
	}

	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(50 * time.Millisecond) // Simulate slow work
		return input, nil
	})

	// Send a few requests
	for i := 0; i < 3; i++ {
		go func() {
			_, _ = wp.Process([]byte("test"))
		}()
	}

	time.Sleep(10 * time.Millisecond) // Ensure jobs are queued

	// Delete the node
	err = wp.Delete()
	if err != nil {
		t.Fatalf("Failed to delete WorkerPoolNode: %v", err)
	}

	// Any new requests should fail with ErrWorkerPoolClosed
	_, err = wp.Process([]byte("test_after_close"))
	// When using Process(), it increments eventCounter and then calls poolProcess.
	// We need to check if the returned error is ErrWorkerPoolClosed.
	if err != node.ErrWorkerPoolClosed {
		t.Fatalf("Expected ErrWorkerPoolClosed, got: %v", err)
	}
}
