package node_test

import (
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestRateLimitMiddleware_Exceed(t *testing.T) {
	n := node.NewBaseNode("rate-limit-node")
	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		return input, nil
	})

	// 1 token per second, max 2 tokens
	n.Use(node.RateLimitMiddleware(1.0, 2))

	// First two should succeed immediately
	for i := 0; i < 2; i++ {
		_, err := n.Process([]byte("test"))
		if err != nil {
			t.Fatalf("Expected no error on attempt %d, got: %v", i+1, err)
		}
	}

	// Third should fail since bucket is empty and we haven't waited
	_, err := n.Process([]byte("test"))
	if err == nil {
		t.Fatal("Expected error on third attempt, got nil")
	}

	if err != node.ErrRateLimitExceeded {
		t.Fatalf("Expected ErrRateLimitExceeded, got: %v", err)
	}
}

func TestRateLimitMiddleware_Refill(t *testing.T) {
	n := node.NewBaseNode("rate-limit-refill-node")
	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		return input, nil
	})

	// 10 tokens per second, max 1 token. This means it refills 1 token every 100ms.
	n.Use(node.RateLimitMiddleware(10.0, 1))

	// First should succeed
	_, err := n.Process([]byte("test"))
	if err != nil {
		t.Fatalf("Expected no error on first attempt, got: %v", err)
	}

	// Second should fail immediately
	_, err = n.Process([]byte("test"))
	if err != node.ErrRateLimitExceeded {
		t.Fatalf("Expected ErrRateLimitExceeded, got: %v", err)
	}

	// Wait for refill (> 100ms)
	time.Sleep(150 * time.Millisecond)

	// Should succeed now
	_, err = n.Process([]byte("test"))
	if err != nil {
		t.Fatalf("Expected no error after refill, got: %v", err)
	}
}
