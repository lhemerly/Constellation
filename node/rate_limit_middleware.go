package node

import (
	"errors"
	"sync"
	"time"
)

// ErrRateLimitExceeded is returned when the rate limit for a node is exceeded.
var ErrRateLimitExceeded = errors.New("rate limit exceeded")

// RateLimitMiddleware limits the rate of processing requests using a token bucket algorithm.
// rate represents the number of tokens added per second, and capacity represents the maximum bucket size.
func RateLimitMiddleware(rate float64, capacity int) Middleware {
	var (
		mu         sync.Mutex
		tokens     float64 = float64(capacity)
		lastUpdate time.Time = time.Now()
	)

	return func(next func([]byte) ([]byte, error)) func([]byte) ([]byte, error) {
		return func(input []byte) ([]byte, error) {
			mu.Lock()

			now := time.Now()
			elapsed := now.Sub(lastUpdate).Seconds()

			// Refill tokens
			tokens += elapsed * rate
			if tokens > float64(capacity) {
				tokens = float64(capacity)
			}
			lastUpdate = now

			// Check if we have enough tokens to process the request
			if tokens >= 1.0 {
				tokens -= 1.0
				mu.Unlock()
				return next(input)
			}

			mu.Unlock()
			return nil, ErrRateLimitExceeded
		}
	}
}
