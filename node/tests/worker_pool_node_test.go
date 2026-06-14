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
	wp := node.NewWorkerPoolNode("wp-node", 2, 5)
	if err := wp.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	defer cleanupNodes(t, []node.Node{wp})

	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("processed: "), input...), nil
	})

	res, err := wp.Process([]byte("data"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(res) != "processed: data" {
		t.Errorf("expected processed: data, got %s", string(res))
	}
}

func TestWorkerPoolNode_ConcurrencyLimit(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-concurrent", 2, 10)
	if err := wp.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	defer cleanupNodes(t, []node.Node{wp})

	var activeWorkers int32
	var maxActiveWorkers int32
	var mu sync.Mutex

	blockCh := make(chan struct{})

	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		current := atomic.AddInt32(&activeWorkers, 1)

		mu.Lock()
		if current > maxActiveWorkers {
			maxActiveWorkers = current
		}
		mu.Unlock()

		<-blockCh // block until released

		atomic.AddInt32(&activeWorkers, -1)
		return []byte("done"), nil
	})

	// Dispatch 5 jobs concurrently
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = wp.Process([]byte("test"))
		}()
	}

	// Give them time to be picked up
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	maxWorkers := maxActiveWorkers
	mu.Unlock()

	if maxWorkers != 2 {
		t.Errorf("expected max active workers to be 2, got %d", maxWorkers)
	}

	// Release all blocked workers
	close(blockCh)

	// Wait for all jobs to finish
	wg.Wait()
}

func TestWorkerPoolNode_QueueFull(t *testing.T) {
	// 1 worker, queue size 1
	wp := node.NewWorkerPoolNode("wp-queue-full", 1, 1)
	if err := wp.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	blockCh := make(chan struct{})
	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		<-blockCh
		return []byte("done"), nil
	})

	// Job 1: taken by the 1 worker
	go wp.Process([]byte("job1"))

	// Job 2: sits in the queue (size 1)
	time.Sleep(50 * time.Millisecond) // Give worker time to pick up Job 1

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := wp.Process([]byte("job2"))
		if err != nil {
			t.Errorf("expected job2 to succeed, got %v", err)
		}
	}()

	time.Sleep(50 * time.Millisecond) // Give Job 2 time to queue

	// Job 3: should fail immediately with queue full error
	_, err := wp.Process([]byte("job3"))
	if !errors.Is(err, node.ErrWorkerPoolQueueFull) {
		t.Fatalf("expected ErrWorkerPoolQueueFull, got %v", err)
	}

	// Cleanup
	close(blockCh)
	wg.Wait()
	if err := wp.Delete(); err != nil {
		t.Fatalf("failed to delete node: %v", err)
	}
}

func TestWorkerPoolNode_GracefulShutdown(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-shutdown", 1, 5)
	if err := wp.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	blockCh := make(chan struct{})
	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		<-blockCh
		return []byte("done"), nil
	})

	// Dispatch job 1 (taken by worker)
	go wp.Process([]byte("job1"))
	time.Sleep(50 * time.Millisecond)

	// Dispatch job 2 (queued)
	var errJob2 error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, errJob2 = wp.Process([]byte("job2"))
	}()
	time.Sleep(50 * time.Millisecond)

	// Start shutdown
	go func() {
		time.Sleep(50 * time.Millisecond)
		close(blockCh) // unblock worker so it can exit
	}()

	if err := wp.Delete(); err != nil {
		t.Fatalf("failed to delete node: %v", err)
	}
	wg.Wait()

	// Job 2 should have been drained and returned ErrWorkerPoolDeleted
	if !errors.Is(errJob2, node.ErrWorkerPoolDeleted) {
		t.Fatalf("expected queued job to return ErrWorkerPoolDeleted during shutdown, got %v", errJob2)
	}

	// New requests after delete should fail immediately
	_, err := wp.Process([]byte("job3"))
	if !errors.Is(err, node.ErrWorkerPoolDeleted) {
		t.Fatalf("expected ErrWorkerPoolDeleted for new requests after delete, got %v", err)
	}
}
