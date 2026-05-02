package node_test

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestRateLimitMiddleware(t *testing.T) {
	// Setup
	processFunc := func(input []byte) ([]byte, error) {
		return input, nil
	}
	// Capacity of 3 tokens, refill every 50ms
	refillRate := 50 * time.Millisecond
	middleware := node.RateLimitMiddleware(3, refillRate)
	wrappedProcess := middleware(processFunc)

	// Step 1: Burst 3 requests immediately. All should pass.
	for i := 0; i < 3; i++ {
		_, err := wrappedProcess([]byte("burst"))
		if err != nil {
			t.Fatalf("Expected success during initial burst, got: %v", err)
		}
	}

	// Step 2: 4th request immediately should fail (rate limit exceeded)
	_, err := wrappedProcess([]byte("fail"))
	if !errors.Is(err, node.ErrRateLimitExceeded) {
		t.Fatalf("Expected rate limit exceeded error, got: %v", err)
	}

	// Step 3: Wait for a refill
	time.Sleep(refillRate + 10*time.Millisecond) // Wait for 1 token to refill
	_, err = wrappedProcess([]byte("success_after_refill"))
	if err != nil {
		t.Fatalf("Expected success after token refill, got: %v", err)
	}

	// Step 4: Another immediate request should fail again
	_, err = wrappedProcess([]byte("fail_again"))
	if !errors.Is(err, node.ErrRateLimitExceeded) {
		t.Fatalf("Expected rate limit exceeded error after single token used, got: %v", err)
	}
}

func TestRateLimitMiddleware_Concurrency(t *testing.T) {
	processFunc := func(input []byte) ([]byte, error) {
		return input, nil
	}

	// Capacity of 100
	middleware := node.RateLimitMiddleware(100, 10*time.Second) // Refill very slow so it doesn't happen during test
	wrappedProcess := middleware(processFunc)

	var wg sync.WaitGroup
	var successCount int32
	var failCount int32

	// Launch 200 concurrent requests
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := wrappedProcess([]byte("concurrent_test"))
			if err == nil {
				atomic.AddInt32(&successCount, 1)
			} else if errors.Is(err, node.ErrRateLimitExceeded) {
				atomic.AddInt32(&failCount, 1)
			}
		}()
	}

	wg.Wait()

	// Exactly 100 should succeed, and 100 should fail
	if successCount != 100 {
		t.Errorf("Expected exactly 100 successes, got: %d", successCount)
	}
	if failCount != 100 {
		t.Errorf("Expected exactly 100 failures, got: %d", failCount)
	}
}
