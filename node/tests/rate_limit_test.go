package node_test

import (
	"errors"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
	"github.com/stretchr/testify/assert"
)

func TestRateLimitMiddleware(t *testing.T) {
	// Create a node and apply the rate limit middleware
	// Rate of 10 tokens per second (0.1s per token), capacity of 2
	rateLimiter := node.RateLimitMiddleware(10.0, 2)

	testNode := node.NewBaseNode("test-rate-limit")
	testNode.Use(rateLimiter)

	// Since capacity is 2, the first two requests should succeed immediately
	_, err := testNode.Process([]byte("req1"))
	assert.NoError(t, err, "First request should succeed")

	_, err = testNode.Process([]byte("req2"))
	assert.NoError(t, err, "Second request should succeed")

	// Third request should fail immediately because capacity is exhausted
	_, err = testNode.Process([]byte("req3"))
	assert.Error(t, err, "Third request should fail")
	assert.True(t, errors.Is(err, node.ErrRateLimitExceeded), "Error should be ErrRateLimitExceeded")

	// Wait for 1 token to refill (10 tokens/sec = 100ms per token)
	// We wait slightly longer to ensure the token has refilled
	time.Sleep(150 * time.Millisecond)

	// Fourth request should succeed
	_, err = testNode.Process([]byte("req4"))
	assert.NoError(t, err, "Fourth request should succeed after token refill")

	// Fifth request should fail again
	_, err = testNode.Process([]byte("req5"))
	assert.Error(t, err, "Fifth request should fail due to no tokens left")
	assert.True(t, errors.Is(err, node.ErrRateLimitExceeded), "Error should be ErrRateLimitExceeded")
}
