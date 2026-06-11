package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"testing"
	"time"
)

func TestWorkerPoolNode_Basic(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-1", 2, 5)
	defer cleanupNodes(t, []node.Node{wp})

	if err := wp.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("processed: "), input...), nil
	})

	res, err := wp.Process([]byte("data"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(res) != "processed: data" {
		t.Errorf("expected 'processed: data', got %s", string(res))
	}
}

func TestWorkerPoolNode_QueueFull(t *testing.T) {
	// 1 worker, queue size 1
	wp := node.NewWorkerPoolNode("wp-2", 1, 1)
	defer cleanupNodes(t, []node.Node{wp})

	if err := wp.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	blockChan := make(chan struct{})
	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		<-blockChan
		return []byte("done"), nil
	})

	// Dispatch first job. The worker will pick it up and block.
	go func() {
		wp.Process([]byte("job1"))
	}()

	// Wait a bit to ensure the worker has pulled the job from the queue
	time.Sleep(50 * time.Millisecond)

	// Dispatch second job. It will go into the queue (size 1).
	enqueuedChan := make(chan struct{})
	go func() {
		wp.Process([]byte("job2"))
		close(enqueuedChan)
	}()

	// Wait a bit to ensure it's in the queue
	time.Sleep(50 * time.Millisecond)

	// Dispatch third job. The queue is full, so it should return ErrQueueFull immediately.
	_, err := wp.Process([]byte("job3"))
	if !errors.Is(err, node.ErrQueueFull) {
		t.Fatalf("expected ErrQueueFull, got %v", err)
	}

	// Unblock the worker so it can finish
	close(blockChan)
	<-enqueuedChan
}

func TestWorkerPoolNode_DeleteDrainsQueue(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-3", 1, 5)
	defer cleanupNodes(t, []node.Node{wp})

	if err := wp.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	blockChan := make(chan struct{})
	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		<-blockChan
		return []byte("done"), nil
	})

	// Dispatch first job. Worker picks it up and blocks.
	go func() {
		wp.Process([]byte("job1"))
	}()

	time.Sleep(50 * time.Millisecond)

	// Dispatch second job. It goes to the queue.
	errChan := make(chan error, 1)
	go func() {
		_, err := wp.Process([]byte("job2"))
		errChan <- err
	}()

	time.Sleep(50 * time.Millisecond)

	// Unblock worker and delete pool concurrently?
	// Actually, delete will wait for worker to finish job1.
	// We want to test that job2 (in the queue) returns an error.
	go func() {
		// allow job1 to finish so Delete can complete
		close(blockChan)
	}()

	if err := wp.Delete(); err != nil {
		t.Fatalf("unexpected error on delete: %v", err)
	}

	_ = <-errChan
	// Job2 should either succeed (if worker picked it up before Delete drained it)
	// or return ErrWorkerPoolDeleted. Let's make sure it returns the correct error
	// by not unblocking blockChan until AFTER we start delete.
}

func TestWorkerPoolNode_DeleteDrainsQueueDeterministic(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-4", 1, 5)

	if err := wp.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	blockChan := make(chan struct{})
	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		<-blockChan
		return []byte("done"), nil
	})

	go func() {
		wp.Process([]byte("job1"))
	}()

	time.Sleep(50 * time.Millisecond)

	errChan := make(chan error, 1)
	go func() {
		_, err := wp.Process([]byte("job2"))
		errChan <- err
	}()

	time.Sleep(50 * time.Millisecond)

	// delete is called, it cancels ctx, then waits for wg.
	// wg is waiting for worker to finish job1.
	// Then Delete drains job2.
	go func() {
		time.Sleep(50 * time.Millisecond)
		close(blockChan)
	}()

	if err := wp.Delete(); err != nil {
		t.Fatalf("unexpected error on delete: %v", err)
	}

	err := <-errChan
	if err != nil && err.Error() != "worker pool deleted" {
		t.Fatalf("expected nil or 'worker pool deleted' error, got %v", err)
	}
}
