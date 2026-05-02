package node_test

import (
	"bytes"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestWorkerPoolNode_Basic(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-node", 2)
	if err := wp.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	defer cleanupNodes(t, []node.Node{wp})

	wp.SetWorkerLogic(func(input []byte) ([]byte, error) {
		return append([]byte("processed: "), input...), nil
	})

	output, err := wp.Process([]byte("test"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []byte("processed: test")
	if !bytes.Equal(output, expected) {
		t.Fatalf("expected %s, got %s", expected, output)
	}
}

func TestWorkerPoolNode_Concurrency(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-node", 5) // 5 workers
	if err := wp.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	defer cleanupNodes(t, []node.Node{wp})

	var activeWorkers int32
	var maxActiveWorkers int32

	wp.SetWorkerLogic(func(input []byte) ([]byte, error) {
		current := atomic.AddInt32(&activeWorkers, 1)

		// Track max concurrency
		for {
			max := atomic.LoadInt32(&maxActiveWorkers)
			if current <= max {
				break
			}
			if atomic.CompareAndSwapInt32(&maxActiveWorkers, max, current) {
				break
			}
		}

		time.Sleep(10 * time.Millisecond) // Simulate work
		atomic.AddInt32(&activeWorkers, -1)

		return input, nil
	})

	var wg sync.WaitGroup
	numRequests := 20

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := wp.Process([]byte("test"))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}

	wg.Wait()

	maxCon := atomic.LoadInt32(&maxActiveWorkers)
	if maxCon > 5 {
		t.Fatalf("expected max active workers to be <= 5, got %d", maxCon)
	}
}

func TestWorkerPoolNode_Closed(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-node", 2)
	if err := wp.Create(); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	err := wp.Delete()
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = wp.Process([]byte("test"))
	if !errors.Is(err, node.ErrPoolClosed) {
		t.Fatalf("expected ErrPoolClosed, got %v", err)
	}
}
