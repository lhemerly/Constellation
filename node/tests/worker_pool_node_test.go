package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"sync"
	"testing"
	"time"
)

func TestWorkerPoolNode_Success(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp", 2, 5)
	if err := wp.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	defer cleanupNodes(t, []node.Node{wp})

	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("processed "), input...), nil
	})

	var wg sync.WaitGroup
	results := make([]string, 4)
	errs := make([]error, 4)

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			out, err := wp.Process([]byte("task"))
			results[idx] = string(out)
			errs[idx] = err
		}(i)
	}

	wg.Wait()

	for i := 0; i < 4; i++ {
		if errs[i] != nil {
			t.Errorf("expected success for task %d, got: %v", i, errs[i])
		}
		if results[i] != "processed task" {
			t.Errorf("expected 'processed task', got: %s", results[i])
		}
	}
}

func TestWorkerPoolNode_QueueFull(t *testing.T) {
	// 1 worker, 1 queue size = max 2 concurrent requests before blocking
	wp := node.NewWorkerPoolNode("wp", 1, 1)
	if err := wp.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	defer cleanupNodes(t, []node.Node{wp})

	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(100 * time.Millisecond) // Block the worker
		return input, nil
	})

	// Fill worker
	go wp.Process([]byte("1"))
	time.Sleep(10 * time.Millisecond)

	// Fill queue
	go wp.Process([]byte("2"))
	time.Sleep(10 * time.Millisecond)

	// This should fail
	_, err := wp.Process([]byte("3"))
	if err != node.ErrWorkerPoolFull {
		t.Fatalf("expected ErrWorkerPoolFull, got: %v", err)
	}
}

func TestWorkerPoolNode_Deleted(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp", 1, 1)
	if err := wp.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		return input, nil
	})

	if err := wp.Delete(); err != nil {
		t.Fatalf("failed to delete node: %v", err)
	}

	_, err := wp.Process([]byte("test"))
	if !errors.Is(err, node.ErrWorkerPoolDeleted) {
		t.Fatalf("expected ErrWorkerPoolDeleted, got: %v", err)
	}
}
