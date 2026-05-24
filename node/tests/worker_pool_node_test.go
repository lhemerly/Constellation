package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"sync"
	"testing"
	"time"
)

func TestWorkerPoolNode_Success(t *testing.T) {
	n := node.NewWorkerPoolNode("wp-1", 3, 10)
	defer cleanupNodes(t, []node.Node{n})

	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("processed: "), input...), nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			input := []byte{byte(idx)}
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
}

func TestWorkerPoolNode_Middlewares(t *testing.T) {
	n := node.NewWorkerPoolNode("wp-2", 2, 5)
	defer cleanupNodes(t, []node.Node{n})

	n.Use(node.TimeoutMiddleware(50 * time.Millisecond))

	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(100 * time.Millisecond) // Should trigger timeout middleware
		return input, nil
	})

	_, err := n.Process([]byte("test"))
	if !errors.Is(err, node.ErrProcessTimeout) {
		t.Errorf("expected timeout error, got %v", err)
	}
}

func TestWorkerPoolNode_DeleteStopsWorkers(t *testing.T) {
	n := node.NewWorkerPoolNode("wp-3", 2, 5)

	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(50 * time.Millisecond)
		return input, nil
	})

	// Fill the buffer a bit
	for i := 0; i < 3; i++ {
		go n.Process([]byte("test"))
	}

	err := n.Delete()
	if err != nil {
		t.Fatalf("unexpected error on delete: %v", err)
	}

	// Try processing after delete
	_, err = n.Process([]byte("test2"))
	if !errors.Is(err, node.ErrWorkerPoolClosed) {
		t.Errorf("expected worker pool closed error, got %v", err)
	}
}
