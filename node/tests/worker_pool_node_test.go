package node_test

import (
	"bytes"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestWorkerPoolNode_ProcessAndScale(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-1", 5)
	if err := wp.Create(); err != nil {
		t.Fatalf("Failed to create worker pool node: %v", err)
	}
	defer wp.Delete()

	var counter int32
	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		atomic.AddInt32(&counter, 1)
		time.Sleep(10 * time.Millisecond) // Simulate work
		return append([]byte("processed: "), input...), nil
	})

	const numRequests = 20
	resChan := make(chan struct {
		out []byte
		err error
	}, numRequests)

	// Send requests concurrently
	for i := 0; i < numRequests; i++ {
		go func(idx int) {
			input := []byte{byte(idx)}
			out, err := wp.Process(input)
			resChan <- struct {
				out []byte
				err error
			}{out, err}
		}(i)
	}

	for i := 0; i < numRequests; i++ {
		res := <-resChan
		if res.err != nil {
			t.Errorf("Unexpected error: %v", res.err)
		}
		if !bytes.HasPrefix(res.out, []byte("processed: ")) {
			t.Errorf("Unexpected output: %s", string(res.out))
		}
	}

	if atomic.LoadInt32(&counter) != int32(numRequests) {
		t.Errorf("Expected %d requests processed, got %d", numRequests, counter)
	}
}

func TestWorkerPoolNode_ShutdownDrain(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-shutdown", 2)
	if err := wp.Create(); err != nil {
		t.Fatalf("Failed to create node: %v", err)
	}

	blockCh := make(chan struct{})
	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		<-blockCh // block the worker to simulate long task and allow queue to build up
		return input, nil
	})

	// Fill the pool and queue
	for i := 0; i < 4; i++ {
		go wp.Process([]byte("test"))
	}

	// Small delay to ensure they are queued/running
	time.Sleep(50 * time.Millisecond)

	errChan := make(chan error, 1)
	go func() {
		errChan <- wp.Delete()
	}()

	// The delete will block until the workers are done. Let's unblock them.
	close(blockCh)

	select {
	case err := <-errChan:
		if err != nil {
			t.Errorf("Unexpected error during delete: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Delete timed out, probably blocked on workers")
	}

	// Ensure new processes are rejected after shutdown
	_, err := wp.Process([]byte("post-shutdown"))
	if err == nil {
		t.Fatal("Expected error processing after shutdown, got nil")
	}
}

func TestWorkerPoolNode_WorkerError(t *testing.T) {
	wp := node.NewWorkerPoolNode("wp-err", 1)
	if err := wp.Create(); err != nil {
		t.Fatalf("Failed to create node: %v", err)
	}
	defer wp.Delete()

	expectedErr := errors.New("worker error")
	wp.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, expectedErr
	})

	_, err := wp.Process([]byte("test"))
	if err != expectedErr {
		t.Fatalf("Expected error %v, got %v", expectedErr, err)
	}
}
