package node

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
)

// RateLimiterNode limits the rate of incoming requests using a token bucket algorithm.
type RateLimiterNode struct {
	*BaseNode
	rate       float64 // tokens per second
	capacity   float64 // maximum burst size
	tokens     float64
	lastUpdate time.Time
	mu         sync.Mutex
}

// NewRateLimiterNode creates a new RateLimiterNode.
// rate is the number of tokens added per second, and capacity is the maximum burst size.
func NewRateLimiterNode(id string, rate float64, capacity float64) *RateLimiterNode {
	return &RateLimiterNode{
		BaseNode:   NewBaseNode(id),
		rate:       rate,
		capacity:   capacity,
		tokens:     capacity, // Start full
		lastUpdate: time.Now(),
	}
}

// Process processes the input if allowed by the rate limiter.
func (n *RateLimiterNode) Process(input []byte) ([]byte, error) {
	n.mu.Lock()
	now := time.Now()
	elapsed := now.Sub(n.lastUpdate).Seconds()

	// Replenish tokens based on elapsed time
	n.tokens += elapsed * n.rate
	if n.tokens > n.capacity {
		n.tokens = n.capacity
	}
	n.lastUpdate = now

	// Check if we have enough tokens to process
	if n.tokens < 1.0 {
		n.mu.Unlock()
		return nil, ErrRateLimitExceeded
	}

	// Consume a token
	n.tokens -= 1.0
	n.mu.Unlock()

	// Proceed with standard base node processing
	return n.BaseNode.Process(input)
}
