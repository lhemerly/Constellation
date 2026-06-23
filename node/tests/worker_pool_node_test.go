package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"sync"
	"testing"
	"time"
)

func TestWorkerPoolNode_Success(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp", 2, 10)

	var mu sync.Mutex
	var processed [][]byte

	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		mu.Lock()
		defer mu.Unlock()
		processed = append(processed, input)
		return nil, nil
	})

	if err := wp.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	defer cleanupNodes(t, []node.Node{wp})

	// Submit tasks
	tasks := []string{"A", "B", "C"}
	for _, task := range tasks {
		_, err := wp.Process([]byte(task))
		if err != nil {
			t.Fatalf("unexpected error enqueueing: %v", err)
		}
	}

	// Give workers time to process
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(processed) != 3 {
		t.Errorf("expected 3 processed tasks, got %d", len(processed))
	}
}

func TestWorkerPoolNode_QueueFull(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp", 1, 1)

	// Block the worker so the queue fills up
	// We'll use a wait group instead of blocking channels to avoid tests hanging
	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		// Just sleep a tiny bit to simulate work, but allow it to finish
		time.Sleep(100 * time.Millisecond)
		return nil, nil
	})

	if err := wp.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	defer cleanupNodes(t, []node.Node{wp})

	// 1. First task is pulled by worker immediately and takes 100ms
	_, err := wp.Process([]byte("task1"))
	if err != nil {
		t.Fatalf("unexpected error enqueueing task1: %v", err)
	}

	time.Sleep(10 * time.Millisecond) // Let worker pull it

	// 2. Second task fills the queue of size 1
	_, err = wp.Process([]byte("task2"))
	if err != nil {
		t.Fatalf("unexpected error enqueueing task2: %v", err)
	}

	// 3. We immediately try to submit a third task. Since the queue of size 1 is full,
	// and the worker is busy sleeping, this should immediately fail
	_, err = wp.Process([]byte("task3"))
	if !errors.Is(err, node.ErrQueueFull) {
		t.Fatalf("expected ErrQueueFull, got %v", err)
	}
}
