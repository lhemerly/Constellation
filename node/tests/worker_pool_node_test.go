package node_test

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestWorkerPoolNode_CreateDelete(t *testing.T) {
	pool := node.NewWorkerPoolNode("pool-1", 2)
	defer cleanupNodes(t, []node.Node{pool})

	if pool.GetID() != "pool-1" {
		t.Errorf("expected ID pool-1, got %s", pool.GetID())
	}

	if err := pool.Create(); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := pool.Delete(); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}

func TestWorkerPoolNode_DeleteTwice(t *testing.T) {
	pool := node.NewWorkerPoolNode("pool-1", 2)
	defer cleanupNodes(t, []node.Node{pool})

	if err := pool.Create(); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := pool.Delete(); err != nil {
		t.Fatalf("first Delete() error = %v", err)
	}

	if err := pool.Delete(); err != nil {
		t.Fatalf("second Delete() error = %v", err)
	}
}

func TestWorkerPoolNode_Process(t *testing.T) {
	pool := node.NewWorkerPoolNode("pool-1", 2)
	defer cleanupNodes(t, []node.Node{pool})

	pool.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte(fmt.Sprintf("processed-%s", string(input))), nil
	})

	if err := pool.Create(); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	result, err := pool.Process([]byte("test"))
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if string(result) != "processed-test" {
		t.Errorf("expected 'processed-test', got '%s'", string(result))
	}
}

func TestWorkerPoolNode_ProcessAfterDelete(t *testing.T) {
	pool := node.NewWorkerPoolNode("pool-1", 2)
	defer cleanupNodes(t, []node.Node{pool})

	if err := pool.Create(); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := pool.Delete(); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err := pool.Process([]byte("test"))
	if err == nil {
		t.Fatalf("expected error processing after delete")
	}
	if !errors.Is(err, node.ErrWorkerPoolDeleted) {
		t.Errorf("expected ErrWorkerPoolDeleted, got %v", err)
	}
}

func TestWorkerPoolNode_ConcurrentProcessing(t *testing.T) {
	numWorkers := 3
	pool := node.NewWorkerPoolNode("pool-1", numWorkers)
	defer cleanupNodes(t, []node.Node{pool})

	var counter int32
	pool.SetProcessFunc(func(input []byte) ([]byte, error) {
		atomic.AddInt32(&counter, 1)
		time.Sleep(10 * time.Millisecond) // Simulate work
		return []byte(strings.ToUpper(string(input))), nil
	})

	if err := pool.Create(); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	numRequests := 10
	var wg sync.WaitGroup
	wg.Add(numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			defer wg.Done()
			input := fmt.Sprintf("req-%d", id)
			result, err := pool.Process([]byte(input))
			if err != nil {
				t.Errorf("Process() error = %v", err)
			}
			expected := strings.ToUpper(input)
			if string(result) != expected {
				t.Errorf("expected %s, got %s", expected, string(result))
			}
		}(i)
	}

	wg.Wait()

	if int(counter) != numRequests {
		t.Errorf("expected %d processes, got %d", numRequests, counter)
	}
}
