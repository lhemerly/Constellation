package node_test

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
	"github.com/stretchr/testify/assert"
)

func TestWorkerPoolNode_Concurrency(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-1", 2)

	var activeWorkers int32
	var maxActiveWorkers int32

	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		current := atomic.AddInt32(&activeWorkers, 1)

		// Track max concurrency
		for {
			max := atomic.LoadInt32(&maxActiveWorkers)
			if current > max {
				if atomic.CompareAndSwapInt32(&maxActiveWorkers, max, current) {
					break
				}
			} else {
				break
			}
		}

		time.Sleep(50 * time.Millisecond)
		atomic.AddInt32(&activeWorkers, -1)
		return []byte(fmt.Sprintf("processed:%s", input)), nil
	})

	err := wp.Create()
	assert.NoError(t, err)

	var wg sync.WaitGroup
	// Send 5 concurrent requests
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			res, err := wp.Process([]byte(fmt.Sprintf("%d", idx)))
			assert.NoError(t, err)
			assert.Equal(t, fmt.Sprintf("processed:%d", idx), string(res))
		}(i)
	}

	wg.Wait()

	// Since we only have 2 workers, max concurrency should be 2
	assert.Equal(t, int32(2), atomic.LoadInt32(&maxActiveWorkers))

	err = wp.Delete()
	assert.NoError(t, err)
}

func TestWorkerPoolNode_Deleted(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-2", 1)
	err := wp.Create()
	assert.NoError(t, err)

	// Process one message successfully
	_, err = wp.Process([]byte("test"))
	assert.NoError(t, err)

	// Delete the node
	err = wp.Delete()
	assert.NoError(t, err)

	// Processing after deletion should fail
	_, err = wp.Process([]byte("test"))
	assert.Error(t, err)
	assert.True(t, errors.Is(err, node.ErrWorkerPoolDeleted))
}
