package node_test

import (
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestRateLimitMiddleware_SuccessAndExceed(t *testing.T) {
	n := node.NewBaseNode("rate-node")

	// Allow 2 requests per second, burst size 2
	n.Use(node.RateLimitMiddleware(2.0, 2))
	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		return input, nil
	})

	n.Create()
	defer cleanupNodes(t, []node.Node{n})

	// Burst: 2 requests should succeed immediately
	_, err := n.Process([]byte("req1"))
	if err != nil {
		t.Fatalf("Expected no error for 1st request, got %v", err)
	}

	_, err = n.Process([]byte("req2"))
	if err != nil {
		t.Fatalf("Expected no error for 2nd request, got %v", err)
	}

	// 3rd request should fail as the bucket is empty
	_, err = n.Process([]byte("req3"))
	if err != node.ErrRateLimitExceeded {
		t.Fatalf("Expected ErrRateLimitExceeded for 3rd request, got %v", err)
	}

	// Wait 600ms to allow 1 token to replenish (rate is 2/sec, so 500ms per token)
	time.Sleep(600 * time.Millisecond)

	_, err = n.Process([]byte("req4"))
	if err != nil {
		t.Fatalf("Expected no error after token replenishment, got %v", err)
	}
}
