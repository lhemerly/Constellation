package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"sync"
	"testing"
	"time"
)

func TestWorkerPoolNode(t *testing.T) {
	target := node.NewBaseNode("target")

	// Fast target
	target.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("processed: "), input...), nil
	})

	wp := node.NewWorkerPoolNode("wp", target, 2, 2)
	defer cleanupNodes(t, []node.Node{wp})

	if err := wp.Create(); err != nil {
		t.Fatalf("unexpected error on create: %v", err)
	}

	res, err := wp.Process([]byte("data"))
	if err != nil {
		t.Fatalf("unexpected error on process: %v", err)
	}
	if string(res) != "processed: data" {
		t.Errorf("expected 'processed: data', got '%s'", string(res))
	}
}

func TestWorkerPoolNode_LoadShedding(t *testing.T) {
	target := node.NewBaseNode("slow-target")

	// Slow target to block workers
	target.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(100 * time.Millisecond)
		return []byte("done"), nil
	})

	// Pool size 1, Queue size 1
	// Concurrency: 1 processing, 1 queued. 3rd should fail immediately.
	wp := node.NewWorkerPoolNode("wp-shed", target, 1, 1)
	defer cleanupNodes(t, []node.Node{wp})

	if err := wp.Create(); err != nil {
		t.Fatalf("unexpected error on create: %v", err)
	}

	errs := make(chan error, 3)
	var wg sync.WaitGroup

	// 1st request should be picked up by the worker and block for 100ms
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := wp.Process([]byte("data"))
		if err != nil {
			errs <- err
		}
	}()

	// Wait a bit to ensure the worker has picked up the first request
	time.Sleep(10 * time.Millisecond)

	// 2nd request should fill the queue (queue size 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := wp.Process([]byte("data"))
		if err != nil {
			errs <- err
		}
	}()

	// Wait a tiny bit to ensure the second request is in the queue
	time.Sleep(10 * time.Millisecond)

	// 3rd request should fail immediately with ErrQueueFull
	_, err := wp.Process([]byte("data"))
	if err != nil {
		errs <- err
	}

	// Wait for all requests to finish processing
	wg.Wait()
	close(errs)

	// We only expect 1 ErrQueueFull from the 3rd request
	var queueFullErrors int

	for err := range errs {
		if errors.Is(err, node.ErrQueueFull) {
			queueFullErrors++
		}
	}

	if queueFullErrors != 1 {
		t.Errorf("expected 1 ErrQueueFull error, got %d", queueFullErrors)
	}
}
