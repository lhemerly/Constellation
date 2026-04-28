package node_test

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"github.com/lhemerly/Constellation/node"
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

func TestRateLimiterMiddleware(t *testing.T) {
	n := node.NewBaseNode("rate_limited_node")
	defer cleanupNodes(t, []node.Node{n})

	// Allow 5 requests burst, 1 request per second replenishment
	rateLimiter := node.RateLimiterMiddleware(1.0, 5)
	n.Use(rateLimiter)
	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		return input, nil
	})

	// Fire 5 requests immediately, all should pass
	for i := 0; i < 5; i++ {
		_, err := n.Process([]byte("test"))
		if err != nil {
			t.Fatalf("Expected no error for burst request %d, got %v", i, err)
		}
	}

	// Fire the 6th request immediately, it should fail
	_, err := n.Process([]byte("test"))
	if err == nil {
		t.Fatalf("Expected error for request exceeding burst limit, got nil")
	}

	// Wait 1.1 second to replenish 1 token
	time.Sleep(1100 * time.Millisecond)

	// Fire another request, it should pass
	_, err = n.Process([]byte("test"))
	if err != nil {
		t.Fatalf("Expected no error after token replenishment, got %v", err)
	}

	// And the next one should fail again
	_, err = n.Process([]byte("test"))
	if err == nil {
		t.Fatalf("Expected error after consuming replenished token, got nil")
	}
}

func TestEncryptionMiddleware(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)

	encMiddleware := node.EncryptionMiddleware(key)

	node1 := node.NewBaseNode("sender_node")
	defer cleanupNodes(t, []node.Node{node1})
	node1.Use(encMiddleware)
	node1.SetProcessFunc(func(input []byte) ([]byte, error) {
		// Outputting raw input, but middleware will encrypt it
		return input, nil
	})

	node2 := node.NewBaseNode("receiver_node")
	defer cleanupNodes(t, []node.Node{node2})
	node2.Use(encMiddleware)
	node2.SetProcessFunc(func(input []byte) ([]byte, error) {
		// The middleware will decrypt the input.
		// We return it prefixed to verify we processed the decrypted data.
		res := append([]byte("processed: "), input...)
		return res, nil
	})

	// Node 1 processing
	originalMsg := []byte("secret message")

	// Since we send 'originalMsg' into node1.Process(), node1's middleware will try to DECRYPT it.
	// But it's not encrypted! We need node1 to be the one that encrypts.
	// We can manually encrypt it first, OR change the test.
	// Actually, the simplest way to test is to just encrypt the initial message ourselves,
	// pass it to node1, and node1 decrypts it, processes it, and re-encrypts it.

	block, _ := aes.NewCipher(key)
	aesgcm, _ := cipher.NewGCM(block)

	nonce := make([]byte, aesgcm.NonceSize())
	rand.Read(nonce)
	encryptedOriginalMsg := aesgcm.Seal(nonce, nonce, originalMsg, nil)

	// node1 decrypts encryptedOriginalMsg, processes it, and encrypts the output
	encryptedMsg, err := node1.Process(encryptedOriginalMsg)
	if err != nil {
		t.Fatalf("Expected no error on encryption, got %v", err)
	}

	if string(encryptedMsg) == string(originalMsg) {
		t.Fatalf("Expected encrypted output to differ from original input")
	}

	// Node 2 processing
	// pass encryptedMsg to node2, it should decrypt it, process it, and re-encrypt the result
	encryptedResult, err := node2.Process(encryptedMsg)
	if err != nil {
		t.Fatalf("Expected no error on decryption/processing, got %v", err)
	}

	// Decrypt the result manually to verify it
	// We already have block and aesgcm above

	nonceSize := aesgcm.NonceSize()
	if len(encryptedResult) < nonceSize {
		t.Fatalf("Encrypted result too short")
	}
	nonce2 := encryptedResult[:nonceSize]
	ciphertext := encryptedResult[nonceSize:]

	decryptedResult, err := aesgcm.Open(nil, nonce2, ciphertext, nil)
	if err != nil {
		t.Fatalf("Failed to decrypt final result: %v", err)
	}

	expectedResult := "processed: secret message"
	if string(decryptedResult) != expectedResult {
		t.Fatalf("Expected %q, got %q", expectedResult, string(decryptedResult))
	}
}

func TestEncryptionMiddleware_InvalidInput(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)

	encMiddleware := node.EncryptionMiddleware(key)
	n := node.NewBaseNode("node")
	defer cleanupNodes(t, []node.Node{n})
	n.Use(encMiddleware)

	// Process unencrypted message, should fail
	_, err := n.Process([]byte("unencrypted"))
	if err == nil {
		t.Fatalf("Expected error when processing unencrypted/invalid message")
	}
}
