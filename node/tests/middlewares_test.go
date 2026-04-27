package node_test

import (
	"bytes"
	"errors"
	"github.com/lhemerly/Constellation/node"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMiddlewares_Logging(t *testing.T) {
	n := node.NewBaseNode("log-node")
	defer cleanupNodes(t, []node.Node{n})

	n.Use(node.LoggingMiddleware)

	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("logged"), nil
	})

	res, err := n.Process([]byte("input"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(res) != "logged" {
		t.Errorf("expected logged, got %s", string(res))
	}
}

func TestMiddlewares_Retry_Success(t *testing.T) {
	n := node.NewBaseNode("retry-node")
	defer cleanupNodes(t, []node.Node{n})

	n.Use(node.RetryMiddleware(3, 10*time.Millisecond))

	attempts := 0
	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		attempts++
		if attempts < 3 {
			return nil, errors.New("temporary error")
		}
		return []byte("success"), nil
	})

	res, err := n.Process([]byte("input"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(res) != "success" {
		t.Errorf("expected success, got %s", string(res))
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestMiddlewares_Retry_Failure(t *testing.T) {
	n := node.NewBaseNode("retry-fail-node")
	defer cleanupNodes(t, []node.Node{n})

	n.Use(node.RetryMiddleware(2, 5*time.Millisecond))

	attempts := 0
	expectedErr := errors.New("persistent error")
	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		attempts++
		return nil, expectedErr
	})

	_, err := n.Process([]byte("input"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if attempts != 3 { // initial + 2 retries
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error to wrap expectedErr")
	}
}

func TestMiddlewares_Recovery(t *testing.T) {
	n := node.NewBaseNode("recovery-node")
	defer cleanupNodes(t, []node.Node{n})

	n.Use(node.RecoveryMiddleware)

	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		panic("test panic")
	})

	_, err := n.Process([]byte("input"))
	if err == nil {
		t.Fatalf("expected error from panic recovery, got nil")
	}
	if err.Error() != "test panic" {
		t.Errorf("expected 'test panic', got '%v'", err.Error())
	}
}

func TestMiddlewares_CircuitBreaker(t *testing.T) {
	n := node.NewBaseNode("cb-node")
	defer cleanupNodes(t, []node.Node{n})

	cooldown := 50 * time.Millisecond
	n.Use(node.CircuitBreakerMiddleware(2, cooldown))

	var fail bool
	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		if fail {
			return nil, errors.New("simulated error")
		}
		return []byte("success"), nil
	})

	// 1. Initially successful requests
	res, err := n.Process([]byte("input1"))
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if string(res) != "success" {
		t.Errorf("expected success, got %s", string(res))
	}

	// 2. Trigger failures
	fail = true
	_, err = n.Process([]byte("input2")) // Failure 1
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	_, err = n.Process([]byte("input3")) // Failure 2 (Open circuit)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	// 3. Circuit is open, should return ErrCircuitBreakerOpen immediately
	_, err = n.Process([]byte("input4"))
	if !errors.Is(err, node.ErrCircuitBreakerOpen) {
		t.Fatalf("expected ErrCircuitBreakerOpen, got %v", err)
	}

	// 4. Wait for cooldown to allow half-open state
	time.Sleep(cooldown + 10*time.Millisecond)

	// 5. Half-open test: we still fail, which re-opens the circuit
	_, err = n.Process([]byte("input5")) // Test failure
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	// 6. Circuit should be open again
	_, err = n.Process([]byte("input6"))
	if !errors.Is(err, node.ErrCircuitBreakerOpen) {
		t.Fatalf("expected ErrCircuitBreakerOpen, got %v", err)
	}

	// 7. Wait for cooldown again
	time.Sleep(cooldown + 10*time.Millisecond)

	// 8. Half-open test: we succeed, closing the circuit
	fail = false
	res, err = n.Process([]byte("input7"))
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if string(res) != "success" {
		t.Errorf("expected success, got %s", string(res))
	}

	// 9. Circuit is closed, subsequent request succeeds
	res, err = n.Process([]byte("input8"))
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if string(res) != "success" {
		t.Errorf("expected success, got %s", string(res))
	}
}

func TestMiddlewares_Cache(t *testing.T) {
	n := node.NewBaseNode("cache-node")
	defer cleanupNodes(t, []node.Node{n})

	ttl := 100 * time.Millisecond
	n.Use(node.CacheMiddleware(ttl))

	var executions int
	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		executions++
		return append([]byte("processed: "), input...), nil
	})

	// 1. Initial process
	res, err := n.Process([]byte("data"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(res) != "processed: data" {
		t.Errorf("expected processed: data, got %s", string(res))
	}
	if executions != 1 {
		t.Fatalf("expected 1 execution, got %d", executions)
	}

	// 2. Second process with same input (should be cached)
	res, err = n.Process([]byte("data"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(res) != "processed: data" {
		t.Errorf("expected processed: data, got %s", string(res))
	}
	if executions != 1 {
		t.Fatalf("expected executions to remain 1, got %d", executions)
	}

	// 3. Process with different input
	res, err = n.Process([]byte("other"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(res) != "processed: other" {
		t.Errorf("expected processed: other, got %s", string(res))
	}
	if executions != 2 {
		t.Fatalf("expected 2 executions, got %d", executions)
	}

	// 4. Wait for TTL to expire on first input
	time.Sleep(ttl + 10*time.Millisecond)

	// 5. Process first input again, should trigger a new execution
	res, err = n.Process([]byte("data"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(res) != "processed: data" {
		t.Errorf("expected processed: data, got %s", string(res))
	}
	if executions != 3 {
		t.Fatalf("expected 3 executions, got %d", executions)
	}
}

func TestMiddlewares_Timeout(t *testing.T) {
	n := node.NewBaseNode("timeout-node")
	defer cleanupNodes(t, []node.Node{n})

	n.Use(node.TimeoutMiddleware(50 * time.Millisecond))

	// Test 1: Successful fast execution
	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("fast"), nil
	})
	res, err := n.Process([]byte("input"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(res) != "fast" {
		t.Errorf("expected 'fast', got '%s'", string(res))
	}

	// Test 2: Slow execution resulting in timeout
	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(100 * time.Millisecond)
		return []byte("slow"), nil
	})
	_, err = n.Process([]byte("input"))
	if !errors.Is(err, node.ErrProcessTimeout) {
		t.Fatalf("expected ErrProcessTimeout, got %v", err)
	}
}

func TestMiddlewares_CircuitBreaker_EmptyInput(t *testing.T) {
	n := node.NewBaseNode("cb-empty-node")
	defer cleanupNodes(t, []node.Node{n})

	cooldown := 50 * time.Millisecond
	n.Use(node.CircuitBreakerMiddleware(2, cooldown))

	var fail bool
	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		if fail {
			return nil, errors.New("simulated error")
		}
		if len(input) != 0 {
			t.Errorf("expected empty input, got %d bytes", len(input))
		}
		return []byte("success"), nil
	})

	// 1. Initially successful requests with empty input
	res, err := n.Process([]byte{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if string(res) != "success" {
		t.Errorf("expected success, got %s", string(res))
	}

	// 2. Trigger failures with empty input
	fail = true
	_, err = n.Process([]byte{}) // Failure 1
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	_, err = n.Process([]byte{}) // Failure 2 (Open circuit)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	// 3. Circuit is open, should return ErrCircuitBreakerOpen immediately for empty input
	_, err = n.Process([]byte{})
	if !errors.Is(err, node.ErrCircuitBreakerOpen) {
		t.Fatalf("expected ErrCircuitBreakerOpen, got %v", err)
	}
}

func TestMiddlewares_Filter(t *testing.T) {
	n := node.NewBaseNode("filter-node")
	defer cleanupNodes(t, []node.Node{n})

	n.Use(node.FilterMiddleware(func(input []byte) bool {
		return string(input) == "valid"
	}))

	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("processed"), nil
	})

	// Valid input
	res, err := n.Process([]byte("valid"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(res) != "processed" {
		t.Errorf("expected processed, got %s", string(res))
	}

	// Invalid input
	res, err = n.Process([]byte("invalid"))
	if !errors.Is(err, node.ErrFiltered) {
		t.Fatalf("expected ErrFiltered, got %v", err)
	}
	if res != nil {
		t.Errorf("expected nil result, got %s", string(res))
	}
}

func TestMiddlewares_ConcurrencyLimit(t *testing.T) {
	n := node.NewBaseNode("concurrency-limit-node")
	defer cleanupNodes(t, []node.Node{n})

	n.Use(node.ConcurrencyLimitMiddleware(2))

	var currentConcurrent int32
	var maxObservedConcurrent int32

	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		// Increment concurrent counter
		current := atomic.AddInt32(&currentConcurrent, 1)
		defer atomic.AddInt32(&currentConcurrent, -1)

		// Record max observed
		for {
			maxObserved := atomic.LoadInt32(&maxObservedConcurrent)
			if current <= maxObserved {
				break
			}
			if atomic.CompareAndSwapInt32(&maxObservedConcurrent, maxObserved, current) {
				break
			}
		}

		time.Sleep(10 * time.Millisecond) // Simulate work
		return []byte("done"), nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = n.Process([]byte("input"))
		}()
	}
	wg.Wait()

	if maxObservedConcurrent > 2 {
		t.Errorf("expected max concurrency to be 2 or less, got %d", maxObservedConcurrent)
	}
}

func TestMiddlewares_RateLimit(t *testing.T) {
	n := node.NewBaseNode("rate-limit-node")
	defer cleanupNodes(t, []node.Node{n})

	// 10 requests per second
	n.Use(node.RateLimitMiddleware(10))

	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("ok"), nil
	})

	// Consume all 10 tokens immediately
	for i := 0; i < 10; i++ {
		_, err := n.Process([]byte("req"))
		if err != nil {
			t.Fatalf("unexpected error on request %d: %v", i, err)
		}
	}

	// The 11th request should hit the rate limit
	_, err := n.Process([]byte("req"))
	if !errors.Is(err, node.ErrRateLimitExceeded) {
		t.Fatalf("expected ErrRateLimitExceeded, got %v", err)
	}
}

func TestMiddlewares_Compression(t *testing.T) {
	n := node.NewBaseNode("compression-node")
	defer cleanupNodes(t, []node.Node{n})

	n.Use(node.CompressionMiddleware())

	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		// The input should be decompressed
		if string(input) != "raw data" {
			t.Errorf("expected decompressed 'raw data', got '%s'", string(input))
		}
		return []byte("processed data"), nil
	})

	// Process without compression first, middleware should pass it as is (since it's not valid gzip)
	res1, err := n.Process([]byte("raw data"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Output should be compressed gzip
	if bytes.HasPrefix(res1, []byte("processed data")) {
		t.Fatalf("expected compressed output, got plain text")
	}

	// Now pass the compressed output back in
	// CompressionMiddleware should decompress it
	n2 := node.NewBaseNode("compression-node-2")
	defer cleanupNodes(t, []node.Node{n2})
	n2.Use(node.CompressionMiddleware())
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		if string(input) != "processed data" {
			t.Errorf("expected decompressed 'processed data', got '%s'", string(input))
		}
		return []byte("done"), nil
	})

	_, err = n2.Process(res1)
	if err != nil {
		t.Fatalf("unexpected error on second process: %v", err)
	}
}
