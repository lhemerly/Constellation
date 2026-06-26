package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"testing"
	"time"
)

func TestWorkerPoolNode_Basic(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp1", 2, 5)
	if err := wp.Create(); err != nil {
		t.Fatalf("failed to create worker pool: %v", err)
	}
	defer cleanupNodes(t, []node.Node{wp})

	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("processed: "), input...), nil
	})

	output, err := wp.Process([]byte("task1"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if string(output) != "processed: task1" {
		t.Errorf("expected 'processed: task1', got '%s'", string(output))
	}
}

func TestWorkerPoolNode_QueueFull(t *testing.T) {
	// 1 worker, queue size 1
	wp := node.NewWorkerPoolNode("wp2", 1, 1)
	if err := wp.Create(); err != nil {
		t.Fatalf("failed to create worker pool: %v", err)
	}
	defer cleanupNodes(t, []node.Node{wp})

	blockCh := make(chan struct{})
	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		<-blockCh // block the worker indefinitely
		return input, nil
	})

	// Ensure we don't deadlock teardown
	defer close(blockCh)

	// Dispatch task 1 (will be picked up by the single worker and block)
	go wp.Process([]byte("task1"))

	// Small sleep to ensure worker has picked up the task from the queue
	time.Sleep(50 * time.Millisecond)

	// Dispatch task 2 (will go into the queue of size 1)
	go wp.Process([]byte("task2"))

	// Small sleep to ensure task2 is in the queue
	time.Sleep(50 * time.Millisecond)

	// Dispatch task 3 (should fail because queue is full)
	_, err := wp.Process([]byte("task3"))
	if !errors.Is(err, node.ErrQueueFull) {
		t.Fatalf("expected ErrQueueFull, got %v", err)
	}
}

func TestWorkerPoolNode_Teardown(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp3", 2, 5)
	if err := wp.Create(); err != nil {
		t.Fatalf("failed to create worker pool: %v", err)
	}

	blockCh := make(chan struct{})
	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		<-blockCh
		return input, nil
	})

	// Dispatch a task
	resChan := make(chan error, 1)
	go func() {
		_, err := wp.Process([]byte("task1"))
		resChan <- err
	}()

	time.Sleep(50 * time.Millisecond) // Give worker time to pick up task

	// Shut down before unblocking
	go func() {
		wp.Delete()
	}()

	time.Sleep(50 * time.Millisecond) // Give delete time to initiate

	// Unblock worker
	close(blockCh)

	// The task could either succeed (if it finished just before shutdown killed it)
	// or return an error. The key is it shouldn't deadlock or panic.
	err := <-resChan
	if err != nil && err.Error() != "worker pool shut down before processing completed" {
		t.Logf("task failed with expected shutdown error: %v", err)
	}
}
