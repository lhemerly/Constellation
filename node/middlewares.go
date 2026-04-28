package node

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
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

// RateLimiterMiddleware limits the rate of processing requests using a token bucket.
// rate is the number of tokens added per second, burst is the maximum bucket size.
func RateLimiterMiddleware(rate float64, burst int) Middleware {
	var (
		mu         sync.Mutex
		tokens     float64   = float64(burst)
		lastUpdate time.Time = time.Now()
	)

	return func(next func([]byte) ([]byte, error)) func([]byte) ([]byte, error) {
		return func(input []byte) ([]byte, error) {
			mu.Lock()
			now := time.Now()
			elapsed := now.Sub(lastUpdate).Seconds()

			// Replenish tokens
			tokens += elapsed * rate
			if tokens > float64(burst) {
				tokens = float64(burst)
			}
			lastUpdate = now

			if tokens < 1.0 {
				mu.Unlock()
				return nil, errors.New("rate limit exceeded")
			}

			tokens -= 1.0
			mu.Unlock()

			return next(input)
		}
	}
}

// EncryptionMiddleware encrypts the output and decrypts the input using AES-GCM.
// The key must be 16, 24, or 32 bytes for AES-128, AES-192, or AES-256 respectively.
func EncryptionMiddleware(key []byte) Middleware {
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err) // Initialization panic, standard for invalid key setup
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err)
	}

	return func(next func([]byte) ([]byte, error)) func([]byte) ([]byte, error) {
		return func(input []byte) ([]byte, error) {
			// Try to decrypt the input
			var decryptedInput []byte
			// Check if we should enforce encryption on the input
			// As this is a middleware in a chain, the very first input might be completely unencrypted,
			// or it might be coming from another node. We must allow unencrypted inputs if they
			// fail to decrypt but ONLY if it's considered raw data. For strictness, if it fails
			// decryption, we will return error, BUT if it is short, we could assume it's raw.
			// Wait, the test uses "secret message" which is short and returns "decryption failed"
			// because it IS long enough to be a nonce but fails decryption, OR it's short.
			// Let's modify the test or the middleware. The easiest is that EncryptionMiddleware assumes
			// the node *receives* encrypted messages (from another node over wire) and *outputs* encrypted messages.
			// If a message cannot be decrypted, it's an error.
			if len(input) >= aesgcm.NonceSize() {
				nonce := input[:aesgcm.NonceSize()]
				ciphertext := input[aesgcm.NonceSize():]
				var decErr error
				decryptedInput, decErr = aesgcm.Open(nil, nonce, ciphertext, nil)
				if decErr != nil {
					// In our test, "secret message" is 14 bytes long.
					// Nonce is 12 bytes. So it's >= 12, but decryption fails.
					return nil, errors.Join(errors.New("decryption failed"), decErr)
				}
			} else {
				return nil, errors.New("input too short to contain nonce")
			}

			// Process decrypted data
			output, err := next(decryptedInput)
			if err != nil {
				return nil, err
			}

			// Encrypt output
			nonce := make([]byte, aesgcm.NonceSize())
			if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
				return nil, err
			}
			encryptedOutput := aesgcm.Seal(nonce, nonce, output, nil)
			return encryptedOutput, nil
		}
	}
}
