package node_test

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestWorkerPoolNode_Basic(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-1", 2, 10)
	err := wp.Create()
	if err != nil {
		t.Fatalf("Failed to create WorkerPoolNode: %v", err)
	}
	defer func() {
		if err := wp.Delete(); err != nil {
			t.Errorf("Failed to delete WorkerPoolNode: %v", err)
		}
	}()

	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("processed_" + string(input)), nil
	})

	out, err := wp.Process([]byte("test"))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if string(out) != "processed_test" {
		t.Fatalf("Expected 'processed_test', got '%s'", string(out))
	}
}

func TestWorkerPoolNode_ErrorHandling(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-err", 2, 10)
	wp.Create()
	defer wp.Delete()

	expectedErr := errors.New("worker error")
	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, expectedErr
	})

	_, err := wp.Process([]byte("test"))
	if err != expectedErr {
		t.Fatalf("Expected '%v', got '%v'", expectedErr, err)
	}
}

func TestWorkerPoolNode_Concurrent(t *testing.T) {
	numWorkers := 3
	wp := node.NewWorkerPoolNode("wp-concurrent", numWorkers, 100)
	wp.Create()
	defer wp.Delete()

	var counter int32

	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		atomic.AddInt32(&counter, 1)
		time.Sleep(10 * time.Millisecond) // Simulate work
		return input, nil
	})

	numRequests := 20
	errChan := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(idx int) {
			_, err := wp.Process([]byte(fmt.Sprintf("req-%d", idx)))
			errChan <- err
		}(i)
	}

	for i := 0; i < numRequests; i++ {
		err := <-errChan
		if err != nil {
			t.Errorf("Unexpected error during concurrent process: %v", err)
		}
	}

	if atomic.LoadInt32(&counter) != int32(numRequests) {
		t.Fatalf("Expected counter to be %d, got %d", numRequests, counter)
	}
}

func TestWorkerPoolNode_ClosedHandling(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-closed", 1, 10)
	wp.Create()

	// Delete it directly
	err := wp.Delete()
	if err != nil {
		t.Fatalf("Failed to delete WorkerPoolNode: %v", err)
	}

	_, err = wp.Process([]byte("test"))
	if err != node.ErrWorkerPoolClosed {
		t.Fatalf("Expected '%v', got '%v'", node.ErrWorkerPoolClosed, err)
	}
}

func TestWorkerPoolNode_PendingJobsDrained(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-drain", 1, 10)

	// Create a worker pool but with a blocking process function to pile up jobs
	waitCh := make(chan struct{})
	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		<-waitCh
		return input, nil
	})
	wp.Create()

	errChan := make(chan error, 3)

	// Send a few requests, first one will block, others will queue up
	for i := 0; i < 3; i++ {
		go func() {
			_, err := wp.Process([]byte("test"))
			errChan <- err
		}()
	}

	// Give them a moment to enqueue
	time.Sleep(50 * time.Millisecond)

	// Release the blocked worker first so that Delete() can complete waiting for waitGroup
	close(waitCh)

	// Delete the pool. This should cancel contexts and close channels
	wp.Delete()

	for i := 0; i < 3; i++ {
		err := <-errChan
		if err != nil && err != node.ErrWorkerPoolClosed && !strings.Contains(err.Error(), "worker pool is closed") {
			t.Errorf("Expected 'ErrWorkerPoolClosed', got '%v'", err)
		}
	}
}
