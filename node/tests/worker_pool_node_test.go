package node_test

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestWorkerPoolNode_Success(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-1", 2, 5)
	if err := wp.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	defer cleanupNodes(t, []node.Node{wp})

	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("processed: "), input...), nil
	})

	out, err := wp.Process([]byte("task1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "processed: task1" {
		t.Errorf("expected 'processed: task1', got '%s'", string(out))
	}
}

func TestWorkerPoolNode_Concurrency(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-2", 2, 10)
	if err := wp.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	defer cleanupNodes(t, []node.Node{wp})

	var activeWorkers int32
	var maxWorkers int32

	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		current := atomic.AddInt32(&activeWorkers, 1)

		for {
			max := atomic.LoadInt32(&maxWorkers)
			if current > max {
				if atomic.CompareAndSwapInt32(&maxWorkers, max, current) {
					break
				}
			} else {
				break
			}
		}

		time.Sleep(50 * time.Millisecond) // Simulate work
		atomic.AddInt32(&activeWorkers, -1)
		return nil, nil
	})

	// Send 5 requests concurrently
	errChan := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func() {
			_, err := wp.Process([]byte("data"))
			errChan <- err
		}()
	}

	for i := 0; i < 5; i++ {
		if err := <-errChan; err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}

	finalMax := atomic.LoadInt32(&maxWorkers)
	if finalMax > 2 {
		t.Errorf("expected max concurrency of 2, got %d", finalMax)
	}
}

func TestWorkerPoolNode_DeleteWhileProcessing(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-3", 1, 10)
	if err := wp.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	// No defer cleanup, we test Delete explicitly

	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(100 * time.Millisecond) // Slow worker
		return nil, nil
	})

	// Start a job
	errChan := make(chan error, 1)
	go func() {
		_, err := wp.Process([]byte("data"))
		errChan <- err
	}()

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	// Queue some more jobs that should get deleted
	for i := 0; i < 2; i++ {
		go func() {
			_, err := wp.Process([]byte("data"))
			if !errors.Is(err, node.ErrWorkerPoolDeleted) {
				t.Errorf("expected ErrWorkerPoolDeleted, got %v", err)
			}
		}()
	}

	time.Sleep(10 * time.Millisecond)

	if err := wp.Delete(); err != nil {
		t.Fatalf("failed to delete node: %v", err)
	}

	err := <-errChan
	if err != nil && !errors.Is(err, node.ErrWorkerPoolDeleted) {
		t.Errorf("unexpected error for first job: %v", err)
	}
}

func TestWorkerPoolNode_DeleteBeforeProcess(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-4", 1, 10)
	if err := wp.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	if err := wp.Delete(); err != nil {
		t.Fatalf("failed to delete node: %v", err)
	}

	_, err := wp.Process([]byte("data"))
	if !errors.Is(err, node.ErrWorkerPoolDeleted) {
		t.Errorf("expected ErrWorkerPoolDeleted, got %v", err)
	}
}
