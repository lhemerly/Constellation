package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"testing"
	"time"
)

func TestWorkerPoolNode_Success(t *testing.T) {
	internalNode := node.NewBaseNode("internal-node")
	internalNode.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("processed "), input...), nil
	})

	wpNode := node.NewWorkerPoolNode("wp-node", 2, 10, internalNode)
	err := wpNode.Create()
	if err != nil {
		t.Fatalf("failed to create WorkerPoolNode: %v", err)
	}
	defer func() {
		_ = wpNode.Delete()
	}()

	res, err := wpNode.Process([]byte("data"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if string(res) != "processed data" {
		t.Fatalf("expected 'processed data', got '%s'", res)
	}
}

func TestWorkerPoolNode_ClosedError(t *testing.T) {
	internalNode := node.NewBaseNode("internal-node")
	wpNode := node.NewWorkerPoolNode("wp-node", 2, 10, internalNode)
	err := wpNode.Create()
	if err != nil {
		t.Fatalf("failed to create WorkerPoolNode: %v", err)
	}

	err = wpNode.Delete()
	if err != nil {
		t.Fatalf("failed to delete WorkerPoolNode: %v", err)
	}

	_, err = wpNode.Process([]byte("data"))
	if !errors.Is(err, node.ErrPoolClosed) {
		t.Fatalf("expected ErrPoolClosed, got %v", err)
	}
}

func TestWorkerPoolNode_Concurrency(t *testing.T) {
	internalNode := node.NewBaseNode("internal-node")
	internalNode.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(10 * time.Millisecond) // Simulate work
		return append([]byte("processed "), input...), nil
	})

	wpNode := node.NewWorkerPoolNode("wp-node", 5, 20, internalNode)
	err := wpNode.Create()
	if err != nil {
		t.Fatalf("failed to create WorkerPoolNode: %v", err)
	}
	defer func() {
		_ = wpNode.Delete()
	}()

	resChan := make(chan struct {
		res []byte
		err error
	}, 10)

	for i := 0; i < 10; i++ {
		go func() {
			res, err := wpNode.Process([]byte("data"))
			resChan <- struct {
				res []byte
				err error
			}{res, err}
		}()
	}

	for i := 0; i < 10; i++ {
		result := <-resChan
		if result.err != nil {
			t.Fatalf("expected no error, got %v", result.err)
		}
		if string(result.res) != "processed data" {
			t.Fatalf("expected 'processed data', got '%s'", result.res)
		}
	}
}
