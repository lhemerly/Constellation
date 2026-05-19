package node_test

import (
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestWorkerPoolNode_ConcurrencyLimit(t *testing.T) {
	var currentWorkers int32
	var maxWorkers int32

	worker := node.NewBaseNode("worker")
	worker.SetProcessFunc(func(input []byte) ([]byte, error) {
		current := atomic.AddInt32(&currentWorkers, 1)

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

		atomic.AddInt32(&currentWorkers, -1)
		return []byte("done-" + string(input)), nil
	})

	poolSize := 3
	poolNode := node.NewWorkerPoolNode("pool", worker, poolSize)

	if err := poolNode.Create(); err != nil {
		t.Fatalf("failed to create WorkerPoolNode: %v", err)
	}
	defer cleanupNodes(t, []node.Node{poolNode})

	numRequests := 10
	var wg sync.WaitGroup

	// Send burst of requests
	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			res, err := poolNode.Process([]byte(strconv.Itoa(id)))
			if err != nil {
				t.Errorf("request %d failed: %v", id, err)
			}
			expectedRes := "done-" + strconv.Itoa(id)
			if string(res) != expectedRes {
				t.Errorf("expected %s, got %s", expectedRes, res)
			}
		}(i)
	}

	wg.Wait()

	max := atomic.LoadInt32(&maxWorkers)
	if max > int32(poolSize) {
		t.Errorf("expected max concurrent workers to be <= %d, got %d", poolSize, max)
	}
	if max == 0 {
		t.Errorf("expected max concurrent workers > 0")
	}
}
