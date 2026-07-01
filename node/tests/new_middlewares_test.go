package node_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestRateLimitMiddleware(t *testing.T) {
	// Capacity 2, refills every 50ms
	rateLimiter := node.RateLimitMiddleware(2, 50*time.Millisecond)

	processCount := 0
	dummyFunc := func(input []byte) ([]byte, error) {
		processCount++
		return input, nil
	}

	wrapped := rateLimiter(dummyFunc)

	// First two should succeed
	_, err := wrapped([]byte("data"))
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	_, err = wrapped([]byte("data"))
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}

	// Third should fail
	_, err = wrapped([]byte("data"))
	if !errors.Is(err, node.ErrRateLimitExceeded) {
		t.Fatalf("Expected ErrRateLimitExceeded, got: %v", err)
	}

	// Wait for refill
	time.Sleep(60 * time.Millisecond)

	// Should succeed again
	_, err = wrapped([]byte("data"))
	if err != nil {
		t.Fatalf("Expected success after refill, got: %v", err)
	}
}

func TestConcurrencyLimitMiddleware(t *testing.T) {
	concurrencyLimiter := node.ConcurrencyLimitMiddleware(2)

	var wg sync.WaitGroup
	blockCh := make(chan struct{})

	dummyFunc := func(input []byte) ([]byte, error) {
		<-blockCh
		return input, nil
	}

	wrapped := concurrencyLimiter(dummyFunc)

	// Start two concurrent requests that will block
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			_, _ = wrapped([]byte("data"))
		}()
	}

	// Wait a tiny bit to ensure they started and acquired the semaphore
	time.Sleep(10 * time.Millisecond)

	// The third one should fail immediately
	_, err := wrapped([]byte("data"))
	if !errors.Is(err, node.ErrConcurrencyLimitExceeded) {
		t.Fatalf("Expected ErrConcurrencyLimitExceeded, got: %v", err)
	}

	// Unblock
	close(blockCh)
	wg.Wait()
}

func TestFallbackMiddleware(t *testing.T) {
	simulatedErr := errors.New("simulated failure")

	fallbackFunc := func(input []byte, err error) ([]byte, error) {
		if !errors.Is(err, simulatedErr) {
			t.Errorf("Expected simulatedErr in fallback, got: %v", err)
		}
		return []byte("fallback-data"), nil
	}

	fallbackMiddleware := node.FallbackMiddleware(fallbackFunc)

	dummyFunc := func(input []byte) ([]byte, error) {
		return nil, simulatedErr
	}

	wrapped := fallbackMiddleware(dummyFunc)

	output, err := wrapped([]byte("data"))
	if err != nil {
		t.Fatalf("Expected no error from fallback, got: %v", err)
	}

	if string(output) != "fallback-data" {
		t.Errorf("Expected fallback-data, got: %s", string(output))
	}
}

func TestMetricsMiddleware(t *testing.T) {
	metrics := &node.Metrics{}
	metricsMiddleware := node.MetricsMiddleware(metrics)

	dummyFunc := func(input []byte) ([]byte, error) {
		time.Sleep(10 * time.Millisecond)
		if string(input) == "fail" {
			return nil, errors.New("simulated error")
		}
		return input, nil
	}

	wrapped := metricsMiddleware(dummyFunc)

	_, _ = wrapped([]byte("success1"))
	_, _ = wrapped([]byte("success2"))
	_, _ = wrapped([]byte("fail"))

	if metrics.TotalRequests != 3 {
		t.Errorf("Expected 3 TotalRequests, got: %d", metrics.TotalRequests)
	}
	if metrics.SuccessfulRequests != 2 {
		t.Errorf("Expected 2 SuccessfulRequests, got: %d", metrics.SuccessfulRequests)
	}
	if metrics.FailedRequests != 1 {
		t.Errorf("Expected 1 FailedRequests, got: %d", metrics.FailedRequests)
	}
	if metrics.TotalProcessingTime < int64(30*time.Millisecond) {
		t.Errorf("Expected TotalProcessingTime > 30ms, got: %d ns", metrics.TotalProcessingTime)
	}
}
