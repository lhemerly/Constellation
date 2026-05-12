package node_test

import (
	"errors"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestMiddlewares_RateLimit(t *testing.T) {
	n := node.NewBaseNode("rate-limit-node")
	defer cleanupNodes(t, []node.Node{n})

	n.Use(node.RateLimitMiddleware(10.0, 2)) // 10 tokens/sec, max 2 tokens

	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		return input, nil
	})

	// 1st request should succeed
	_, err := n.Process([]byte("1"))
	if err != nil {
		t.Errorf("Expected success, got %v", err)
	}

	// 2nd request should succeed
	_, err = n.Process([]byte("2"))
	if err != nil {
		t.Errorf("Expected success, got %v", err)
	}

	// 3rd request should fail (rate limit exceeded)
	_, err = n.Process([]byte("3"))
	if !errors.Is(err, node.ErrRateLimitExceeded) {
		t.Errorf("Expected rate limit error, got %v", err)
	}

	// Wait for refill (> 1 token, ~100ms)
	time.Sleep(150 * time.Millisecond)

	// 4th request should succeed
	_, err = n.Process([]byte("4"))
	if err != nil {
		t.Errorf("Expected success after wait, got %v", err)
	}
}
