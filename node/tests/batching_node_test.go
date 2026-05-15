package node_test

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestBatchingNode_NoTarget(t *testing.T) {
	bn := node.NewBatchingNode("batching-1", 2, 100*time.Millisecond)
	defer bn.Delete()

	_, err := bn.Process([]byte("test"))
	if !errors.Is(err, node.ErrBatchingNodeNoTarget) {
		t.Errorf("expected ErrBatchingNodeNoTarget, got %v", err)
	}
}

func TestBatchingNode_BatchSizeTrigger(t *testing.T) {
	bn := node.NewBatchingNode("batching-1", 3, 100*time.Millisecond)
	target := node.NewBaseNode("target-1")
	defer cleanupNodes(t, []node.Node{bn, target})

	var callCount int32
	target.SetProcessFunc(func(input []byte) ([]byte, error) {
		atomic.AddInt32(&callCount, 1)
		return append([]byte("processed: "), input...), nil
	})

	bn.SetTarget(target)

	var wg sync.WaitGroup
	results := make([]string, 3)

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			res, err := bn.Process([]byte("a"))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			results[idx] = string(res)
		}(i)
	}

	wg.Wait()

	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("expected target to be called exactly 1 time, got %d", callCount)
	}

	expectedRes := "processed: aaa"
	for i, res := range results {
		if res != expectedRes {
			t.Errorf("expected result %d to be %q, got %q", i, expectedRes, res)
		}
	}
}

func TestBatchingNode_TimeoutTrigger(t *testing.T) {
	bn := node.NewBatchingNode("batching-1", 10, 50*time.Millisecond)
	target := node.NewBaseNode("target-1")
	defer cleanupNodes(t, []node.Node{bn, target})

	var callCount int32
	target.SetProcessFunc(func(input []byte) ([]byte, error) {
		atomic.AddInt32(&callCount, 1)
		return append([]byte("processed: "), input...), nil
	})

	bn.SetTarget(target)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		res, err := bn.Process([]byte("a"))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if string(res) != "processed: a" {
			t.Errorf("expected 'processed: a', got %s", string(res))
		}
	}()

	wg.Wait()

	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("expected target to be called exactly 1 time, got %d", callCount)
	}
}

func TestBatchingNode_DeleteInterruptsWaiters(t *testing.T) {
	bn := node.NewBatchingNode("batching-1", 10, 5*time.Second)
	target := node.NewBaseNode("target-1")
	bn.SetTarget(target)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := bn.Process([]byte("a"))
		if err == nil {
			t.Errorf("expected error due to deletion, got nil")
		}
	}()

	// wait a bit for the item to be queued
	time.Sleep(10 * time.Millisecond)

	bn.Delete() // this should interrupt the waiting Process
	wg.Wait()
}
