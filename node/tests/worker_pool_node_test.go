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
	pool := node.NewWorkerPoolNode("wp-1", 2)
	defer cleanupNodes(t, []node.Node{pool})

	if err := pool.Create(); err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}

	pool.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("processed: "), input...), nil
	})

	res, err := pool.Process([]byte("data"))
	if err != nil {
		t.Fatalf("unexpected process error: %v", err)
	}
	if string(res) != "processed: data" {
		t.Errorf("expected 'processed: data', got '%s'", string(res))
	}
}

func TestWorkerPoolNode_Concurrency(t *testing.T) {
	numWorkers := 3
	pool := node.NewWorkerPoolNode("wp-concurrent", numWorkers)
	defer cleanupNodes(t, []node.Node{pool})

	if err := pool.Create(); err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}

	var activeWorkers int32
	var maxWorkers int32
	var mu sync.Mutex

	pool.SetProcessFunc(func(input []byte) ([]byte, error) {
		current := atomic.AddInt32(&activeWorkers, 1)

		mu.Lock()
		if current > maxWorkers {
			maxWorkers = current
		}
		mu.Unlock()

		time.Sleep(10 * time.Millisecond) // Simulate work

		atomic.AddInt32(&activeWorkers, -1)
		return input, nil
	})

	var wg sync.WaitGroup
	numTasks := 10
	errs := make([]error, numTasks)

	for i := 0; i < numTasks; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := pool.Process([]byte("data"))
			if err != nil {
				errs[idx] = err
			}
		}(i)
	}

	wg.Wait()

	for _, err := range errs {
		if err != nil {
			t.Errorf("unexpected error during concurrent process: %v", err)
		}
	}

	mu.Lock()
	if maxWorkers > int32(numWorkers) {
		t.Errorf("exceeded max workers limit. expected %d, got %d", numWorkers, maxWorkers)
	}
	if maxWorkers == 0 {
		t.Errorf("no workers were utilized")
	}
	mu.Unlock()
}

func TestWorkerPoolNode_ClosedError(t *testing.T) {
	pool := node.NewWorkerPoolNode("wp-closed", 1)

	if err := pool.Create(); err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}

	if err := pool.Delete(); err != nil {
		t.Fatalf("unexpected delete error: %v", err)
	}

	_, err := pool.Process([]byte("data"))
	if !errors.Is(err, node.ErrWorkerPoolClosed) {
		t.Errorf("expected ErrWorkerPoolClosed, got %v", err)
	}
}
