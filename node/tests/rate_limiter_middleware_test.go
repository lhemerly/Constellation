package node_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestRateLimiterMiddleware_TokensAccumulation(t *testing.T) {
	n := node.NewBaseNode("target")

	var processed int32
	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		atomic.AddInt32(&processed, 1)
		return []byte("ok"), nil
	})

	// Capacity of 5, refill 1 token every 100ms
	n.Use(node.RateLimiterMiddleware(5, 100*time.Millisecond))

	n.Create()

	// Exhaust the initial 5 tokens
	for i := 0; i < 5; i++ {
		_, err := n.Process([]byte("input"))
		if err != nil {
			t.Fatalf("Expected initial request to succeed, got %v", err)
		}
	}

	// Wait just a tiny bit, not enough for a full token
	time.Sleep(10 * time.Millisecond)

	// Send many requests very fast
	for i := 0; i < 100; i++ {
		_, err := n.Process([]byte("input"))
		if err == nil {
			t.Fatalf("Expected fast requests to fail rate limit, got success on request %d", i)
		}
	}

	n.Delete()
}
