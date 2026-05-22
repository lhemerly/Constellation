package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWorkerPoolNode_BasicProcessing(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-1", 3)
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
		t.Errorf("expected 'processed: data', got '%s'", string(res))
	}
}

func TestWorkerPoolNode_Concurrency(t *testing.T) {
	poolSize := 5
	wp := node.NewWorkerPoolNode("wp-concurrency", poolSize)
	defer cleanupNodes(t, []node.Node{wp})

	if err := wp.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	var activeWorkers int32
	var maxActiveWorkers int32

	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		current := atomic.AddInt32(&activeWorkers, 1)
		defer atomic.AddInt32(&activeWorkers, -1)

		// Record the max concurrently active workers observed
		for {
			max := atomic.LoadInt32(&maxActiveWorkers)
			if current <= max || atomic.CompareAndSwapInt32(&maxActiveWorkers, max, current) {
				break
			}
		}

		time.Sleep(10 * time.Millisecond) // Simulate some work
		return input, nil
	})

	const numRequests = 20
	var wg sync.WaitGroup
	wg.Add(numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			defer wg.Done()
			_, err := wp.Process([]byte("work"))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}

	wg.Wait()

	maxObserved := atomic.LoadInt32(&maxActiveWorkers)
	if maxObserved > int32(poolSize) {
		t.Errorf("observed %d concurrent workers, which exceeds pool size of %d", maxObserved, poolSize)
	}
	if maxObserved == 0 {
		t.Errorf("expected at least 1 active worker, observed 0")
	}
}

func TestWorkerPoolNode_ErrorHandling(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-error", 2)
	defer cleanupNodes(t, []node.Node{wp})

	if err := wp.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	expectedErr := errors.New("worker error")
	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, expectedErr
	})

	_, err := wp.Process([]byte("data"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error to be '%v', got '%v'", expectedErr, err)
	}
}

func TestWorkerPoolNode_ProcessBeforeCreate(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-not-created", 2)
	defer cleanupNodes(t, []node.Node{wp})

	// Do NOT call Create()

	_, err := wp.Process([]byte("data"))
	if err == nil {
		t.Fatalf("expected error when processing before Create, got nil")
	}
	if err.Error() != "worker pool is not created" {
		t.Errorf("expected 'worker pool is not created' error, got: %v", err)
	}
}

func TestWorkerPoolNode_ZeroPoolSizePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic when pool size is <= 0")
		}
	}()

	node.NewWorkerPoolNode("wp-panic", 0)
}
