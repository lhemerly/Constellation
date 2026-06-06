package node_test

import (
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestRateLimitMiddleware(t *testing.T) {
	// Rate limit: 10 tokens/sec, max burst: 2
	rateLimitMw := node.RateLimitMiddleware(10.0, 2)

	processFunc := func(input []byte) ([]byte, error) {
		return input, nil
	}

	wrappedFunc := rateLimitMw(processFunc)

	// Attempt to consume 3 tokens immediately.
	// We start with a burst of 2 tokens.

	// Token 1: should succeed
	_, err := wrappedFunc([]byte("req1"))
	if err != nil {
		t.Fatalf("Expected success for first request, got: %v", err)
	}

	// Token 2: should succeed
	_, err = wrappedFunc([]byte("req2"))
	if err != nil {
		t.Fatalf("Expected success for second request, got: %v", err)
	}

	// Token 3: should fail as burst is 2 and we haven't waited
	_, err = wrappedFunc([]byte("req3"))
	if err != node.ErrRateLimitExceeded {
		t.Fatalf("Expected ErrRateLimitExceeded for third request, got: %v", err)
	}

	// Wait enough time to refill at least 1 token.
	// Rate is 10/sec, so 1 token takes 100ms. Wait 150ms.
	time.Sleep(150 * time.Millisecond)

	// Token 4: should succeed after refill
	_, err = wrappedFunc([]byte("req4"))
	if err != nil {
		t.Fatalf("Expected success for fourth request after refill, got: %v", err)
	}
}
