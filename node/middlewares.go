package node

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"sync"
	"time"
)

var (
	ErrCircuitBreakerOpen = errors.New("circuit breaker is open")
	ErrProcessTimeout     = errors.New("process timed out")
	ErrRateLimitExceeded  = errors.New("rate limit exceeded")
)

// LoggingMiddleware logs the payload size and processing time.
func LoggingMiddleware(next func([]byte) ([]byte, error)) func([]byte) ([]byte, error) {
	return func(input []byte) ([]byte, error) {
		start := time.Now()
		log.Printf("Process started. Input size: %d bytes\n", len(input))

		output, err := next(input)

		duration := time.Since(start)
		if err != nil {
			log.Printf("Process failed after %s: %v\n", duration, err)
		} else {
			log.Printf("Process completed in %s. Output size: %d bytes\n", duration, len(output))
		}

		return output, err
	}
}

// RetryMiddleware retries a failed operation for a specified number of times with a delay.
func RetryMiddleware(retries int, delay time.Duration) Middleware {
	return func(next func([]byte) ([]byte, error)) func([]byte) ([]byte, error) {
		return func(input []byte) ([]byte, error) {
			var err error
			var output []byte

			for i := 0; i <= retries; i++ {
				// Clone the input buffer for each attempt to avoid side effects
				// from in-place modifications during a failed process attempt.
				inputCopy := make([]byte, len(input))
				copy(inputCopy, input)

				output, err = next(inputCopy)
				if err == nil {
					return output, nil
				}

				if i < retries {
					time.Sleep(delay)
				}
			}

			return nil, errors.Join(errors.New("operation failed after retries"), err)
		}
	}
}

// CompressionMiddleware decompresses input and compresses output using gzip.
// It assumes all input should be decompressed, and all output should be compressed.
func CompressionMiddleware(next func([]byte) ([]byte, error)) func([]byte) ([]byte, error) {
	return func(input []byte) ([]byte, error) {
		// Decompress input if it's not empty
		var decompressedInput []byte
		if len(input) > 0 {
			reader, err := gzip.NewReader(bytes.NewReader(input))
			if err != nil {
				return nil, errors.Join(errors.New("failed to create gzip reader for input"), err)
			}
			decompressedInput, err = io.ReadAll(reader)
			reader.Close()
			if err != nil {
				return nil, errors.Join(errors.New("failed to decompress input"), err)
			}
		}

		// Process decompressed input
		output, err := next(decompressedInput)
		if err != nil {
			return nil, err // Return error directly, don't try to compress
		}

		// Compress output
		var compressedOutput bytes.Buffer
		writer := gzip.NewWriter(&compressedOutput)
		if _, writeErr := writer.Write(output); writeErr != nil {
			return nil, errors.Join(errors.New("failed to write compressed output"), writeErr)
		}
		if closeErr := writer.Close(); closeErr != nil {
			return nil, errors.Join(errors.New("failed to close gzip writer"), closeErr)
		}

		return compressedOutput.Bytes(), nil
	}
}

// RateLimitMiddleware restricts the number of processes allowed within a specified time interval.
// If the limit is exceeded, it returns an ErrRateLimitExceeded.
func RateLimitMiddleware(requests int, interval time.Duration) Middleware {
	var (
		mu         sync.Mutex
		tokens     = requests
		lastRefill = time.Now()
	)

	return func(next func([]byte) ([]byte, error)) func([]byte) ([]byte, error) {
		return func(input []byte) ([]byte, error) {
			mu.Lock()

			now := time.Now()
			elapsed := now.Sub(lastRefill)

			if elapsed >= interval {
				tokens = requests
				lastRefill = now
			}

			if tokens <= 0 {
				mu.Unlock()
				return nil, ErrRateLimitExceeded
			}

			tokens--
			mu.Unlock()

			return next(input)
		}
	}
}

// CacheMiddleware caches the output of successful processes for a given TTL, keyed by the hash of the input.
func CacheMiddleware(ttl time.Duration) Middleware {
	type cacheEntry struct {
		output    []byte
		expiresAt time.Time
	}

	var (
		mu    sync.RWMutex
		cache = make(map[string]cacheEntry)
	)

	return func(next func([]byte) ([]byte, error)) func([]byte) ([]byte, error) {
		return func(input []byte) ([]byte, error) {
			hasher := sha256.New()
			hasher.Write(input)
			key := hex.EncodeToString(hasher.Sum(nil))

			mu.RLock()
			entry, exists := cache[key]
			mu.RUnlock()

			if exists && time.Now().Before(entry.expiresAt) {
				// Return cached output (clone to prevent external modification)
				outputCopy := make([]byte, len(entry.output))
				copy(outputCopy, entry.output)
				return outputCopy, nil
			}

			output, err := next(input)
			if err == nil {
				// Cache successful result (clone to prevent external modification)
				outputCopy := make([]byte, len(output))
				copy(outputCopy, output)

				mu.Lock()
				cache[key] = cacheEntry{
					output:    outputCopy,
					expiresAt: time.Now().Add(ttl),
				}
				mu.Unlock()
			}

			return output, err
		}
	}
}

// TimeoutMiddleware limits the execution time of the processing function.
// If the function takes longer than the specified timeout, it returns a timeout error.
func TimeoutMiddleware(timeout time.Duration) Middleware {
	return func(next func([]byte) ([]byte, error)) func([]byte) ([]byte, error) {
		return func(input []byte) ([]byte, error) {
			resultChan := make(chan struct {
				output []byte
				err    error
			}, 1)

			go func() {
				// Clone input to avoid race condition if 'next' modifies it
				// while the parent function has already returned a timeout error.
				inputCopy := make([]byte, len(input))
				copy(inputCopy, input)

				output, err := next(inputCopy)
				resultChan <- struct {
					output []byte
					err    error
				}{output, err}
			}()

			select {
			case res := <-resultChan:
				return res.output, res.err
			case <-time.After(timeout):
				return nil, ErrProcessTimeout
			}
		}
	}
}

// RecoveryMiddleware recovers from panics gracefully and turns them into errors.
func RecoveryMiddleware(next func([]byte) ([]byte, error)) func([]byte) ([]byte, error) {
	return func(input []byte) (output []byte, err error) {
		defer func() {
			if r := recover(); r != nil {
				switch x := r.(type) {
				case string:
					err = errors.New(x)
				case error:
					err = x
				default:
					err = errors.New("unknown panic occurred")
				}
			}
		}()

		return next(input)
	}
}

// CircuitBreakerMiddleware prevents processing when failures exceed a threshold.
// After a cooldown period, it allows a single request to test if the service has recovered.
func CircuitBreakerMiddleware(maxFailures int, cooldown time.Duration) Middleware {
	var (
		mu          sync.Mutex
		failures    int
		state       int // 0: Closed, 1: Open, 2: Half-Open
		lastFailure time.Time
	)

	return func(next func([]byte) ([]byte, error)) func([]byte) ([]byte, error) {
		return func(input []byte) ([]byte, error) {
			mu.Lock()

			if state == 1 { // Open
				if time.Since(lastFailure) >= cooldown {
					state = 2 // Half-Open
				} else {
					mu.Unlock()
					return nil, ErrCircuitBreakerOpen
				}
			}

			if state == 2 { // Half-Open
				// Temporarily switch to Open to block other concurrent requests while we test
				state = 1
				lastFailure = time.Now()
			}

			mu.Unlock()

			output, err := next(input)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				failures++
				if failures >= maxFailures {
					state = 1 // Open
					lastFailure = time.Now()
				}
			} else {
				failures = 0
				state = 0 // Closed
			}

			return output, err
		}
	}
}
