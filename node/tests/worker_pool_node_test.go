package node_test

import (
	"github.com/lhemerly/Constellation/node"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWorkerPoolNode_BasicProcessing(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-1", 2, 5)
	if err := wp.Create(); err != nil {
		t.Fatalf("unexpected error creating worker pool: %v", err)
	}
	defer wp.Delete()

	var processedCount int32
	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		atomic.AddInt32(&processedCount, 1)
		return append([]byte("processed: "), input...), nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			res, err := wp.Process([]byte("data"))
			if err != nil {
				t.Errorf("unexpected error on process: %v", err)
			}
			if string(res) != "processed: data" {
				t.Errorf("unexpected result: %s", string(res))
			}
		}(i)
	}
	wg.Wait()

	if atomic.LoadInt32(&processedCount) != 5 {
		t.Errorf("expected 5 processed, got %d", processedCount)
	}
}

func TestWorkerPoolNode_QueueFull(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-full", 1, 1)
	if err := wp.Create(); err != nil {
		t.Fatalf("unexpected error creating worker pool: %v", err)
	}
	defer wp.Delete()

	// Slow down processing so queue backs up
	blockChan := make(chan struct{})
	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		<-blockChan
		return []byte("done"), nil
	})

	// 1 takes the worker
	go wp.Process([]byte("w1"))
	time.Sleep(10 * time.Millisecond) // Ensure worker picks it up

	// 1 takes the queue
	errChan := make(chan error, 1)
	go func() {
		_, err := wp.Process([]byte("q1"))
		errChan <- err
	}()
	time.Sleep(10 * time.Millisecond) // Ensure it gets queued

	// 1 gets rejected (queue full)
	_, err := wp.Process([]byte("reject"))
	if err == nil {
		t.Errorf("expected queue full error, got nil")
	} else if err.Error() != "worker pool queue is full" {
		t.Errorf("expected queue full error message, got: %v", err)
	}

	// Unblock workers
	close(blockChan)
	if err := <-errChan; err != nil {
		t.Errorf("unexpected error on queued item: %v", err)
	}
}

func TestWorkerPoolNode_DeleteShutsDownWorkers(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-del", 2, 2)
	if err := wp.Create(); err != nil {
		t.Fatalf("unexpected error creating worker pool: %v", err)
	}

	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(50 * time.Millisecond)
		return []byte("done"), nil
	})

	// Queue some jobs
	var wg sync.WaitGroup
	errChan := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := wp.Process([]byte("test"))
			errChan <- err
		}()
	}

	// Wait briefly so jobs get into the pool
	time.Sleep(10 * time.Millisecond)

	// Delete should wait for active workers, and return errors to the waiting queue (if any were stuck there, though here they are processing)
	err := wp.Delete()
	if err != nil {
		t.Fatalf("unexpected error deleting worker pool: %v", err)
	}

	wg.Wait()
	close(errChan)
	for err := range errChan {
		// they should succeed or fail cleanly, in this case they had enough time to start processing
		// but if they were stuck in queue they'd get "node deleted before processing"
		if err != nil && err.Error() != "node deleted before processing" && err.Error() != "node is shutting down while waiting for result" {
			t.Errorf("unexpected error: %v", err)
		}
	}

	// New process attempts should fail
	_, err = wp.Process([]byte("new"))
	if err == nil {
		t.Fatalf("expected error processing after delete")
	} else if err.Error() != "node is shutting down" {
		t.Errorf("unexpected error message: %v", err)
	}
}
