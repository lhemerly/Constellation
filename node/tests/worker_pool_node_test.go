package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWorkerPoolNode_Success(t *testing.T) {
	n := node.NewWorkerPoolNode("wp-1", 3, 10)
	defer cleanupNodes(t, []node.Node{n})

	var counter int32
	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		atomic.AddInt32(&counter, 1)
		time.Sleep(10 * time.Millisecond) // Simulate work
		return append([]byte("processed: "), input...), nil
	})

	if err := n.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			input := []byte{byte(i)}
			res, err := n.Process(input)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			expected := append([]byte("processed: "), input...)
			if string(res) != string(expected) {
				t.Errorf("expected %s, got %s", expected, res)
			}
		}(i)
	}

	wg.Wait()

	if atomic.LoadInt32(&counter) != 5 {
		t.Errorf("expected 5 executions, got %d", counter)
	}
}

func TestWorkerPoolNode_QueueFull(t *testing.T) {
	// 1 worker, queue capacity 0.
	// This means the process block immediately waits for worker to pick up.
	n := node.NewWorkerPoolNode("wp-2", 1, 0)
	defer cleanupNodes(t, []node.Node{n})

	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(20 * time.Millisecond)
		return input, nil
	})

	if err := n.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	var wg sync.WaitGroup
	// Send 3 requests. They will block waiting for the single worker.
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := n.Process([]byte("test"))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
}

func TestWorkerPoolNode_Shutdown(t *testing.T) {
	n := node.NewWorkerPoolNode("wp-3", 2, 5)

	if err := n.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(50 * time.Millisecond)
		return input, nil
	})

	// Add some jobs that will block
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := n.Process([]byte("test"))
			if err == nil {
				// We expect some might succeed, some might fail if deleted while in queue
			}
		}()
	}

	// Wait a moment then delete the node
	time.Sleep(10 * time.Millisecond)
	if err := n.Delete(); err != nil {
		t.Fatalf("failed to delete node: %v", err)
	}

	// Sending a job after shutdown should return an error immediately
	_, err := n.Process([]byte("test"))
	if err == nil {
		t.Fatalf("expected error when processing after shutdown, got nil")
	}

	wg.Wait()
}

func TestWorkerPoolNode_ProcessError(t *testing.T) {
	n := node.NewWorkerPoolNode("wp-4", 2, 5)
	defer cleanupNodes(t, []node.Node{n})

	expectedErr := errors.New("test error")
	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, expectedErr
	})

	if err := n.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	_, err := n.Process([]byte("test"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected expectedErr, got %v", err)
	}
}
