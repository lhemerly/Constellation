package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWorkerPoolNode_Basic(t *testing.T) {
	target := node.NewBaseNode("target")
	target.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("processed: "), input...), nil
	})

	wp := node.NewWorkerPoolNode("wp", target, 2, 5)
	defer cleanupNodes(t, []node.Node{wp})

	if err := wp.Create(); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	res, err := wp.Process([]byte("hello"))
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if string(res) != "processed: hello" {
		t.Errorf("Process() got = %s, want %s", res, "processed: hello")
	}
}

func TestWorkerPoolNode_ConcurrencyAndBackpressure(t *testing.T) {
	target := node.NewBaseNode("target")

	// Create a target that blocks to simulate slow processing
	blockCh := make(chan struct{})
	var activeWorkers int32

	target.SetProcessFunc(func(input []byte) ([]byte, error) {
		atomic.AddInt32(&activeWorkers, 1)
		defer atomic.AddInt32(&activeWorkers, -1)
		<-blockCh
		return []byte("done"), nil
	})

	// Concurrency 2, Queue Size 1
	wp := node.NewWorkerPoolNode("wp", target, 2, 1)
	defer cleanupNodes(t, []node.Node{wp})

	if err := wp.Create(); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 10)

	// Fire 4 concurrent requests
	// Req 1 & 2 will be processed by the 2 workers (and block)
	// Req 3 will be queued (queue size 1)
	// Req 4 should hit backpressure (ErrQueueFull)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := wp.Process([]byte("req"))
			if err != nil {
				errs <- err
			}
		}()
	}

	// Give goroutines time to spin up and hit the queue/workers
	time.Sleep(100 * time.Millisecond)

	// Wait a moment for queues to fill and requests to hit backpressure
	time.Sleep(50 * time.Millisecond)

	// Verify concurrency limit
	if val := atomic.LoadInt32(&activeWorkers); val != 2 {
		t.Errorf("expected 2 active workers, got %d", val)
	}

	// Verify we got backpressure error(s)
	close(errs)
	var fullErrs int
	for err := range errs {
		if errors.Is(err, node.ErrQueueFull) {
			fullErrs++
		} else {
			t.Errorf("unexpected error: %v", err)
		}
	}

	if fullErrs == 0 {
		t.Errorf("expected at least 1 ErrQueueFull, got %d", fullErrs)
	}

	// Unblock workers
	close(blockCh)
	wg.Wait()
}

func TestWorkerPoolNode_Delete(t *testing.T) {
	target := node.NewBaseNode("target")
	blockCh := make(chan struct{})
	target.SetProcessFunc(func(input []byte) ([]byte, error) {
		<-blockCh
		return []byte("done"), nil
	})

	wp := node.NewWorkerPoolNode("wp", target, 1, 10)
	if err := wp.Create(); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Fire request that blocks
	var err1 error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err1 = wp.Process([]byte("req"))
	}()

	// Enqueue another job
	wg.Add(1)
	var err2 error
	go func() {
		defer wg.Done()
		_, err2 = wp.Process([]byte("queued"))
	}()

	time.Sleep(50 * time.Millisecond)

	// Unblock the processing worker asynchronously so Delete() can wait for workers
	go func() {
		time.Sleep(50 * time.Millisecond)
		close(blockCh)
	}()

	// Delete while processing and queued
	if err := wp.Delete(); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	wg.Wait()

	// The first job might complete successfully or fail depending on exact timing.
	// But the queued job should definitely fail with "worker pool deleted".
	if err2 == nil || err2.Error() != "worker pool deleted" {
		t.Errorf("expected 'worker pool deleted' for queued job, got %v", err2)
	}
	_ = err1
}

func TestWorkerPoolNode_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()

	target := node.NewBaseNode("target")
	_ = node.NewWorkerPoolNode("wp", target, 0, 10)
}
