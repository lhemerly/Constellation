package node_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestWorkerPoolNode_BasicProcessing(t *testing.T) {
	n := node.NewWorkerPoolNode("wp-basic", 2, 5)
	defer cleanupNodes(t, []node.Node{n})

	if err := n.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("processed: "), input...), nil
	})

	output, err := n.Process([]byte("data"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(output) != "processed: data" {
		t.Errorf("expected 'processed: data', got '%s'", string(output))
	}
}

func TestWorkerPoolNode_ConcurrencyAndQueueing(t *testing.T) {
	n := node.NewWorkerPoolNode("wp-queue", 2, 2)
	// Do not use defer cleanupNodes immediately to avoid closing the node while processes are pending

	if err := n.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	var mu sync.Mutex
	var processedCount int
	blockCh := make(chan struct{})

	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		<-blockCh // block to simulate long task and allow queue to fill
		mu.Lock()
		processedCount++
		mu.Unlock()
		return input, nil
	})

	// Fill the workers (2) + queue (2) = 4 tasks total can be accepted without blocking
	var wg sync.WaitGroup
	errorsChan := make(chan error, 10)

	// Dispatch 4 tasks that should be accepted
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := n.Process([]byte("task"))
			if err != nil {
				errorsChan <- err
			}
		}()
	}

	// Give a small sleep to ensure tasks are pulled to workers and queue
	time.Sleep(50 * time.Millisecond)

	// 5th task should be rejected with ErrQueueFull
	_, err := n.Process([]byte("task5"))
	if !errors.Is(err, node.ErrQueueFull) {
		t.Errorf("expected ErrQueueFull, got %v", err)
	}

	// Unblock workers
	close(blockCh)

	// Wait for the accepted tasks to finish
	wg.Wait()
	close(errorsChan)

	for e := range errorsChan {
		t.Errorf("unexpected error from queued tasks: %v", e)
	}

	if processedCount != 4 {
		t.Errorf("expected 4 tasks to be processed, got %d", processedCount)
	}

	n.Delete()
}

func TestWorkerPoolNode_Shutdown(t *testing.T) {
	n := node.NewWorkerPoolNode("wp-shutdown", 1, 1)

	if err := n.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	blockCh := make(chan struct{})
	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		<-blockCh // block to simulate long task
		return input, nil
	})

	// Start a long-running process
	go func() {
		n.Process([]byte("task"))
	}()
	time.Sleep(50 * time.Millisecond)

	// Queue another task which should get queued
	errChan := make(chan error, 1)
	go func() {
		_, err := n.Process([]byte("queued_task"))
		errChan <- err
	}()
	time.Sleep(50 * time.Millisecond)

	// Delete the node in a goroutine since it waits for the current task to finish
	go func() {
		n.Delete()
	}()

	time.Sleep(50 * time.Millisecond)

	// Unblock the worker so it can finish the current task
	close(blockCh)

	// The queued task should have failed with ErrNodeDeleted
	err := <-errChan
	if !errors.Is(err, node.ErrNodeDeleted) {
		t.Errorf("expected ErrNodeDeleted on shutdown, got %v", err)
	}
}
