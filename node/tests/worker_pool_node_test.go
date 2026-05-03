package node_test

import (
	"bytes"
	"github.com/lhemerly/Constellation/node"
	"sync"
	"testing"
)

func TestWorkerPoolNode_BasicProcess(t *testing.T) {
	n, err := node.NewWorkerPoolNode("wp-node", 3)
	if err != nil {
		t.Fatalf("unexpected error creating WorkerPoolNode: %v", err)
	}
	defer cleanupNodes(t, []node.Node{n})

	if err := n.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	n.SetWorkerProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("processed: "), input...), nil
	})

	res, err := n.Process([]byte("data"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(res) != "processed: data" {
		t.Errorf("expected 'processed: data', got '%s'", string(res))
	}
}

func TestWorkerPoolNode_Concurrency(t *testing.T) {
	n, err := node.NewWorkerPoolNode("wp-concurrent", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanupNodes(t, []node.Node{n})

	if err := n.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	n.SetWorkerProcessFunc(func(input []byte) ([]byte, error) {
		return input, nil // simple echo
	})

	var wg sync.WaitGroup
	requests := 100
	results := make([][]byte, requests)
	errs := make([]error, requests)

	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			input := []byte{byte(idx)}
			res, err := n.Process(input)
			results[idx] = res
			errs[idx] = err
		}(i)
	}

	wg.Wait()

	for i := 0; i < requests; i++ {
		if errs[i] != nil {
			t.Errorf("unexpected error on request %d: %v", i, errs[i])
		}
		if len(results[i]) != 1 || results[i][0] != byte(i) {
			t.Errorf("expected result [%d], got %v", i, results[i])
		}
	}
}

func TestWorkerPoolNode_ZeroWorkersError(t *testing.T) {
	_, err := node.NewWorkerPoolNode("wp-zero", 0)
	if err == nil {
		t.Fatal("expected error when creating WorkerPoolNode with 0 workers")
	}
}

func TestWorkerPoolNode_Middlewares(t *testing.T) {
	n, err := node.NewWorkerPoolNode("wp-middleware", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanupNodes(t, []node.Node{n})

	if err := n.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	// The WorkerPoolNode uses poolProcess as baseProcess.
	// Middlewares should wrap poolProcess.
	n.Use(func(next func([]byte) ([]byte, error)) func([]byte) ([]byte, error) {
		return func(input []byte) ([]byte, error) {
			// Prepend before sending to pool
			modifiedInput := append([]byte("mid-in: "), input...)
			out, err := next(modifiedInput)
			if err != nil {
				return nil, err
			}
			// Append after pool processing
			return append([]byte("mid-out: "), out...), nil
		}
	})

	n.SetWorkerProcessFunc(func(input []byte) ([]byte, error) {
		if !bytes.HasPrefix(input, []byte("mid-in: ")) {
			t.Errorf("expected middleware to modify input, got %s", string(input))
		}
		return append([]byte("worker: "), input...), nil
	})

	res, err := n.Process([]byte("data"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "mid-out: worker: mid-in: data"
	if string(res) != expected {
		t.Errorf("expected '%s', got '%s'", expected, string(res))
	}
}
